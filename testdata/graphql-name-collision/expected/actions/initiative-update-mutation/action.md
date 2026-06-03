+++
name = "initiative-update-mutation"
version = "0.0.0-dev"
source = "github://example/connector-collision/actions/initiative-update-mutation@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-collision"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["initiative_update_mutation"]

[match]
intent = "Create or update an initiative update — collides with Query.initiativeUpdate."

[[execute]]
id = "initiative"
connector = "github://example/connector-collision"
op = "initiative_update_mutation"
idempotent = false

[approval]
required = true

[[inputs]]
name = "body"
type = "String"
description = "Markdown body of the update."
required = true
+++
