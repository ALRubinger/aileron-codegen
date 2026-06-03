package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// HandlerEmitter renders connector/handlers.go from the spec + overlay.
// One per-op function plus the shared arg-extraction helpers. Each
// handler builds a GraphQL query string (with the selection set
// derived from Operation.ReturnScalarFields), validates required args,
// rebuilds any input-object wrapping the loader flattened, and calls
// the connector's `graphqlCall` transport.
//
// The emitted file assumes the connector also has a hand-written (or
// later codegen-emitted) `dispatch.go` defining `Transport` and a
// `graphql.go` defining `graphqlCall` in the same package.
//
// Skipped silently when the spec has no GraphQL operations (every op's
// Method falls outside QUERY / MUTATION / SUBSCRIPTION) — keeps
// OpenAPI test cases green while OpenAPI handler emission lands later.
type HandlerEmitter struct{}

// Emit writes connector/handlers.go under outDir. No-op when there are
// no GraphQL operations to emit.
func (HandlerEmitter) Emit(spec Spec, overlay Overlay, outDir string) error {
	ops := emittableGraphQLOps(spec, overlay)
	if len(ops) == 0 {
		return nil
	}
	view := handlerFileView{Operations: make([]handlerOpView, 0, len(ops))}
	for _, op := range ops {
		view.Operations = append(view.Operations, buildHandlerOpView(op))
	}
	var raw bytes.Buffer
	if err := handlersTmpl.Execute(&raw, view); err != nil {
		return err
	}
	formatted, err := format.Source(raw.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt generated handlers.go: %w", err)
	}
	dir := filepath.Join(outDir, "connector")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "handlers.go"), formatted, 0o644)
}

// emittableGraphQLOps filters the spec to GraphQL ops minus any in the
// overlay's exclude list. OpenAPI ops are skipped silently — the
// handler emitter for HTTP/REST ops is later work.
func emittableGraphQLOps(spec Spec, overlay Overlay) []Operation {
	excluded := make(map[string]bool, len(overlay.Exclude))
	for _, id := range overlay.Exclude {
		excluded[id] = true
	}
	var out []Operation
	for _, op := range spec.Operations {
		if excluded[op.ID] {
			continue
		}
		switch strings.ToUpper(op.Method) {
		case "QUERY", "MUTATION", "SUBSCRIPTION":
			out = append(out, op)
		}
	}
	return out
}

type handlerFileView struct {
	Operations []handlerOpView
}

type handlerOpView struct {
	QueryConstName string // e.g., "issueQuery"
	HandlerName    string // e.g., "handleIssue"
	QueryString    string // the GraphQL document
	Body           string // the handler function body (lines after the signature)
}

func buildHandlerOpView(op Operation) handlerOpView {
	return handlerOpView{
		QueryConstName: queryConstName(op),
		HandlerName:    handlerName(op),
		QueryString:    buildQueryString(op),
		Body:           buildHandlerBody(op),
	}
}

// queryConstName returns the Go identifier of the GraphQL query
// constant: camelCase(op.ID) + ("Query" | "Mutation" | "Subscription").
func queryConstName(op Operation) string {
	suffix := "Query"
	switch strings.ToUpper(op.Method) {
	case "MUTATION":
		suffix = "Mutation"
	case "SUBSCRIPTION":
		suffix = "Subscription"
	}
	return lowerFirst(op.ID) + suffix
}

