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
// an output directory. One implementation per output kind: ActionEmitter
// today, with manifest.toml / suite.toml / typed Go client emitters in
// follow-up PRs.
type Emitter interface {
	Emit(spec Spec, overlay Overlay, outDir string) error
}

// defaultEmitters runs in declared order on every Generate call.
var defaultEmitters = []Emitter{ActionEmitter{}}

// Generate is the high-level entry point used by the CLI. It loads the spec
// and overlay, then dispatches each registered emitter against the parsed
// inputs.
func Generate(opts Options) error {
	spec, err := LoadSpec(opts.SpecPath)
	if err != nil {
		return err
	}
	overlay, err := LoadOverlay(opts.OverlayPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}
	for _, e := range defaultEmitters {
		if err := e.Emit(spec, overlay, opts.OutDir); err != nil {
			return err
		}
	}
	return nil
}
