package codegen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Overlay is the parsed gen.yaml — connector-level metadata plus a
// per-operation map keyed by OpenAPI operationId (or GraphQL field name in
// future loaders).
//
// The shape is provisional until two greenfield connectors have shipped
// without breaking-change releases (acceptance criterion in
// ALRubinger/aileron#893).
type Overlay struct {
	Connector  ConnectorOverlay
	Operations map[string]OperationOverlay
}

// ConnectorOverlay names the connector and declares its credential surface.
// Credential is connector-scoped because Aileron connectors mediate exactly
// one credential per WASM binary; per-operation credential would invite
// surface-area drift.
type ConnectorOverlay struct {
	Name       string
	Credential CredentialOverlay
}

// CredentialOverlay declares the credential kind the host injects at the
// network boundary (e.g. "oauth2", "api_key").
type CredentialOverlay struct {
	Kind string
}

// OperationOverlay carries governance metadata for one spec operation.
type OperationOverlay struct {
	// Idempotent declares whether retries are safe (ADR-0010).
	Idempotent bool
	// Approval is "required" to gate per-call (ADR-0009), "none" otherwise.
	Approval string
	// Capabilities is the capability identifier list this operation needs.
	Capabilities []string
	// Intent overrides the spec's operation summary when set.
	Intent string
	// ActionName overrides the kebab-cased operationId when set.
	ActionName string
	// ExecuteID overrides the first-word default for the [[execute]] id.
	ExecuteID string
}

// LoadOverlay reads a gen.yaml file at path and parses it into Overlay.
func LoadOverlay(path string) (Overlay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Overlay{}, fmt.Errorf("read overlay: %w", err)
	}
	var raw overlayDoc
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Overlay{}, fmt.Errorf("parse overlay: %w", err)
	}
	return raw.toOverlay(), nil
}

type overlayDoc struct {
	Connector  connectorYAML            `yaml:"connector"`
	Operations map[string]operationYAML `yaml:"operations"`
}

type connectorYAML struct {
	Name       string         `yaml:"name"`
	Credential credentialYAML `yaml:"credential"`
}

type credentialYAML struct {
	Kind string `yaml:"kind"`
}

type operationYAML struct {
	Idempotent   bool     `yaml:"idempotent"`
	Approval     string   `yaml:"approval"`
	Capabilities []string `yaml:"capabilities"`
	Intent       string   `yaml:"intent"`
	ActionName   string   `yaml:"action_name"`
	ExecuteID    string   `yaml:"execute_id"`
}

func (d overlayDoc) toOverlay() Overlay {
	ops := make(map[string]OperationOverlay, len(d.Operations))
	for k, v := range d.Operations {
		ops[k] = OperationOverlay(v)
	}
	return Overlay{
		Connector: ConnectorOverlay{
			Name:       d.Connector.Name,
			Credential: CredentialOverlay(d.Connector.Credential),
		},
		Operations: ops,
	}
}
