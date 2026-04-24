# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0](https://github.com/tiziano093/infra-composer-cli/compare/v1.0.0...v2.0.0) (2026-04-24)


### ⚠ BREAKING CHANGES

* **compose:** --root-stack flag removed from `compose`; no more main.tf/variables.tf/outputs.tf/providers.tf/versions.tf/locals.tf emitted at the root of --output-dir. Consumers build their own environment layer.

### Features

* add --filter/--all to compose, fix catalog validation, improve interactive UX ([1df0b13](https://github.com/tiziano093/infra-composer-cli/commit/1df0b13f43559440e5d8ea9879d18693dd1f6868))
* add Homebrew formula for v1.0.0 ([59ba979](https://github.com/tiziano093/infra-composer-cli/commit/59ba979245c2c31d7d1d7cafb4d1562000157d4e))
* **compose:** drop --root-stack, emit modules.json ([3e9a028](https://github.com/tiziano093/infra-composer-cli/commit/3e9a028c6c22cf323356dd983db48be27ca4cea1))


### Bug Fixes

* exclude fmt.Fprintf and defer Close from errcheck ([76e28af](https://github.com/tiziano093/infra-composer-cli/commit/76e28afd81fdc2053c8d66ba2a54e37bb223e4c5))
* gofmt formatting on registry and git test files ([7370d69](https://github.com/tiziano093/infra-composer-cli/commit/7370d6933146b5f1cc105096a3358940a0e8d749))
* remove unused errBinaryMissing, use type conversions in fake.go, filepath.Clean in catalog.go ([086a33d](https://github.com/tiziano093/infra-composer-cli/commit/086a33d1ab504a96850606548d20020aad6b4498))
* suppress gosec G304/G301/G306 false positives for CLI file I/O ([8e8cca5](https://github.com/tiziano093/infra-composer-cli/commit/8e8cca55e436fe06b404690e13f57fb6fcba67a6))


### Miscellaneous

* trigger release 2.0.0 ([7520a88](https://github.com/tiziano093/infra-composer-cli/commit/7520a88cd06f87358ffb9abf3fc7dfe6c14393cf))

## [Unreleased]

### Removed
- `compose --root-stack` and the root-stack generator
  (`internal/terraform/rootstack.go`). Rationale: the aggregated
  `main.tf`/`variables.tf`/`outputs.tf`/`providers.tf` emitted at the
  root of `--output-dir` were a thin skeleton that could not match the
  wide range of real consumption patterns (per-env backend.hcl, tfvars
  bundles, `for_each` loops, stack objects). Consumers build their own
  environment layer instead.

### Added
- `modules.json` manifest written at the root of `--output-dir` by
  `compose`. Machine-readable contract describing every composed
  module (folder path, provider, variables, outputs, cross-module
  references) so downstream tooling and human reviewers can discover
  modules without opening each generated folder. Reuses
  `catalog.ModuleEntry` so the format aligns with the existing
  catalog schema.

## [1.0.0] — 2026-04-24

### Added
- Initial repository scaffolding (Phase 1).
- Directory layout per `docs/REPOSITORY_SETUP.md`.
- Go module, Makefile, `.gitignore`, `.golangci.yml`.
- GitHub Actions workflows: test, lint, release, docker.
- Apache 2.0 LICENSE.
- Cobra-based root command with persistent flags
  (`--config`, `--log-level`, `--log-format`, `--format`,
  `--verbose`, `--quiet`).
- `internal/cli`: `CLIError` type and `ExitCode` constants
  (1–10) matching `docs/ARCHITECTURE.md`.
- `internal/config`: Viper-backed configuration loader with the
  documented hierarchy (defaults → `~/.infra-composer/config.yaml`
  → `INFRA_COMPOSER_*` env → CLI flags).
- `internal/output`: structured logger built on `log/slog`
  (text/json handlers, level + quiet support).
- `internal/commands`: shared `Runtime` context plus the first
  subcommand, `version` (text and JSON output).
- Unit tests for errors, logger, config hierarchy and version
  command (all green via `go test ./...`).
- `internal/catalog`: schema types (`Schema`, `ModuleEntry`,
  `Variable`, `Output`, `ModuleType`), JSON parser with
  unknown-field rejection, file loader, and one-pass validator
  reporting all issues with field paths.
- `internal/catalog`: `ParseError` and `ValidationError` types,
  plus `AsValidationError` helper for command-layer mapping.
- `pkg/catalog`: public re-exports of the catalog data types and
  `SchemaVersion` constant for downstream library consumers.
- Test fixtures under `test/fixtures/schemas/` (valid minimal,
  valid full, malformed, missing provider, duplicate modules)
  and 94% coverage on `internal/catalog`.
- `internal/catalog`: keyword search with AND logic over module
  name/group/description, group + type filters, result limit,
  weighted scoring (exact name > substring > group > description >
  fuzzy subsequence) with stable name-ordered tie-breaking.
- `pkg/catalog`: re-exported `Search`, `SearchOptions`,
  `SearchResult`.
- `internal/clierr`: extracted CLI error type and exit code constants
  to a neutral package so subcommand implementations can build
  CLIError values without importing `internal/cli` (which depends on
  `internal/commands`). `internal/cli` keeps thin aliases for
  backward compatibility.
- `internal/commands`: shared `cliError` / `invalidArgs` helpers and
  central mapping of `catalog.Load` failures (not-found, parse
  error, validation error) to the matching CLIError exit codes.
- `search` subcommand: positional keywords, `--schema/--group/
  --type/--limit/--format` flags, falls back to `catalog.schema`
  config value, table and JSON output.
- `catalog` parent command with `validate [path]` subcommand:
  reports either an OK summary (provider, version, module count)
  or every validation issue in one pass (text or JSON), exits with
  ExitValidationFailed when issues are found.
- `internal/catalog/registry`: pluggable registry `Client` interface
  (DiscoverProvider, ListResources, GetResourceSchema), neutral DTOs,
  and a JSON fixture-backed `FakeClient` used by `catalog build` and
  the integration suite. Sentinel errors `ErrProviderNotFound` and
  `ErrResourceNotFound` for precise error mapping.
- `test/fixtures/registry/hashicorp/aws/provider.json` covering one
  data source and two resources for builder / E2E tests.
- `internal/catalog`: `Builder` + `Build()` pipeline (discover → list →
  fetch → normalize → validate) with deterministic module ordering
  (resources before data, alphabetical within group).
- `internal/catalog`: `Export()` writes `schema.json` atomically
  (tmp + rename) with 0644 permissions and a trailing newline; supports
  both `Path` and `Dir` destinations.
- `pkg/catalog`: re-exported `Builder`, `BuildOptions`, `ExportOptions`,
  `NewBuilder`, `Export`, and `SchemaFileName`.
- `catalog build --provider <addr> --output-dir <dir>` subcommand:
  builds a fresh schema from the registry fixtures (configurable via
  `--registry-dir`, default `./catalog/registry`), exports it, and
  reports either a one-line summary or a JSON record. Errors map to
  `ExitFileNotFound` (provider/resource missing) or
  `ExitValidationFailed` (registry produced invalid catalog).
- `catalog list [path]` subcommand: prints provider metadata plus a
  module table (text) or a structured `entries` array (JSON), with an
  optional `--group` filter.
- `catalog export [path] --output <file|dir>` subcommand: re-serialises
  a validated schema to a new location (file or directory) using the
  canonical formatter; prints text or JSON summary.
- End-to-end integration tests under `test/integration/` covering
  build → validate → list → search → export against the fake registry,
  including error-path exit codes.
- `internal/catalog`: `Variable.References` field
  (`[]VariableReference{Module, Output}`) declaring explicit
  cross-module dependencies; validator resolves each reference to an
  existing module + output, rejects self-references and unknown
  targets, and reports issues alongside the rest of the schema in a
  single pass. Re-exported via `pkg/catalog`.
- `internal/catalog`: dependency graph (`BuildGraph`, `Graph`,
  `Edge`, `DependencyNode`) built from `Variable.References` with
  deterministic node and edge order, 3-colour DFS cycle detector
  returning every elementary cycle in canonical form, and
  depth-bounded `Resolve` traversal that returns a typed
  `*CycleError` (carrying the cycle path) or `ErrUnknownModule` for
  invalid roots.
- `dependencies <module>` subcommand: `--schema`, `--depth`,
  `--check-cycles`, `--format text|json`. Renders an ASCII tree (text)
  or a structured JSON node graph annotated with the parent edge.
  Errors map to `ExitModuleNotFound` (root not in catalog) or
  `ExitDependencyFailed` (cycle, with cycle path in suggestions).
- `internal/catalog`: `ExtractInterface(schema, opts)` builds a
  composer-facing view of a requested module subset, distinguishing
  user-facing inputs from auto-wired inputs satisfied by another
  selected module's output. Per-module + flattened (sorted) aggregate
  views; supports `RequiredOnly` and `Full` modes.
- `interface <modules...>` subcommand: `--schema`, `--required-only`,
  `--full` (mutually exclusive), `--format text|json|yaml`. Adds YAML
  output (gopkg.in/yaml.v3) reusing the JSON wire shape.
- `test/fixtures/schemas/valid_full.json` extended with an
  `aws_subnet` module wired to `aws_vpc.id` so reference handling has
  on-disk coverage.
- `internal/terraform`: HCL stack generator built on
  `hashicorp/hcl/v2/hclwrite`. `Plan(schema, opts)` resolves module
  selection in deps-first order (DFS postorder), wires variable
  references that resolve inside the selection, and surfaces
  placeholder/missing-source warnings; ambiguous references fail with
  `ErrAmbiguousReference`. `Generate(plan)` emits the five core files
  (`providers.tf`, `variables.tf`, `locals.tf`, `main.tf`,
  `outputs.tf`) using `this_<module>` naming and `<module>_<var>`
  external variable / `<module>_<output>` stack output namespacing.
  Module sources resolve via a pluggable `SourceResolver`
  (`PlaceholderResolver` is the default; uses `ModuleEntry.Source` when
  set, otherwise a `TODO: set module source` placeholder). Each
  `GeneratedFile` carries a `SHA256Hex()` for dry-run summaries.
- `compose` subcommand: `--schema`, `--modules` (space- or
  comma-separated), `--output-dir`, `--dry-run`, `--force`,
  `--format text|json`. Dry-run prints the planned files (path,
  truncated sha256, byte count) plus plan warnings without touching
  the filesystem; write mode materialises every file via tmp+rename
  for atomic per-file replacement, and refuses to clobber any of the
  five generated files in a non-empty `--output-dir` unless `--force`
  is set. Errors map to `ExitModuleNotFound` (unknown module),
  `ExitDependencyFailed` (cycle or ambiguous reference) and
  `ExitInvalidArgs` (missing modules / output-dir).
- `internal/catalog/registry`: real Terraform CLI source (`terraform_exec.go`)
  shells out to `terraform providers schema -json`, caches raw output on disk,
  supports `--include`/`--exclude` glob patterns. HTTP registry client (`http.go`)
  hits registry.terraform.io for version listing. Schema translator (`translate.go`)
  normalises provider JSON schema into `catalog.Schema`.
- `internal/commands/interactive`: guided multi-step workflow (survey/v2) —
  provider choice, version pinning, resource multi-select, catalog write,
  optional compose trigger. Gated behind `--output-dir` (required).
- `internal/terraform/rootstack`: root-stack HCL generator emitting
  `providers.tf`, `versions.tf`, `variables.tf`, `locals.tf`, `main.tf`,
  `outputs.tf` that compose the per-module folders.
- `internal/git`: remote detection (`remote.go`) and Git tag listing (`tags.go`)
  for source-URL resolution in generated stacks.
- `test/integration/registry_http_test.go`: E2E registry integration test
  (gated by `INFRA_COMPOSER_E2E=1`, targets `hashicorp/random`).
- `docs/INTERACTIVE.md`, `docs/REGISTRY.md`: feature documentation.

### Changed
- `compose` subcommand extended with `--root-stack` flag (default `true`) to
  control whether the top-level stack files are emitted.
- `internal/terraform/{generator,plan,naming,types}` refactored: `source.go`
  deleted; source resolution moved into the registry package.
- `pkg/catalog/types.go` extended with registry-related public types.
- `.gitignore`: generated `catalog/` and `infrastructure/` directories excluded.

### Distribution (Phase 4)
- Cross-platform binaries via `scripts/release.sh` and GoReleaser.
- `INSTALL.md`, `QUICKSTART.md`, `CLI.md`, `CONFIG.md`, `PIPELINE.md` added.
- Homebrew tap configuration added to `.goreleaser.yml`.
- npm wrapper package added under `npm/`.
- GitHub issue and PR templates added.

[1.0.0]: https://github.com/tiziano093/infra-composer-cli/releases/tag/v1.0.0
