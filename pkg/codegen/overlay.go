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
	// Operations are governance overrides keyed by spec operationId.
	// An entry here is an override on top of the kind-based defaults
	// (Query/Subscription/GET/HEAD/PUT → idempotent, non-write;
	// Mutation/POST/PATCH/DELETE → non-idempotent, approval-required).
	// Operations not declared here still emit, using the defaults. An
	// entry whose operationId does not appear in the spec is a typo
	// and is rejected at emission time.
	Operations map[string]OperationOverlay
	// Exclude drops specific spec operations from emission. Use for
	// introspection helpers, deprecated endpoints, admin/destructive
	// paths the connector deliberately doesn't surface, etc. Listing
	// an operationId here that the spec doesn't define is a typo and
	// is rejected at emission time.
	Exclude []string
	// Suite is optional. When absent, SuiteEmitter is a no-op (some
	// connectors prefer per-action installability without a top-level
	// suite).
	Suite *SuiteOverlay
}

// ConnectorOverlay names the connector and declares its credential +
// network surface.
//
// Credential is connector-scoped because Aileron connectors mediate
// exactly one credential per WASM binary; per-operation credential would
// invite surface-area drift.
type ConnectorOverlay struct {
	Name       string
	Publisher  string
	Credential CredentialOverlay
	Network    NetworkOverlay
}

// CredentialOverlay declares the credential kind plus the per-kind config
// the manifest emitter renders.
//
// Fields are flat by design — yaml.v3 doesn't support tagged unions
// cleanly. The kind decides which fields render into the emitted
// manifest.toml: api_key uses Header/Format; oauth2 uses
// AuthorizeURL/TokenURL/ClientID/ClientSecret/Scopes.
type CredentialOverlay struct {
	Kind string

	// api_key kind fields. Defaults applied at emission time:
	// Header  = "Authorization"
	// Format  = "Bearer {key}"
	// These defaults match the aileron runtime's current hard-coded
	// injection (internal/sandbox/host.go), so existing connectors that
	// don't set them keep working once the runtime reads them.
	Header string
	Format string

	// oauth2 kind fields. Render under [capabilities.credential.oauth2]
	// per the Google connector's existing manifest shape.
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// NetworkOverlay enumerates the host:port pairs the connector is allowed
// to dial. Renders into [capabilities.network] in the emitted manifest.
type NetworkOverlay struct {
	Hosts []string
}

// SuiteOverlay drives the top-level suite.toml. Action paths are computed
// at emission time from the same filter ActionEmitter applies, so the
// suite stays in sync with the emitted actions/.
type SuiteOverlay struct {
	Name        string
	Description string
}

// OperationOverlay carries per-operation overrides on top of the
// kind-based governance defaults (Query/Subscription/GET/HEAD/PUT →
// idempotent, no approval; Mutation/POST/PATCH/DELETE → non-idempotent,
// approval-required). Every field is optional: an empty field means
// "use the default."
//
// Booleans use a pointer to distinguish "user explicitly set this" from
// "user did not set this" — yaml.v3 leaves a *bool nil when the key is
// absent and points at the parsed value when present.
type OperationOverlay struct {
	// Idempotent (when non-nil) overrides whether retries are safe
	// (ADR-0010). Nil means use the default for the operation's method.
	Idempotent *bool
	// Approval is "required" to gate per-call (ADR-0009), "none" to
	// relax. Empty means use the default for the operation's method.
	Approval string
	// Capabilities is the capability identifier list this operation
	// needs. Empty means default to [snake_case(operationId)].
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
	Exclude    []string                 `yaml:"exclude"`
	Suite      *suiteYAML               `yaml:"suite"`
}

type connectorYAML struct {
	Name       string         `yaml:"name"`
	Publisher  string         `yaml:"publisher"`
	Credential credentialYAML `yaml:"credential"`
	Network    networkYAML    `yaml:"network"`
}

type credentialYAML struct {
	Kind         string   `yaml:"kind"`
	Header       string   `yaml:"header"`
	Format       string   `yaml:"format"`
	AuthorizeURL string   `yaml:"authorize_url"`
	TokenURL     string   `yaml:"token_url"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

type networkYAML struct {
	Hosts []string `yaml:"hosts"`
}

type suiteYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type operationYAML struct {
	Idempotent   *bool    `yaml:"idempotent"`
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
	out := Overlay{
		Exclude: append([]string(nil), d.Exclude...),
		Connector: ConnectorOverlay{
			Name:       d.Connector.Name,
			Publisher:  d.Connector.Publisher,
			Credential: CredentialOverlay(d.Connector.Credential),
			Network:    NetworkOverlay(d.Connector.Network),
		},
		Operations: ops,
	}
	if d.Suite != nil {
		s := SuiteOverlay(*d.Suite)
		out.Suite = &s
	}
	return out
}
