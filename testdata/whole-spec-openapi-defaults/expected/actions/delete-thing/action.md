+++
name = "delete-thing"
version = "0.0.0-dev"
source = "github://example/connector-defaults/actions/delete-thing@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-defaults"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["delete_thing"]

[match]
intent = "delete a thing"

[[execute]]
id = "delete"
connector = "github://example/connector-defaults"
op = "delete_thing"
idempotent = false

[approval]
required = true

[[inputs]]
name = "id"
type = "string"
description = "Identifier of the thing."
required = true
+++
