# Copilot Instructions for Infra-Composer CLI

**Project Status:** Phase 1 — repository scaffolded, CLI implementation in progress  
**Language:** Go 1.21+  
**Framework:** Cobra CLI framework  
**Target:** Portable CLI tool for composing Terraform stacks

---

## High-Level Architecture

Infra-Composer is a **modular, layered CLI** with clean separation of concerns:

```
CLI Layer (Cobra)
    ↓
Command Layer (catalog, search, compose, dependencies, interface)
    ↓
Domain Layer (catalog schema/builder/searcher, terraform generation, config)
    ↓
Infrastructure Layer (git, output formatting, logging)
```

**Key principle:** Commands orchestrate; domain packages implement business logic. Each package has a single responsibility.

### 5 Core Domain Packages

- **`internal/catalog/`** — Schema parsing, module search, dependency resolution (discover → crawl → normalize → export)
- **`internal/terraform/`** — HCL generation from modules, support files (CI/CD, tflint, terraform-docs)
- **`internal/config/`** — Configuration hierarchy (defaults → file → env vars → flags)
- **`internal/git/`** — Git remote URL extraction, semver tag fetching
- **`internal/output/`** — JSON, YAML, table formatting + structured logging

### Public API

- **`pkg/catalog/`** — Exported catalog types (for library use)
- **`pkg/terraform/`** — Exported terraform types (for library use)

---

## Build, Test, and Lint Commands

When implemented, these commands will exist (based on planned Makefile):

```bash
# Build
make build                          # Build binary: bin/infra-composer

# Test
make test                           # Run all tests with coverage
go test ./... -v                    # Run all tests verbose
go test -run TestSearchModules ./internal/catalog/...  # Single test
go test ./test/integration/...      # Integration tests only

# Lint
make lint                           # Run golangci-lint
go fmt ./...                        # Format code
goimports -w .                      # Organize imports

# Documentation
make docs                           # Auto-generate CLI reference

# Clean
make clean                          # Remove build artifacts
```

**Note:** Phase 1 scaffolding is complete (Go module, Makefile, workflows, stub `main.go` with `--version`). Cobra root command, config, and logging packages are pending — internal packages currently contain only `.gitkeep` placeholders.

---

## Key Conventions

### Package Organization

- **`cmd/infra-composer/`** — Minimal entry point; no business logic
- **`internal/`** — Private packages; not exported to external users
- **`pkg/`** — Public API; exported types only
- **`test/`** — All tests; `*_test.go` for unit, `*_integration_test.go` for integration

### Error Handling

1. Define custom error types per domain package (e.g., `ValidationError` in `internal/catalog/`)
2. Wrap errors with context: `fmt.Errorf("operation: %w", err)`
3. Commands convert domain errors to CLI errors with suggestions
4. Exit codes: 0=success, 1=generic, 2=invalid args, 3=file not found, 4=validation, 5=module not found, etc.

### Testing Patterns

- **Table-driven tests** for multiple scenarios
- **Mock external dependencies** (HTTP, filesystem) for unit tests
- **Integration tests use fixtures** in `test/fixtures/` (schemas, requests, expected outputs)
- **Coverage target:** >80% for business logic; acceptable gaps in rare error paths

### Naming Conventions

- **Packages:** lowercase, single word (`catalog`, `terraform`), plural for collections (`commands`)
- **Types/Functions:** PascalCase for exported, camelCase for unexported
- **Variables:** `i`, `err` acceptable in small scopes; descriptive names for larger scopes (`catalogSchema`, `isValid`)
- **Constants:** UPPER_SNAKE_CASE, grouped logically, well-commented
- **Files:** no `_utils.go` (split by domain)

### Code Style

- Run `gofmt -w .` and `goimports -w .` before commit
- 80-char soft limit, 120-char hard limit
- Comments explain **why**, not **what**
- All exported types, functions, constants must have doc comments
- Error wrapping always uses `%w` verb to preserve chain

### Dependency Injection

Inject dependencies into command/service constructors (not singletons):

```go
type CatalogService struct {
    logger Logger
    client *http.Client
    cache  Cache
}

func NewCatalogService(logger Logger, client *http.Client, cache Cache) *CatalogService {
    return &CatalogService{logger: logger, client: client, cache: cache}
}
```

