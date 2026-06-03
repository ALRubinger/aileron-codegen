package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ActionEmitter renders one action.md per Spec Operation into
// actions/<name>/ under outDir. The emitted file is the TOML front-matter
// only; prose body generation lands in a follow-up PR.
//
// The `0.0.0-dev` and `sha256:bound-at-release` strings in the template are
// load-bearing placeholders, not bugs. The release workflow substitutes
// them per ADR-0002 (connector identity + content-addressed hash); the
// generator emits the placeholders verbatim so that substitution stays
// idempotent against an unchanged template.
type ActionEmitter struct{}

// Emit writes actions/<name>/action.md for every operation in spec, less
// the ones in overlay.Exclude. Each emission is driven by the kind-based
// governance defaults (Query / GET / HEAD / PUT / Subscription →
// idempotent + approval none; Mutation / POST / PATCH / DELETE →
// non-idempotent + approval required), with overlay.Operations[op.ID]
// applied as field-by-field overrides when present.
//
// Overlay entries (in Operations or Exclude) whose op id does not appear
// in the spec are rejected as typos, surfacing renames or
// removed-but-not-cleaned-up overlays at emission time rather than
// silently dropping the action like the pre-#7 behavior did.
func (ActionEmitter) Emit(spec Spec, overlay Overlay, outDir string) error {
	if err := validateOverlayOps(spec, overlay); err != nil {
		return err
	}
	for _, op := range emittableOperations(spec, overlay) {
		o := overlay.Operations[op.ID]
		if err := emitAction(op, o, overlay.Connector, outDir); err != nil {
			return fmt.Errorf("emit %s: %w", op.ID, err)
		}
	}
	return nil
}

// validateOverlayOps surfaces typo-style errors at the codegen
// boundary: an overlay block (Operations or Exclude) referencing an
// operation that the spec does not define. Pre-#7 this was silent —
// the action just didn't emit — which made overlay/spec drift very
// easy to ship.
func validateOverlayOps(spec Spec, overlay Overlay) error {
	valid := make(map[string]bool, len(spec.Operations))
	for _, op := range spec.Operations {
		valid[op.ID] = true
	}
	for id := range overlay.Operations {
		if !valid[id] {
			return fmt.Errorf("overlay.operations declares %q, but the spec defines no such operation (typo, or renamed in the spec?)", id)
		}
	}
	for _, id := range overlay.Exclude {
		if !valid[id] {
			return fmt.Errorf("overlay.exclude lists %q, but the spec defines no such operation", id)
		}
	}
	return nil
}

// emittableOperations returns the spec operations that will produce
// action.md files, after applying overlay.Exclude. Shared between
// ActionEmitter (which writes the files) and SuiteEmitter (which lists
// them in suite.toml) so the two stay in sync without sharing state.
func emittableOperations(spec Spec, overlay Overlay) []Operation {
	if len(overlay.Exclude) == 0 {
		return spec.Operations
	}
	excluded := make(map[string]bool, len(overlay.Exclude))
	for _, id := range overlay.Exclude {
		excluded[id] = true
	}
	out := make([]Operation, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		if !excluded[op.ID] {
			out = append(out, op)
		}
	}
	return out
}

func emitAction(op Operation, o OperationOverlay, conn ConnectorOverlay, outDir string) error {
	view := actionView{
		Name:         resolveActionName(op, o),
		ExecuteID:    resolveExecuteID(op, o),
		OpName:       snakeCase(op.ID),
		Connector:    conn.Name,
		Capabilities: resolveCapabilities(op, o),
		Intent:       resolveIntent(op, o),
		Idempotent:   resolveIdempotent(op, o),
		ApprovalReq:  resolveApprovalRequired(op, o),
		Inputs:       flattenInputs(op),
	}
	dir := filepath.Join(outDir, "actions", view.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "action.md"))
	if err != nil {
		return err
	}
	defer f.Close()
	return actionTmpl.Execute(f, view)
}

func resolveActionName(op Operation, o OperationOverlay) string {
	if o.ActionName != "" {
		return o.ActionName
	}
	return kebabCase(op.ID)
}

func resolveExecuteID(op Operation, o OperationOverlay) string {
	if o.ExecuteID != "" {
		return o.ExecuteID
	}
	return firstWord(op.ID)
}

