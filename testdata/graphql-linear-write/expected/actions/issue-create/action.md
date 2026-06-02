+++
name = "issue-create"
version = "0.0.0-dev"
source = "github://ALRubinger/aileron-connector-linear/actions/issue-create@0.0.0-dev"

[[requires.connectors]]
name = "github://ALRubinger/aileron-connector-linear"
version = "0.0.0-dev"
hash = "sha256:bound-at-release"
capabilities = ["create_issue"]

[match]
intent = "Create a new Linear issue."

[[execute]]
id = "issue"
connector = "github://ALRubinger/aileron-connector-linear"
op = "issue_create"
idempotent = false

[approval]
required = true

[[inputs]]
name = "assigneeId"
type = "String"
description = "UUID of the assignee."
required = false

[[inputs]]
name = "description"
type = "String"
description = "Markdown body of the issue."
required = false

[[inputs]]
name = "priority"
type = "Int"
description = "Priority value: 0 (No priority), 1 (Urgent), 2 (High), 3 (Medium), 4 (Low)."
required = false

[[inputs]]
name = "teamId"
type = "String"
description = "UUID of the target team."
required = true

[[inputs]]
name = "title"
type = "String"
description = "Issue title."
required = true
+++
