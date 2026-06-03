+++
name = "viewer"
version = "0.0.0-dev"
source = "github://example/connector-defaults/actions/viewer@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-defaults"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["viewer"]

[match]
intent = "Get the currently authenticated user."

[[execute]]
id = "viewer"
connector = "github://example/connector-defaults"
op = "viewer"
idempotent = true
+++
