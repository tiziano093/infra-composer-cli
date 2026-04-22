# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
