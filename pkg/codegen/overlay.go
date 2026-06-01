package codegen

// Overlay captures per-operation governance metadata (idempotency, approval,
// credential scheme, capabilities) layered on top of a Spec. The shape is
// provisional until two greenfield connectors have shipped without
// breaking-change releases (acceptance criterion in issue #893).
type Overlay struct{}

// LoadOverlay reads a gen.yaml file at path and parses it into Overlay.
func LoadOverlay(path string) (Overlay, error) {
	return Overlay{}, nil
}
