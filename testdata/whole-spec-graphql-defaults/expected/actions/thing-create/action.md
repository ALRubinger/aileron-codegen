+++
name = "thing-create"
version = "0.0.0-dev"
source = "github://example/connector-defaults/actions/thing-create@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-defaults"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["thing_create"]

[match]
intent = "Create a thing."

[[execute]]
id = "thing"
connector = "github://example/connector-defaults"
op = "thing_create"
idempotent = false

[approval]
required = true

[[inputs]]
name = "name"
type = "String"
description = "Display name for the new thing."
required = true
+++
