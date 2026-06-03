+++
name = "initiative-update"
version = "0.0.0-dev"
source = "github://example/connector-collision/actions/initiative-update@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-collision"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["initiative_update"]

[match]
intent = "Fetch one initiative update by id."

[[execute]]
id = "initiative"
connector = "github://example/connector-collision"
op = "initiative_update"
idempotent = true

[[inputs]]
name = "id"
type = "String"
description = ""
required = true
+++
