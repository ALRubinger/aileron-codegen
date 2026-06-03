+++
name = "list-things"
version = "0.0.0-dev"
source = "github://example/connector-defaults/actions/list-things@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-defaults"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["list_things"]

[match]
intent = "list things"

[[execute]]
id = "list"
connector = "github://example/connector-defaults"
op = "list_things"
idempotent = true
+++
