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