Enables testing (mock dependencies) and flexibility (swap implementations).

### Configuration Loading Hierarchy

Config is loaded in this order (highest priority last):

1. **Hardcoded defaults** (`internal/config/defaults.go`)
2. **Config file** (`~/.infra-composer/config.yaml`)
3. **Environment variables** (`INFRA_COMPOSER_*`)
4. **CLI flags** (highest priority)

---

## Phased Implementation

The project is structured for 4-phase delivery over 8 weeks:

### Phase 1: Foundation (Weeks 1-2)
- Cobra CLI scaffold, config system, logging, error handling
- Deliverable: `./bin/infra-composer --version` works

### Phase 2: Catalog Ops (Weeks 3-4)
- Schema parsing, catalog builder (discover → crawl → normalize), search
- Commands: `catalog build`, `search`, `catalog list`, `catalog validate`

### Phase 3: Module Composition (Weeks 5-6) — CORE VALUE
- Dependency resolution, compose command, HCL generation
- Commands: `dependencies`, `interface`, `compose`
- E2E integration tests

### Phase 4: Distribution (Weeks 7-8)
- Cross-platform builds, releases, Docker, npm, Homebrew
- v1.0.0 release

**Key constraint:** Each phase blocks the next (Phase 1 → Phase 2 → Phase 3 → Phase 4).

---

## Documentation References

- **Architecture details:** `docs/ARCHITECTURE.md`
- **Coding standards:** `docs/DEVELOPMENT_GUIDELINES.md`
- **Command structure:** `INFRA_COMPOSER_CLI_DESIGN.md` (Section 2)
- **Implementation roadmap:** `docs/ROADMAP.md`
- **Repository setup:** `docs/REPOSITORY_SETUP.md`
- **Contributing:** `docs/CONTRIBUTING.md`

---

## Common Patterns to Follow

### Command Structure (from ARCHITECTURE.md)

```go
func (c *CatalogCommand) Execute(ctx context.Context, args []string) error {
    // 1. Validate inputs
    if err := c.validateFlags(); err != nil {
        return fmt.Errorf("invalid flags: %w", err)
    }
    
    // 2. Call domain
    schema, err := c.catalogService.Build(ctx, c.provider)
    if err != nil {
        return fmt.Errorf("catalog build failed: %w", err)
    }
    
    // 3. Format output
    return c.output.Print(ctx, schema)
}
```

### Searcher Implementation Pattern

Module search uses fuzzy matching with AND logic:
- All keywords must match a module (AND)
- Fuzzy matching allows partial/misspelled names
- Filter by group, type, or limit results

### Dependency Graph Pattern

Build directed graphs from variable references:
- DFS-based cycle detection
- Return dependency tree with depth
- Suggest breaking points for cycles

---

## Pre-Implementation Notes

Phase 1 scaffolding is **complete**: directory tree, `go.mod`, `Makefile`, `.gitignore`, `.golangci.yml`, `.goreleaser.yml`, `Dockerfile`, GitHub Actions (test/lint/release/docker), Apache 2.0 LICENSE, README, CHANGELOG, PR + issue templates, `scripts/release.sh`, and a stub `cmd/infra-composer/main.go` that builds and responds to `--version`/`--help`.

**Still pending in Phase 1:** Cobra root command, Viper-based config, structured logging, error/exit-code framework, unit tests.

Files that exist:
- `go.mod` — Module definition with Cobra, Viper, testify dependencies
- `Makefile` — Build targets (build, test, lint, docs, release, clean)
- `cmd/infra-composer/main.go` — Entry point with version injection
- `.github/workflows/test.yml`, `lint.yml`, `release.yml`, `docker.yml` — CI/CD automation
- `.golangci.yml` — Linting configuration

---

## When to Ask for Help

This repository follows the patterns documented in `docs/CONTRIBUTING.md` and `docs/DEVELOPMENT_GUIDELINES.md`. Refer to those files for:

- Exact testing examples (unit, integration, table-driven)
- PR process and commit message format
- Code review expectations
- Anti-patterns to avoid
