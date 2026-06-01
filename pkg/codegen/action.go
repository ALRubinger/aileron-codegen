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

// Emit walks spec.Operations and writes actions/<name>/action.md for every
// operation that has a matching overlay entry. Operations without an
// overlay entry are skipped because the action.md contract requires
// governance metadata (idempotency, approval, capabilities) per operation.
func (ActionEmitter) Emit(spec Spec, overlay Overlay, outDir string) error {
	for _, op := range spec.Operations {
		o, ok := overlay.Operations[op.ID]
		if !ok {
			continue
		}
		if err := emitAction(op, o, overlay.Connector, outDir); err != nil {
			return fmt.Errorf("emit %s: %w", op.ID, err)
		}
	}
	return nil
}

func emitAction(op Operation, o OperationOverlay, conn ConnectorOverlay, outDir string) error {
	view := actionView{
		Name:         resolveActionName(op, o),
		ExecuteID:    resolveExecuteID(op, o),
		OpName:       snakeCase(op.ID),
		Connector:    conn.Name,
		Capabilities: o.Capabilities,
		Intent:       resolveIntent(op, o),
		Idempotent:   o.Idempotent,
		ApprovalReq:  strings.EqualFold(o.Approval, "required"),
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
