package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// loadGraphQLSpec reads a GraphQL SDL file and returns its Query and
// Mutation root fields as Operations, sorted by name for deterministic
// emission.
//
// Each top-level field argument becomes a Parameter. Arguments whose type
// is a GraphQL input object are flattened one level: the input object's
// fields become individual Parameters. Deeper nesting and list-of-input
// patterns are out of scope until a real connector needs them.
func loadGraphQLSpec(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read spec: %w", err)
	}
	schema, gqlErr := gqlparser.LoadSchema(&ast.Source{
		Name:  filepath.Base(path),
		Input: string(data),
	})
	if gqlErr != nil {
		return Spec{}, fmt.Errorf("parse spec: %w", gqlErr)
	}
	var ops []Operation
	ops = append(ops, rootOperations(schema.Query, "QUERY", schema)...)
	ops = append(ops, rootOperations(schema.Mutation, "MUTATION", schema)...)
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	return Spec{Operations: ops}, nil
}

func rootOperations(root *ast.Definition, kind string, schema *ast.Schema) []Operation {
	if root == nil {
		return nil
	}
	var ops []Operation
	for _, f := range root.Fields {
		// gqlparser injects synthetic introspection fields (__schema, __type)
		// at the Query root. Skip them — they're not part of the user-facing
		// API surface.
		if strings.HasPrefix(f.Name, "__") {
			continue
		}
		ops = append(ops, graphqlFieldToOperation(f, kind, schema))
	}
	return ops
}

func graphqlFieldToOperation(f *ast.FieldDefinition, kind string, schema *ast.Schema) Operation {
	var params []Parameter
	argTypes := make(map[string]string, len(f.Arguments))
	for _, arg := range f.Arguments {
		argTypes[arg.Name] = arg.Type.String()
		params = append(params, expandGraphQLArg(arg, schema)...)
	}
	sort.Slice(params, func(i, j int) bool { return params[i].Name < params[j].Name })

	retName, retIsScalar, retScalarFields := resolveReturnType(f.Type, schema)

	op := Operation{
		ID:                 f.Name,
		Method:             kind,
		Summary:            f.Description,
		Parameters:         params,
		ReturnType:         retName,
		ReturnTypeIsScalar: retIsScalar,
		ReturnScalarFields: retScalarFields,
	}
	if len(argTypes) > 0 {
		op.ArgTypes = argTypes
	}
	return op
}

// resolveReturnType walks the GraphQL field's return type and returns
// (named, isScalar, scalarFields).
//
//   - named is the unwrapped named type (e.g. "Issue" for `Issue!` or
//     `[Issue!]!`). Empty when the parser doesn't know the type.
//   - isScalar is true when the named type is SCALAR or ENUM. Handler
//     emitters skip building a selection set in that case.
//   - scalarFields is the sorted list of field names on the return
//     object whose own type unwraps to a scalar/enum. The handler
//     emitter uses these to build a default selection set. Empty for
//     scalar returns, unions, and interfaces (selection on unions /
//     interfaces requires `__typename` + per-variant fragments — out
//     of scope until a real connector needs them).
func resolveReturnType(t *ast.Type, schema *ast.Schema) (string, bool, []string) {
	name := unwrapNamedType(t)
	if name == "" {
		return "", false, nil
	}
	def, ok := schema.Types[name]
	if !ok {
		return name, false, nil
	}
	switch def.Kind {
	case ast.Scalar, ast.Enum:
		return name, true, nil
	case ast.Object, ast.Interface:
		var fields []string
		for _, f := range def.Fields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			inner := unwrapNamedType(f.Type)
			innerDef, ok := schema.Types[inner]
			if !ok {
				continue
			}
			if innerDef.Kind == ast.Scalar || innerDef.Kind == ast.Enum {
				fields = append(fields, f.Name)
			}
		}
		sort.Strings(fields)
		return name, false, fields
	}
	return name, false, nil
}

// expandGraphQLArg returns the [[inputs]] entries for one GraphQL argument.
// Scalar / enum / list args produce a single Parameter; arguments typed as
// an input object are flattened into their fields, with WrappedIn set to
// the original arg name so the handler emitter can rebuild the wrapping.
func expandGraphQLArg(arg *ast.ArgumentDefinition, schema *ast.Schema) []Parameter {
	named := unwrapNamedType(arg.Type)
	if typeDef, ok := schema.Types[named]; ok && typeDef.Kind == ast.InputObject {
		fields := make([]Parameter, 0, len(typeDef.Fields))
		for _, inputField := range typeDef.Fields {
			fields = append(fields, Parameter{
				Name:        inputField.Name,
				Type:        unwrapNamedType(inputField.Type),
				Description: inputField.Description,
				Required:    inputField.Type.NonNull,
				WrappedIn:   arg.Name,
			})
		}
		return fields
	}
	return []Parameter{{
		Name:        arg.Name,
		Type:        named,
		Description: arg.Description,
		Required:    arg.Type.NonNull,
	}}
}

// unwrapNamedType returns the underlying named type for a GraphQL type
// reference, peeling off list wrappers. The "named" form is what the
// emitter writes into action.md `type = "..."`.
func unwrapNamedType(t *ast.Type) string {
	if t == nil {
		return ""
	}
	if t.Elem != nil {
		return unwrapNamedType(t.Elem)
	}
	return t.NamedType
}
