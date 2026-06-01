// Package codegen loads OpenAPI/GraphQL specs plus a per-operation overlay
// and emits Aileron connector scaffolding (action.md, manifest.toml, typed
// clients).
//
// Step 2 implements just the slice needed to drive single-operation
// action.md emission from an OpenAPI spec. The OpenAPI parser is the
// minimal subset (paths, operations, parameters, JSON request body). Spec
// fields not consumed by the action.md emitter are intentionally ignored;
// further coverage lands with the emitters that need it.
package codegen

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is the parsed representation of an OpenAPI (and eventually GraphQL)
// specification — operation-by-operation, with only the fields consumed by
// the emitters.
type Spec struct {
	Operations []Operation
}

// Operation is one HTTP operation pinned to a method + path. Fields map
// directly to the action.md surface.
type Operation struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Parameters  []Parameter
	RequestBody *RequestBody
}

// Parameter is one OpenAPI parameter or required body field, surfaced as an
// [[inputs]] block in the emitted action.md.
type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// RequestBody captures the JSON body schema for an operation. Nil for
// operations without one.
type RequestBody struct {
	Fields []Parameter
}

// LoadSpec reads an OpenAPI YAML file at path and returns its operations
// sorted by operationId for deterministic emission.
func LoadSpec(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read spec: %w", err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Spec{}, fmt.Errorf("parse spec: %w", err)
	}
	return doc.toSpec(), nil
}

type openAPIDoc struct {
	Paths map[string]openAPIPath `yaml:"paths"`
}

type openAPIPath struct {
	Get    *openAPIOperation `yaml:"get"`
	Put    *openAPIOperation `yaml:"put"`
	Post   *openAPIOperation `yaml:"post"`
	Delete *openAPIOperation `yaml:"delete"`
	Patch  *openAPIOperation `yaml:"patch"`
}

type openAPIOperation struct {
	OperationID string              `yaml:"operationId"`
	Summary     string              `yaml:"summary"`
	Parameters  []openAPIParameter  `yaml:"parameters"`
	RequestBody *openAPIRequestBody `yaml:"requestBody"`
}

type openAPIParameter struct {
	Name        string        `yaml:"name"`
	In          string        `yaml:"in"`
	Description string        `yaml:"description"`
	Required    bool          `yaml:"required"`
	Schema      openAPISchema `yaml:"schema"`
}

type openAPISchema struct {
	Type        string                   `yaml:"type"`
	Description string                   `yaml:"description"`
	Properties  map[string]openAPISchema `yaml:"properties"`
	Required    []string                 `yaml:"required"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIMediaType struct {
	Schema openAPISchema `yaml:"schema"`
}

func (d openAPIDoc) toSpec() Spec {
	var ops []Operation
	for path, p := range d.Paths {
		for method, op := range p.byMethod() {
			ops = append(ops, op.toOperation(method, path))
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	return Spec{Operations: ops}
}

func (p openAPIPath) byMethod() map[string]openAPIOperation {
	out := map[string]openAPIOperation{}
	if p.Get != nil {
		out["GET"] = *p.Get
	}
	if p.Put != nil {
		out["PUT"] = *p.Put
	}
	if p.Post != nil {
		out["POST"] = *p.Post
	}
	if p.Delete != nil {
		out["DELETE"] = *p.Delete
	}
	if p.Patch != nil {
		out["PATCH"] = *p.Patch
	}
	return out
}

func (op openAPIOperation) toOperation(method, path string) Operation {
	params := make([]Parameter, 0, len(op.Parameters))
	for _, p := range op.Parameters {
		params = append(params, Parameter{
			Name:        p.Name,
			Type:        p.Schema.Type,
			Description: p.Description,
			Required:    p.Required,
		})
	}
	out := Operation{
		ID:         op.OperationID,
		Method:     strings.ToUpper(method),
		Path:       path,
		Summary:    op.Summary,
		Parameters: params,
	}
	if op.RequestBody != nil {
		if media, ok := op.RequestBody.Content["application/json"]; ok {
			out.RequestBody = &RequestBody{Fields: media.Schema.toFields()}
		}
	}
	return out
}

func (s openAPISchema) toFields() []Parameter {
	required := map[string]bool{}
	for _, name := range s.Required {
		required[name] = true
	}
	fields := make([]Parameter, 0, len(s.Properties))
	for name, prop := range s.Properties {
		fields = append(fields, Parameter{
			Name:        name,
			Type:        prop.Type,
			Description: prop.Description,
			Required:    required[name],
		})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields
}
