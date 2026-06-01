+++
name = "send-message"
version = "0.0.0-dev"
source = "github://ALRubinger/aileron-connector-example/actions/send-message@0.0.0-dev"

[[requires.connectors]]
name = "github://ALRubinger/aileron-connector-example"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["send_message"]

[match]
intent = "send a message to a channel"

[[execute]]
id = "send"
connector = "github://ALRubinger/aileron-connector-example"
op = "send_message"
idempotent = false

[approval]
required = true

[[inputs]]
name = "body"
type = "string"
description = "Message body."
required = true

[[inputs]]
name = "to"
type = "string"
description = "Channel or user the message is sent to."
required = true
+++
