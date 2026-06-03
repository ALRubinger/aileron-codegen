+++
name = "get-issue"
version = "0.0.0-dev"
source = "github://ALRubinger/aileron-connector-linear/actions/get-issue@0.0.0-dev"

[[requires.connectors]]
name = "github://ALRubinger/aileron-connector-linear"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["get_issue"]

[match]
intent = "fetch a Linear issue by id"

[[execute]]
id = "get"
connector = "github://ALRubinger/aileron-connector-linear"
op = "issue"
idempotent = true

[[inputs]]
name = "id"
type = "String"
description = "Unique identifier (UUID) of the Linear issue."
required = true
+++
