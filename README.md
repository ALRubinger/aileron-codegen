# aileron-codegen

Build-time codegen that turns an OpenAPI or GraphQL specification (plus a small `gen.yaml` overlay) into an Aileron connector scaffold:

- Per-action `action.md` with `[match]`, `[[execute]]`, the `idempotent` flag (ADR-0010), and an `[approval]` block (ADR-0009) where required.
- Connector `manifest.toml` capability surface.
- Typed Go fetch code via [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (OpenAPI) and [genqlient](https://github.com/Khan/genqlient) (GraphQL).

The generator runs at build time only. Generic typed-client codegen is delegated upstream. The differentiated value here is the governance-metadata mapping from a spec operation plus a small overlay to a policy-annotated action manifest.

Tracking issue: [ALRubinger/aileron#893](https://github.com/ALRubinger/aileron/issues/893).

## Status

`v0.0.x` — scaffolding only. The CLI accepts `--spec`, `--overlay`, `--out` and exits 0 without emitting files. Emitter work lands in follow-up PRs.

## Usage

```
aileron-codegen --spec <spec.{yaml,graphql}> --overlay <gen.yaml> --out <dir>
```

## Development

```
task build   # build the CLI into build/aileron-codegen
task test    # run unit tests with coverage
task vet     # go vet
task lint    # staticcheck
task ci      # full local gate (build + test + vet + lint)
```

## License

Apache-2.0 — see [LICENSE](LICENSE).
