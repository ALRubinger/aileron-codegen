// Package codegen loads OpenAPI/GraphQL specs plus a per-operation overlay
// and emits Aileron connector scaffolding (action.md, manifest.toml, typed
// clients).
//
// The exported surface is small on purpose: Spec, Operation, Parameter,
// RequestBody plus the LoadSpec / LoadOverlay / Generate entry points. The
// OpenAPI and GraphQL parsers live in sibling files and feed into the same
// Operation shape so the emitter stays format-agnostic.
package codegen

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Spec is the parsed representation of an OpenAPI or GraphQL specification
// — operation-by-operation, with only the fields consumed by the emitters.
type Spec struct {
	Operations []Operation
}

// Operation is one spec operation surfaced to the emitter. For OpenAPI this
// is one HTTP method + path; for GraphQL one Query/Mutation root field.
type Operation struct {
	// ID is the canonical operation identifier — drives every downstream
	// Go identifier (handler name, query const) and emitted artifact name
	// (action directory, snake_case op string). For OpenAPI this is the
	// operationId. For GraphQL this is the root-field name, suffixed with
	// "Mutation" when the same field name also appears on type Query (and
	// similarly with "Subscription" for three-way collisions) so that the
	// emitted Go and action names stay unique. The raw GraphQL field name
	// (which the emitted query string must use to call the field) lives in
	// FieldName.
	ID string
	// FieldName is the raw GraphQL root-field name for GraphQL operations.
	// Empty for OpenAPI. The handler emitter uses this when writing the
	// GraphQL document so the field invocation matches the schema even
	// when ID has been suffixed to disambiguate a Query/Mutation collision.
	FieldName string
	// Method is "GET"/"POST"/... for OpenAPI or "QUERY"/"MUTATION" for
	// GraphQL.
	Method string
	// Path is the OpenAPI URL path (empty for GraphQL operations).
	Path string
	// Summary is the human-readable one-line description (OpenAPI summary
	// or GraphQL field description).
	Summary string
	// Parameters are out-of-body inputs (OpenAPI query/path/header params
	// or GraphQL field arguments).
	Parameters []Parameter
	// RequestBody is the JSON body schema for OpenAPI operations; nil when
	// absent or for GraphQL.
	RequestBody *RequestBody
	// ReturnType is the named GraphQL return type (e.g. "Issue") or the
	// OpenAPI response schema name (when populated by the loader). Lists
	// and non-null wrappers are unwrapped to the inner named type;
	// empty when the loader cannot determine one (OpenAPI today).
	ReturnType string
	// ReturnTypeIsScalar is true when ReturnType is a GraphQL scalar /
	// enum / built-in (String, Int, Boolean, ID, custom DateTime, etc).
	// Handler emitters use this to skip building a selection set and
	// just request the value directly.
	ReturnTypeIsScalar bool
	// ReturnFields is the (sorted) selection tree for the return type.
	// Scalar fields appear as leaves (Nested empty); fields whose own
	// type is itself an OBJECT / INTERFACE recurse exactly one level —
	// their scalar fields are surfaced as Nested entries so handler
	// queries get the canonical Linear-style "wrapper { success,
	// lastSyncId, entity { id, ... } }" shape. Empty for scalar
	// returns, unions, and unknown types.
	ReturnFields []ReturnField
	// ArgTypes maps each GraphQL field argument name to its full GraphQL
	// type string (e.g., "String!", "IssueCreateInput!", "[String!]").
	// Used by the handler emitter to build the variables declaration in
	// the emitted GraphQL query string. Empty for OpenAPI ops.
	ArgTypes map[string]string
}

// ReturnField is one entry in the GraphQL selection set for an
// operation's return type. Leaves (scalars / enums) have Nested
// empty; object-typed fields have Nested populated with their own
// scalar leaves. Recursion stops at one level (Nested entries cannot
// themselves have Nested entries) — deeper recursion blows up query
// payloads and risks server-side depth limits. Connectors that need
// deeper selection can declare it per-op via overlay in a future PR.
type ReturnField struct {
	Name   string
	Nested []ReturnField
}

// Parameter is one input surfaced as an [[inputs]] block in the emitted
// action.md.
type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	// WrappedIn names the original GraphQL argument when this Parameter
	// came from flattening a GraphQL input object (e.g. `input` for
	// `issueCreate(input: IssueCreateInput!)`). Empty when the param
	// was a direct scalar arg. The action.md emitter ignores it; the
	// handler emitter uses it to rebuild the wrapping JSON shape the
	// API expects.
	WrappedIn string
}

// RequestBody captures the JSON body schema for an OpenAPI operation. Nil
// for operations without one and for all GraphQL operations.
type RequestBody struct {
	Fields []Parameter
}

// LoadSpec dispatches by file extension: .graphql/.graphqls/.gql route to
// the GraphQL loader, .yaml/.yml/.json to the OpenAPI loader. Both produce
// the same Spec shape so the emitter stays format-agnostic.
func LoadSpec(path string) (Spec, error) {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".graphql", ".graphqls", ".gql":
		return loadGraphQLSpec(path)
	case ".yaml", ".yml", ".json":
		return loadOpenAPISpec(path)
	default:
		return Spec{}, fmt.Errorf("unsupported spec extension %q for %s", ext, path)
	}
}
