# AGENTS.md

Operator notes for AI agents (and humans) working in this repo. Read this first.

## What this repo is

`aileron-codegen` is a build-time codegen package that turns an OpenAPI or GraphQL spec plus a small `gen.yaml` overlay into Aileron connector scaffolding (`action.md`, eventually `manifest.toml` + typed Go client). The differentiated value is the **governance-metadata mapping** — spec operation + overlay → policy-annotated action manifest — not the typed-client codegen (that is delegated upstream to [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) and [genqlient](https://github.com/Khan/genqlient)).

The generator runs at build time only. Connector binaries never see codegen.

**Tracking issue:** [ALRubinger/aileron#893](https://github.com/ALRubinger/aileron/issues/893) (codegen umbrella). The first connector that consumes this package is tracked at [ALRubinger/aileron#915](https://github.com/ALRubinger/aileron/issues/915) (Linear). Read both before starting work.

## Status

| Capability | State |
|---|---|
| OpenAPI spec loading | shipped — `.yaml` / `.yml` / `.json` |
| GraphQL SDL loading | shipped — `.graphql` / `.graphqls` / `.gql` |
| `action.md` emission | shipped (TOML front-matter only; prose body deferred) |
| `connector/manifest.toml` emission | shipped — `[connector]` + `[capabilities.network]` + `[capabilities.credential]` for `api_key` and `oauth2` kinds |
| `suite.toml` emission | shipped — optional, no-op when overlay lacks `suite:` block |
| Whole-spec default action emission | shipped — every op in the spec emits with kind-based defaults; `operations:` is override-only; `exclude:` drops unwanted ops; unknown overlay op ids are rejected as typos |
| Typed Go fetch client | **not shipped** — delegate to oapi-codegen / genqlient when wired |
| Multi-spec / multi-tenant | **out of scope** |

## Local development

```sh
task build   # ./build/aileron-codegen
task test    # go test with -coverprofile=coverage.out
task vet     # go vet ./...
task lint    # staticcheck (pinned to 2025.1.1)
task ci      # full gate: build + test + vet + lint
```

`task ci` mirrors the GitHub Actions workflow. **Run it locally before pushing.** The hosted CI runs the same gate plus a Codecov upload (no-ops without `CODECOV_TOKEN` configured on the repo).

## Testing philosophy

Per the Aileron project rule, **tests assert on the contract, not the implementation**. For this repo that means:

- **Golden-file tests.** Each `testdata/<case>/` directory contains `spec.{yaml,graphql}` + `gen.yaml` + `expected/`. The harness in `pkg/codegen/codegen_test.go` runs `Generate` and diffs the emitted tree byte-for-byte. If output changes, the golden updates.
- **No implementation-mirror assertions.** Do not write tests like "the parser builds an AST with N nodes" — assert on emitted file content.
- **Refactor survival.** Internal restructuring (e.g. PR #3 moved OpenAPI parsing out of `spec.go` into `openapi.go`) should leave all golden tests green. If a refactor breaks a golden, the contract changed — check whether that was intentional.
- **Bug fixes get a regression test.** Add a case under `testdata/` whose `expected/` would have failed before the fix.

The existing golden cases:

| Case | Format | Exercises |
|---|---|---|
| `empty/` | OpenAPI (empty) | Round-trip baseline — no operations, no emitted files |
| `openapi-single-write/` | OpenAPI | POST + JSON body, `approval: required` emits `[approval]` block |
| `openapi-single-read/` | OpenAPI | GET + query param, `approval: none` omits `[approval]` block |
| `graphql-linear-read/` | GraphQL | Query field, scalar arg, all three overlay overrides exercised |
| `graphql-linear-write/` | GraphQL | Mutation + input-object flattening into multiple `[[inputs]]` |
| `connector-full-graphql/` | GraphQL | Full stack: action.md per op + `connector/manifest.toml` (api_key with `header`/`format`) + `suite.toml` |
| `whole-spec-graphql-defaults/` | GraphQL | One query + one mutation, no `operations:` block — kind-based defaults emit both with correct (idempotent, approval) pair |
| `whole-spec-openapi-defaults/` | OpenAPI | GET + POST + DELETE, no `operations:` block — method-based defaults table |
| `whole-spec-overrides/` | GraphQL | Query uses defaults, mutation overrides idempotent + approval — verifies override-on-top-of-default semantics |

When you add a new emitter or spec loader, add one case per shape that materially changes the output.

## Load-bearing placeholders — do not substitute

The emitted `action.md` contains two strings that **must** appear verbatim. The CI release workflow in the consuming connector repo substitutes them at release time per ADR-0002:

- `version = "0.0.0-dev"` — substituted with the pushed tag.
- `source = "...@0.0.0-dev"` — same substitution applied inside the URL.
- `hash = "sha256:bound-at-release"` in `[[requires.connectors]]` — substituted with the content-addressed connector hash.

If you change the placeholders, the substitution breaks silently and signed manifests ship with `0.0.0-dev`. Don't.

## Naming derivations

The emitter derives action shape from the spec operation identifier. Overrides live in the per-operation overlay block.

| Output | Default | Overlay key |
|---|---|---|
| Action directory + manifest `name` | `kebab-case(operationId)` | `action_name` |
| `op` in `[[execute]]` | `snake_case(operationId)` | (none — always derived) |
| `id` in `[[execute]]` | first lower-cased word of `operationId` | `execute_id` |
| `intent` in `[match]` | OpenAPI `summary` / GraphQL field description | `intent` |

Use overrides when the spec's natural id produces an awkward action (e.g. GraphQL's single-word `issue` query becomes `get-issue` under override). Avoid renaming the spec.

## Where to add things

| Adding... | File |
|---|---|
| A new emitter | New file in `pkg/codegen/`, implement `Emitter` interface, register in `defaultEmitters` in `emitter.go`. **No-op cleanly when the overlay lacks your inputs** — that's how `ManifestEmitter` and `SuiteEmitter` coexist with action-only test cases. |
| A new spec format | New `load<Format>Spec` function in its own file, dispatch in `LoadSpec` in `spec.go` |
| A new overlay field | The `*Overlay` struct in `overlay.go` (typed surface) plus the mirror `*YAML` struct (with `yaml:` tags); add the field to `toOverlay()` |
| A new test case | `testdata/<case>/` with `spec.{yaml,graphql}`, `gen.yaml`, `expected/` — `TestGolden` picks it up automatically |

## Workflow conventions

- **Branch + PR for everything except this file.** Use conventional-commit titles (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`).
- **Squash-merge.** The repo is admin-merge-authorized; use `gh pr merge <N> --squash --admin --delete-branch`.
- **Wait for CI before merging.** Even with `--admin`, never merge while a check is pending.
- **No backwards-compat shims.** Pre-release. Just change the code.
- **No emojis in generated output or in this repo's code/docs unless explicitly requested.**

## Out-of-band context

This repo intentionally keeps the design conversation in commit history + PR descriptions + the umbrella tracking issue. There are no ADRs in this repo (yet — they live in `ALRubinger/aileron/docs/adr/` and are referenced by number: ADR-0002 connector identity, ADR-0009 user channel / approval, ADR-0010 idempotency). If a decision feels load-bearing enough to ADR, write it under the main `aileron` repo and link back from the PR that implements it.
