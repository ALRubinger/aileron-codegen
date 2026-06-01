package codegen

import "os"

// Options configures a Generate call.
type Options struct {
	// SpecPath points at an OpenAPI YAML or GraphQL schema file.
	SpecPath string
	// OverlayPath points at the gen.yaml overlay describing per-operation
	// governance metadata.
	OverlayPath string
	// OutDir is the directory into which scaffolding is written. It is
	// created if it does not exist.
	OutDir string
}

// Emitter writes connector scaffolding derived from a Spec and Overlay into
// an output directory. One implementation per output kind lands as the
// emitters are built: action.md, manifest.toml, suite.toml, typed Go client.
type Emitter interface {
	Emit(spec Spec, overlay Overlay, outDir string) error
}

// Generate is the high-level entry point used by the CLI. It loads the spec
// and overlay, then dispatches to the registered emitters. With no emitters
// registered yet, it ensures OutDir exists and returns.
func Generate(opts Options) error {
	_, _ = LoadSpec(opts.SpecPath)
	_, _ = LoadOverlay(opts.OverlayPath)
	return os.MkdirAll(opts.OutDir, 0o755)
}
