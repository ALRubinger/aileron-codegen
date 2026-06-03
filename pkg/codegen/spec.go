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
	// ID is the canonical operation identifier (OpenAPI operationId, or the
	// GraphQL root-field name).
	ID string
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
	// ReturnScalarFields is the (sorted) list of scalar-typed field
	// names on the return type. The GraphQL handler emitter uses this
	// to build a default selection set; the OpenAPI emitter ignores it
	// today. Empty for scalar returns, unions, and interfaces.
	ReturnScalarFields []string
	// ArgTypes maps each GraphQL field argument name to its full GraphQL
	// type string (e.g., "String!", "IssueCreateInput!", "[String!]").
	// Used by the handler emitter to build the variables declaration in
	// the emitted GraphQL query string. Empty for OpenAPI ops.
	ArgTypes map[string]string
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
