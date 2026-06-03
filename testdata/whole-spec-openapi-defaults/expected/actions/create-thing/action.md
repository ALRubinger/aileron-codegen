+++
name = "create-thing"
version = "0.0.0-dev"
source = "github://example/connector-defaults/actions/create-thing@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-defaults"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["create_thing"]

[match]
intent = "create a thing"

[[execute]]
id = "create"
connector = "github://example/connector-defaults"
op = "create_thing"
idempotent = false

[approval]
required = true

[[inputs]]
name = "name"
type = "string"
description = "Thing name."
required = true
+++
