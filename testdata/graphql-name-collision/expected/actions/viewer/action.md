+++
name = "viewer"
version = "0.0.0-dev"
source = "github://example/connector-collision/actions/viewer@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-collision"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["viewer"]

[match]
intent = "A non-colliding query — verifies the suffix only fires on collision."

[[execute]]
id = "viewer"
connector = "github://example/connector-collision"
op = "viewer"
idempotent = true
+++
