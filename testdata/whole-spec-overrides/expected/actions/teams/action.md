+++
name = "teams"
version = "0.0.0-dev"
source = "github://example/connector-overrides/actions/teams@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-overrides"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["teams"]

[match]
intent = "List teams the viewer belongs to."

[[execute]]
id = "teams"
connector = "github://example/connector-overrides"
op = "teams"
idempotent = true
+++
