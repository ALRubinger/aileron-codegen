// Package codegen loads OpenAPI/GraphQL specs plus a per-operation overlay
// and emits Aileron connector scaffolding (action.md, manifest.toml, typed
// clients).
//
// This is the scaffolding pass: the public surface (Options, Generate, Spec,
// Overlay, Emitter) is fixed so callers can integrate. The spec loader,
// overlay parser, and emitters are intentionally stubs and land in
// follow-up PRs.
package codegen

// Spec is the parsed representation of an OpenAPI or GraphQL specification.
// Fields populate as the OpenAPI and GraphQL loaders land.
type Spec struct{}

// LoadSpec reads a spec file at path and parses it into Spec.
func LoadSpec(path string) (Spec, error) {
	return Spec{}, nil
}