func resolveIntent(op Operation, o OperationOverlay) string {
	if o.Intent != "" {
		return o.Intent
	}
	return op.Summary
}

// resolveIdempotent merges the overlay's optional idempotent override
// with the kind-based default for the operation's method.
func resolveIdempotent(op Operation, o OperationOverlay) bool {
	if o.Idempotent != nil {
		return *o.Idempotent
	}
	return defaultIdempotentForMethod(op.Method)
}

// resolveApprovalRequired merges the overlay's optional approval string
// with the kind-based default. Returns true when the action.md should
// emit an `[approval] required = true` block.
func resolveApprovalRequired(op Operation, o OperationOverlay) bool {
	if o.Approval != "" {
		return strings.EqualFold(o.Approval, "required")
	}
	return defaultApprovalRequiredForMethod(op.Method)
}

// resolveCapabilities returns the overlay's capability list when
// non-empty; otherwise falls back to a single-entry list derived from
// the operationId — a sensible "one capability per op" default that
// surfaces if the overlay author forgot to declare them. Real connectors
// typically override this to coalesce capabilities by domain.
func resolveCapabilities(op Operation, o OperationOverlay) []string {
	if len(o.Capabilities) > 0 {
		return o.Capabilities
	}
	return []string{snakeCase(op.ID)}
}

// defaultIdempotentForMethod is the kind-based idempotency default per
// the table in ALRubinger/aileron-codegen#7. Read operations (GraphQL
// queries / subscriptions, OpenAPI GET / HEAD) and conventionally
// idempotent writes (PUT) default to idempotent. Mutation-style writes
// (POST / PATCH / DELETE / GraphQL mutations) default to non-idempotent.
// Unknown / empty method falls through to non-idempotent — conservative
// because the consequence of a wrong "true" is silently re-running a
// write under the runtime's retry layer.
func defaultIdempotentForMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "QUERY", "SUBSCRIPTION", "GET", "HEAD", "PUT":
		return true
	}
	return false
}

// defaultApprovalRequiredForMethod is the kind-based approval default.
// Reads (queries / GET / HEAD / subscriptions) default to no approval.
// Writes (mutations / POST / PUT / PATCH / DELETE) default to requiring
// per-call approval. Unknown method defaults to requiring approval —
// fail closed if the loader produced an unrecognised method.
func defaultApprovalRequiredForMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "QUERY", "SUBSCRIPTION", "GET", "HEAD":
		return false
	}
	return true
}

// flattenInputs concatenates path/query/header parameters with required
// JSON body fields, in that order. Both lists are already sorted at parse
// time so the final sequence is deterministic.
func flattenInputs(op Operation) []Parameter {
	out := append([]Parameter{}, op.Parameters...)
	if op.RequestBody != nil {
		out = append(out, op.RequestBody.Fields...)
	}
	return out
}

type actionView struct {
	Name         string
	ExecuteID    string
	OpName       string
	Connector    string
	Capabilities []string
	Intent       string
	Idempotent   bool
	ApprovalReq  bool
	Inputs       []Parameter
}

var actionTmpl = template.Must(template.New("action").Funcs(template.FuncMap{
	"quote":     tomlQuote,
	"quoteList": tomlStringList,
}).Parse(actionTemplate))

const actionTemplate = `+++
name = {{quote .Name}}
version = "0.0.0-dev"
source = "{{.Connector}}/actions/{{.Name}}@0.0.0-dev"

[[requires.connectors]]
name = {{quote .Connector}}
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = {{quoteList .Capabilities}}

[match]
intent = {{quote .Intent}}

[[execute]]
id = {{quote .ExecuteID}}
connector = {{quote .Connector}}
op = {{quote .OpName}}
idempotent = {{.Idempotent}}
{{if .ApprovalReq}}
[approval]
required = true
{{end}}{{range .Inputs}}
[[inputs]]
name = {{quote .Name}}
type = {{quote .Type}}
description = {{quote .Description}}
required = {{.Required}}
{{end}}+++
`

// tomlQuote returns s as a TOML basic string with the minimal escape set
// the templates need (backslash, double quote, newline, tab).
func tomlQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tomlStringList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = tomlQuote(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