// handlerName returns the Go identifier of the handler function:
// "handle" + PascalCase(op.ID).
func handlerName(op Operation) string {
	return "handle" + upperFirst(op.ID)
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// buildQueryString assembles the GraphQL document for one op. Shape:
//
//	<verb> <PascalCase(opID)>($arg: Type, ...) {
//	  <opID>(arg: $arg, ...) {
//	    <scalar field 1>
//	    <scalar field 2>
//	  }
//	}
//
// For scalar-return ops the inner selection set is omitted (just the
// field call). For ops with no args, the variable declaration and the
// field-call parens are omitted.
func buildQueryString(op Operation) string {
	verb := strings.ToLower(op.Method)
	if verb == "" {
		verb = "query"
	}
	var b strings.Builder
	b.WriteString(verb)
	b.WriteByte(' ')
	b.WriteString(upperFirst(op.ID))

	argNames := sortedArgNames(op)
	if len(argNames) > 0 {
		b.WriteByte('(')
		for i, name := range argNames {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteByte('$')
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(op.ArgTypes[name])
		}
		b.WriteByte(')')
	}
	b.WriteString(" {\n  ")
	b.WriteString(op.ID)
	if len(argNames) > 0 {
		b.WriteByte('(')
		for i, name := range argNames {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
			b.WriteString(": $")
			b.WriteString(name)
		}
		b.WriteByte(')')
	}
	if !op.ReturnTypeIsScalar && len(op.ReturnScalarFields) > 0 {
		b.WriteString(" {\n")
		for _, f := range op.ReturnScalarFields {
			b.WriteString("    ")
			b.WriteString(f)
			b.WriteByte('\n')
		}
		b.WriteString("  }")
	}
	b.WriteString("\n}")
	return b.String()
}

func sortedArgNames(op Operation) []string {
	names := make([]string, 0, len(op.ArgTypes))
	for n := range op.ArgTypes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// buildHandlerBody assembles the body of the handler function. Lines
// are pre-indented one tab. The signature
// `func handleXxx(args map[string]any, transport Transport) (map[string]any, error) {`
// is written by the template.
//
// Two cases:
//   - No-arg op: body is just `return graphqlCall(transport, fooQuery, nil)`.
//   - Args op:
//       1. Extract required scalars upfront with `require*` helpers.
//       2. Build the variables map literal with required values inline
//          and optional values via if-ok blocks afterward.
//
// Input-object wrapping (Parameter.WrappedIn) groups flattened fields
// into a sub-map under the wrapping argument name.
func buildHandlerBody(op Operation) string {
	if len(op.Parameters) == 0 {
		return "\treturn graphqlCall(transport, " + queryConstName(op) + ", nil)\n"
	}
	groups := groupByWrap(op.Parameters)
	var b strings.Builder

	// Pass 1: extract all required params upfront.
	for _, g := range groups {
		for _, p := range g.params {
			if !p.Required {
				continue
			}
			emitRequireLine(&b, p)
		}
	}

	// Pass 2: build per-group sub-maps for wrapped groups; emit
	// optional fields after each map literal.
	for _, g := range groups {
		if g.wrappedIn == "" {
			continue
		}
		emitWrappedGroup(&b, g)
	}

	// Pass 3: assemble the outer variables map.
	emitVariablesMap(&b, op, groups)
	return b.String()
}

type paramGroup struct {
	wrappedIn string
	params    []Parameter
}

func groupByWrap(params []Parameter) []paramGroup {
	// Preserve a stable order: wrapped groups by wrappedIn name, then
	// the unwrapped group (wrappedIn == "") last.
	seen := map[string]int{}
	var groups []paramGroup
	for _, p := range params {
		idx, ok := seen[p.WrappedIn]
		if !ok {
			idx = len(groups)
			seen[p.WrappedIn] = idx
			groups = append(groups, paramGroup{wrappedIn: p.WrappedIn})
		}
		groups[idx].params = append(groups[idx].params, p)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		ai, bi := groups[i].wrappedIn, groups[j].wrappedIn
		if ai == bi {
			return false
		}
		if ai == "" {
			return false
		}
		if bi == "" {
			return true
		}
		return ai < bi
	})
	return groups
}

// emitRequireLine writes the Go statements that extract one required
// scalar param and propagate any error. Uses requireString for unknown
// types because GraphQL custom scalars and enums travel as strings on
// the JSON wire.
func emitRequireLine(b *strings.Builder, p Parameter) {
	fn := requireFnForType(p.Type)
	fmt.Fprintf(b, "\t%s, err := %s(args, %q)\n", goVarName(p.Name), fn, p.Name)
	b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")
}

func requireFnForType(t string) string {
	switch t {
	case "Int":
		return "requireInt"
	case "Float":
		return "requireFloat"
	case "Boolean":
		return "requireBool"
	}
	return "requireString"
}

func optionalFnForType(t string) string {
	switch t {
	case "Int":
		return "optionalInt"
	case "Float":
		return "optionalFloat"
	case "Boolean":
		return "optionalBool"
	}
	return "optionalString"
}

// goVarName mangles a GraphQL field name into a safe Go local-variable
// identifier. "id" → "id"; "teamId" → "teamID" (idiomatic Go);
// "type" (reserved) → "type_". Required because connector authors
// will read the generated code and shouldn't trip over warnings.
func goVarName(name string) string {
	if name == "" {
		return name
	}
	switch name {
	case "type", "func", "range", "select", "case", "default", "go",
		"map", "chan", "interface", "struct", "package", "import",
		"return", "if", "else", "for", "switch", "const", "var",
		"break", "continue", "fallthrough", "defer":
		return name + "_"
	}
	// Idiomatic Go: trailing "Id" → "ID", "Url" → "URL", etc.
	// Conservative single-case rewrite for the most common one.
	if strings.HasSuffix(name, "Id") && !strings.HasSuffix(name, "iId") {
		return name[:len(name)-2] + "ID"
	}
	return name
}

// emitWrappedGroup writes the sub-map literal for one input-object
// wrapping group. Required fields go inline; optional fields each get
// an if-ok block appended after the map.
func emitWrappedGroup(b *strings.Builder, g paramGroup) {
	target := goVarName(g.wrappedIn)
	requiredParams := make([]Parameter, 0, len(g.params))
	optionalParams := make([]Parameter, 0, len(g.params))
	for _, p := range g.params {
		if p.Required {
			requiredParams = append(requiredParams, p)
		} else {
			optionalParams = append(optionalParams, p)
		}
	}
	if len(requiredParams) == 0 {
		fmt.Fprintf(b, "\t%s := map[string]any{}\n", target)
	} else {
		fmt.Fprintf(b, "\t%s := map[string]any{\n", target)
		for _, p := range requiredParams {
			fmt.Fprintf(b, "\t\t%q: %s,\n", p.Name, goVarName(p.Name))
		}
		b.WriteString("\t}\n")
	}
	for _, p := range optionalParams {
		emitOptionalAssign(b, target, p)
	}
}

func emitOptionalAssign(b *strings.Builder, target string, p Parameter) {
	fn := optionalFnForType(p.Type)
	fmt.Fprintf(b, "\tif v, ok := %s(args, %q); ok {\n", fn, p.Name)
	fmt.Fprintf(b, "\t\t%s[%q] = v\n", target, p.Name)
	b.WriteString("\t}\n")
}

// emitVariablesMap writes the outer variables map literal + the
// trailing graphqlCall return statement. The map keys are the
// GraphQL argument names; the values are either the local variable
// from a require* extraction (for direct scalar args) or the wrapping
// sub-map built earlier (for input-object args).
func emitVariablesMap(b *strings.Builder, op Operation, groups []paramGroup) {
	// Optional unwrapped scalar params: assemble after the map literal.
	type entry struct {
		key       string // argument name on the wire
		valueExpr string // Go expression
	}
	var inline []entry
	for _, g := range groups {
		if g.wrappedIn != "" {
			inline = append(inline, entry{
				key:       g.wrappedIn,
				valueExpr: goVarName(g.wrappedIn),
			})
			continue
		}
		for _, p := range g.params {
			if p.Required {
				inline = append(inline, entry{
					key:       p.Name,
					valueExpr: goVarName(p.Name),
				})
			}
		}
	}
	sort.SliceStable(inline, func(i, j int) bool { return inline[i].key < inline[j].key })

	if len(inline) == 0 {
		b.WriteString("\tvariables := map[string]any{}\n")
	} else {
		b.WriteString("\tvariables := map[string]any{\n")
		for _, e := range inline {
			fmt.Fprintf(b, "\t\t%q: %s,\n", e.key, e.valueExpr)
		}
		b.WriteString("\t}\n")
	}

	// Append optional unwrapped scalars to the variables map. These are
	// rare (most optional args travel inside input objects), but cover
	// them for completeness.
	for _, g := range groups {
		if g.wrappedIn != "" {
			continue
		}
		for _, p := range g.params {
			if !p.Required {
				emitOptionalAssign(b, "variables", p)
			}
		}
	}
	fmt.Fprintf(b, "\treturn graphqlCall(transport, %s, variables)\n", queryConstName(op))
}

var handlersTmpl = template.Must(template.New("handlers").Parse(handlersTemplate))

const handlersTemplate = `// Generated by aileron-codegen — do not edit.
//
// Re-run ` + "`task generate`" + ` to refresh after changing the spec or
// gen.yaml. Hand edits will be overwritten on the next run.

package main

import "fmt"

// Per-operation GraphQL constants and dispatch handlers.
{{range .Operations}}
const {{.QueryConstName}} = ` + "`{{.QueryString}}`" + `

func {{.HandlerName}}(args map[string]any, transport Transport) (map[string]any, error) {
{{.Body}}}
{{end}}
// Argument extraction helpers shared across all generated handlers.

func requireString(args map[string]any, name string) (string, error) {
	v, ok := args[name]
	if !ok {
		return "", fmt.Errorf("missing required arg %q", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("arg %q: expected string, got %T", name, v)
	}
	if s == "" {
		return "", fmt.Errorf("arg %q: must not be empty", name)
	}
	return s, nil
}

func requireInt(args map[string]any, name string) (int, error) {
	v, ok := args[name]
	if !ok {
		return 0, fmt.Errorf("missing required arg %q", name)
	}
	switch n := v.(type) {
	case int:
		return n, nil
	case float64:
		return int(n), nil
	}
	return 0, fmt.Errorf("arg %q: expected integer, got %T", name, v)
}

func requireFloat(args map[string]any, name string) (float64, error) {
	v, ok := args[name]
	if !ok {
		return 0, fmt.Errorf("missing required arg %q", name)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	}
	return 0, fmt.Errorf("arg %q: expected number, got %T", name, v)
}

func requireBool(args map[string]any, name string) (bool, error) {
	v, ok := args[name]
	if !ok {
		return false, fmt.Errorf("missing required arg %q", name)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("arg %q: expected bool, got %T", name, v)
	}
	return b, nil
}

func optionalString(args map[string]any, name string) (string, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}

func optionalInt(args map[string]any, name string) (int, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}

func optionalFloat(args map[string]any, name string) (float64, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func optionalBool(args map[string]any, name string) (bool, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
`
