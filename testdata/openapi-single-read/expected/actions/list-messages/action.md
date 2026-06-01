+++
name = "list-messages"
version = "0.0.0-dev"
source = "github://ALRubinger/aileron-connector-example/actions/list-messages@0.0.0-dev"

[[requires.connectors]]
name = "github://ALRubinger/aileron-connector-example"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["list_messages"]

[match]
intent = "list recent messages"

[[execute]]
id = "list"
connector = "github://ALRubinger/aileron-connector-example"
op = "list_messages"
idempotent = true

[[inputs]]
name = "limit"
type = "integer"
description = "Maximum number of messages to return."
required = false
+++
