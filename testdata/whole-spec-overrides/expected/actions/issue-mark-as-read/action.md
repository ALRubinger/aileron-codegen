+++
name = "issue-mark-as-read"
version = "0.0.0-dev"
source = "github://example/connector-overrides/actions/issue-mark-as-read@0.0.0-dev"

[[requires.connectors]]
name = "github://example/connector-overrides"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["issue_mark_as_read"]

[match]
intent = "Mark an issue as read. Reversible client-side, so no per-call approval needed."

[[execute]]
id = "issue"
connector = "github://example/connector-overrides"
op = "issue_mark_as_read"
idempotent = true

[[inputs]]
name = "id"
type = "String"
description = "ID of the issue to mark as read."
required = true
+++
