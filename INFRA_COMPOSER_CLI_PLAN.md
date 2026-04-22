# infra-composer CLI — Implementation Plan

**Created:** 2026-04-22  
**Status:** Planning  
**Repository:** github.com/tiziano093/infra-composer-cli (new)  
**Language:** Go  
**Scope:** All 4 Phases (full MVP)  
**Architecture:** Standalone CLI with embedded catalog pipeline

---

## Executive Summary

Transform the `infra-composer` skill (currently: scattered bash scripts with hardcoded paths) into a **standalone, portable Go CLI** that:

- **Solves the portability problem:** Replace `/Users/tiziano/.claude/skills/...` hardcoded paths with embedded, environment-agnostic logic
- **Enables pipeline integration:** JSON output, exit codes, `--dry-run`, and configuration for CI/CD (GitHub Actions, Azure Pipelines)
- **Distributes easily:** Pre-built binaries (macOS/Linux/Windows), npm package, Homebrew tap, Docker image
- **Reduces friction:** No external dependencies on `tf_provider_pipeline` project — catalog building is embedded
- **Improves discoverability:** `--help`, man pages, structured error messages with suggestions

**Target Users:**
- Terraform module developers (build catalogs)
- Infrastructure engineers (compose stacks from catalogs)
- DevOps/SRE (integrate into pipelines)
- Teams on multiple OS/machines (no setup friction)

---

## What the CLI Solves

### Current Problems (Skill-based approach)

1. **Hardcoded Paths**
   - Scripts reference `/Users/tiziano/.claude/skills/infra-composer/scripts/...`
   - Not portable: breaks on other machines, OS, or CI/CD runners
   - Requires manual setup of symlinks or environment tweaks

2. **Fragmented Workflow**
   - 7-step manual process: parse → search → resolve → interface → generate → support → review
   - Each step involves running separate bash scripts
   - Error handling is ad-hoc (bash exit codes, no structured errors)
   - User must manually verify catalog exists (Phase 0 is a bottleneck)

3. **Poor Pipeline Integration**
   - Text-only output (hard to parse in scripts)
   - No `--dry-run` or `--format json` flags
   - Unclear when to stop/retry on error
   - Difficult to embed in GitHub Actions, Azure Pipelines, or other CI/CD

4. **Catalog Friction**
   - Requires external `tf_provider_pipeline` project
   - Phase 0 (catalog build) is unclear and multi-step
   - No auto-detection of which providers are ready

5. **Distribution Overhead**
   - Copying scripts locally per machine
   - No version management (vs. pinned releases)
   - Different bash versions cause subtle failures

### New CLI Solutions

| Problem | Solution |
|---------|----------|
| Hardcoded paths | Embedded logic, zero external dependencies |
| Fragmented workflow | Single `infra-composer` command with subcommands |
| Poor error handling | Structured errors (JSON), exit codes, suggestions |
| Pipeline friction | `--format json`, `--dry-run`, CI-ready flags |
| Catalog friction | `catalog build` command with built-in pipeline |
| Distribution | Binary download + Homebrew + npm + Docker |

---

## Scopo Della CLI

### Primary Goal
**Enable infrastructure-as-code developers to compose Terraform consumer stacks from a modular catalog, without leaving the command line, and without hitting portability or pipeline integration issues.**

### Key Capabilities

1. **Catalog Management** (`infra-composer catalog`)
   - Build from scratch (discover → crawl → normalize → generate → export)
   - Export existing normalized catalogs
   - Validate schema integrity
   - List available providers + status

2. **Module Discovery** (`infra-composer search`)
   - Find modules by keyword (fuzzy match)
   - Group search (all network resources at once)
   - Filter by type, provider, tags
   - Return structured results (JSON/YAML/table)

3. **Dependency Resolution** (`infra-composer dependencies`)
   - Visualize module dependency trees
   - Detect circular dependencies
   - Suggest missing prerequisites

4. **Interface Extraction** (`infra-composer interface`)
   - Read module variables + outputs
   - Filter to required-only (token-efficient for LLM use)
   - Support YAML/JSON output for scripting

5. **Stack Composition** (`infra-composer compose`)
   - High-level: "I want VPC + subnets + EC2" → full Terraform stack
   - Or explicit: "Use these 5 modules" → wired together
   - Generate 5 HCL files (providers, variables, locals, main, outputs)
   - Generate 5 support files (CI/CD, tflint, terraform-docs, tfvars)
   - Dry-run preview before writing

6. **Version Management** (`infra-composer version`)
   - Show CLI version + build info
   - Show catalog version (schema.json semver)

---

## Problems It Addresses

### Problem 1: Portability
**Current:** Scripts hardcoded to `/Users/tiziano/...`  
**CLI Solution:**
- Single binary (no script copying)
- Works on macOS, Linux, Windows+WSL out-of-the-box
- `~/.infra-composer/config.yaml` for user config (portable)
- `INFRA_COMPOSER_*` env vars override config

### Problem 2: Pipeline Integration
**Current:** No `--format json`, text output hard to parse  
**CLI Solution:**
- `--format json|yaml|text` flags
- `--dry-run` to preview without side effects
- Exit codes: 0=success, 1-10=specific errors
- Structured error output with suggestions

### Problem 3: Catalog Onboarding
**Current:** Phase 0 requires knowing about `tf_provider_pipeline` externally  
**CLI Solution:**
- `infra-composer catalog build --provider aws` handles everything
- Embedded pipeline logic (no external dependency)
- Fast path: `--only-export` if catalog already normalized
- `catalog list` shows which providers are ready

### Problem 4: Manual Step Sequencing
**Current:** Follow 7 manual steps, check outputs, decide next step  
**CLI Solution:**
- `infra-composer compose` orchestrates steps 1-6 automatically
- Or use fast-path single command: `compose-context.sh` → `infra-composer compose`
- Clear help text + structured output guide next steps

### Problem 5: Error Handling
**Current:** "Command not found" or "jq error" — unclear what to do  
**CLI Solution:**
- Exit code 5 = "Module not found"
- Suggestions: "Did you mean 'X'? Try `infra-composer search Y`"
- JSON error output for scripts: `{"error": {...}, "suggestions": [...]}`

---

## Objectives

### Primary Objectives
- [ ] **Eliminate portability barriers:** Zero hardcoded paths, single binary works everywhere
- [ ] **Enable pipeline integration:** JSON output, exit codes, dry-run support
- [ ] **Simplify catalog setup:** Embedded pipeline, auto-detection of readiness
- [ ] **Improve discoverability:** `--help`, man pages, structured errors

### Secondary Objectives
- [ ] **Maintain backward compatibility:** Skill still works, CLI is an alternative
- [ ] **Reduce token usage:** `--required-only --format yaml` for LLM efficiency
- [ ] **Provide multiple distribution methods:** Binary, npm, Homebrew, Docker
- [ ] **Ensure quality:** Unit + integration tests, cross-platform CI/CD

---

## Implementation Phases

### Phase 1: Core Foundation (Weeks 1-2)
**Deliverable:** Scaffold Go project, CLI framework, basic commands  
**Todos:**
- [ ] Create GitHub repo `infra-composer-cli`
- [ ] Init Go module: `go mod init github.com/tiziano093/infra-composer-cli`
- [ ] Scaffold directory structure (cmd/, internal/, pkg/, test/)
- [ ] Add Cobra framework for CLI
- [ ] Implement root command + `--help`, `--version`
- [ ] Config loading: env vars + `~/.infra-composer/config.yaml`
- [ ] Logging framework (structured, levels: debug/info/warn/error)
- [ ] Error handling + exit codes
- [ ] Unit tests for config, logging
- [ ] Create Makefile + build scripts
- [ ] GitHub Actions: test.yml (runs on push)

**Key Files:**
- `cmd/infra-composer/main.go`
- `internal/cli/root.go`
- `internal/config/config.go`
- `internal/output/log.go`
- `Makefile`, `go.mod`

---

### Phase 2: Catalog Operations (Weeks 3-4)
**Deliverable:** Catalog build, export, search, discovery  
**Todos:**
- [ ] Implement `catalog build` command
  - Discover providers from Terraform registry
  - Crawl resource/data source metadata
  - Normalize to internal catalog format
  - Generate module stubs (HCL templates)
  - Export to schema.json
- [ ] Implement `catalog export` command
  - Read normalized catalog
  - Output schema.json
- [ ] Implement `catalog list` command
  - Show providers + status (discovered/normalized/ready)
- [ ] Implement `catalog validate` command
  - Schema.json integrity checks
- [ ] Implement `catalog info` command
  - Provider name, version, module count, last updated
- [ ] Implement `search` command
  - Keyword search (fuzzy match, AND logic)
  - Group search (`--group network`)
  - Filter by type
  - Output: text table, JSON, YAML
- [ ] Catalog schema parsing + validation (`internal/catalog/schema.go`)
- [ ] Module search + filtering (`internal/catalog/searcher.go`)
- [ ] Integration tests (mock schemas, fixtures)
- [ ] Test fixtures: `test/fixtures/schemas/aws-schema.json`

**Key Files:**
- `internal/commands/catalog.go`
- `internal/commands/search.go`
- `internal/catalog/builder.go`
- `internal/catalog/exporter.go`
- `internal/catalog/searcher.go`
- `test/fixtures/schemas/aws-schema.json`

---

### Phase 3: Module Composition (Weeks 5-6)
**Deliverable:** Dependencies, interfaces, full stack composition  
**Todos:**
- [ ] Implement `dependencies` command
  - Build dependency graph from schema
  - Detect circular dependencies
  - Output: tree (text), JSON, `--check-cycles` flag
- [ ] Implement `interface` command
  - Read module variables + outputs from modules/
  - Output: YAML, JSON
  - `--required-only` flag (reduce tokens)
  - `--full` flag (all variables)
- [ ] Implement `compose` command
  - High-level: parse natural language request → modules
  - Explicit: `--modules "vpc subnet"` → wire them
  - Resolve dependencies automatically
  - Generate 5 Terraform files:
    - `providers.tf`
    - `variables.tf`
    - `locals.tf`
    - `main.tf`
    - `outputs.tf`
  - Generate 5 support files:
    - `azure-pipelines.yml` (or `github-actions.yml` based on provider)
    - `.pre-commit-config.yaml`
    - `.tflint.hcl`
    - `.terraform-docs.yml`
    - `environments/<ENV>/<COMPONENT>.tfvars`
  - `--dry-run` flag: preview without writing
  - `--format json` output: list of files, modules used, assumptions
- [ ] Terraform generator (`internal/terraform/generator.go`)
  - HCL templates for each file
  - Variable validation blocks
  - Naming conventions (snake_case, "this" for singletons)
- [ ] Support file templates (`internal/terraform/support.go`)
- [ ] Dependency resolution (`internal/catalog/dependency.go`)
- [ ] Git operations: auto-detect source URL, fetch semver tags (`internal/git/`)
- [ ] End-to-end integration tests: request → full stack
- [ ] Test fixtures: example requests, expected outputs

**Key Files:**
- `internal/commands/dependencies.go`
- `internal/commands/interface.go`
- `internal/commands/compose.go`
- `internal/terraform/generator.go`
- `internal/terraform/templates.go`
- `internal/terraform/support.go`
- `internal/catalog/dependency.go`
- `internal/git/remote.go`, `internal/git/tags.go`
- `test/integration/compose_test.go`

---

### Phase 4: Distribution + Polish (Weeks 7-8)
**Deliverable:** Releases, documentation, distribution channels  
**Todos:**
- [ ] Cross-platform build script (`scripts/build.sh`)
  - macOS (Intel + ARM64)
  - Linux (amd64, arm64)
  - Windows (amd64)
- [ ] Release automation (`scripts/release.sh`)
  - Generate checksums + signatures
  - Create GitHub release
  - Upload binaries
- [ ] GitHub Actions: `release.yml` workflow
  - Trigger on git tag push
  - Build cross-platform binaries
  - Upload artifacts
  - Create release
- [ ] Dockerfile (multi-stage build)
  - Build stage: Go builder
  - Runtime stage: Alpine minimal
  - Push to ghcr.io on release
- [ ] GitHub Actions: `docker.yml` workflow
  - Build + push Docker image
- [ ] Homebrew tap setup
  - Formula: `infra-composer.rb`
  - Auto-update script on release
- [ ] npm package setup
  - `package.json` wrapper
  - Download appropriate binary
  - Add to `$PATH`
- [ ] Documentation
  - `docs/README.md` — overview
  - `docs/INSTALL.md` — all installation methods
  - `docs/QUICKSTART.md` — 5-min tutorial
  - `docs/CLI.md` — command reference (auto-generated from `--help`)
  - `docs/CONFIG.md` — configuration guide
  - `docs/PIPELINE.md` — GitHub Actions + Azure Pipelines examples
  - `docs/CONTRIBUTING.md` — dev guide
  - Man pages (auto-generated)
- [ ] Examples
  - `examples/aws-basic.yaml` — VPC + subnet
  - `examples/azure-advanced.yaml` — multi-region AKS
  - `examples/outputs/` — sample generated stacks
- [ ] Changelog: `CHANGELOG.md`
- [ ] License: `LICENSE` (Apache 2.0 or MIT)
- [ ] PR template + issue template
- [ ] Smoke tests: e2e test in CI/CD
- [ ] Performance benchmarks (startup time, compose speed)

**Key Files:**
- `scripts/build.sh`, `scripts/release.sh`, `scripts/docker.sh`
- `.github/workflows/release.yml`, `docker.yml`
- `Dockerfile`
- `homebrew/infra-composer.rb`
- `package.json` (npm wrapper)
- `docs/*.md`
- `examples/*.yaml`

---

## File Structure (Complete)

```
infra-composer-cli/
├── cmd/
│   └── infra-composer/
│       ├── main.go                    # Entry point
│       └── _docs/
│           └── gen.go                 # CLI docs generator
├── internal/
│   ├── cli/
│   │   ├── root.go                    # Root command (Cobra)
│   │   └── middleware.go              # Logging, error handling
│   ├── commands/
│   │   ├── catalog.go                 # catalog build|export|list|validate|info
│   │   ├── search.go                  # search <keyword>
│   │   ├── dependencies.go            # dependencies <module>
│   │   ├── interface.go               # interface <modules>
│   │   ├── compose.go                 # compose --modules ...
│   │   └── version.go                 # version
│   ├── catalog/
│   │   ├── schema.go                  # Schema parsing + types
│   │   ├── builder.go                 # Catalog build pipeline
│   │   ├── exporter.go                # Export to schema.json
│   │   ├── searcher.go                # Module search + filter
│   │   ├── dependency.go              # Dependency resolution
│   │   └── normalizer.go              # Terraform registry normalization
│   ├── terraform/
│   │   ├── generator.go               # Main TF stack generator
│   │   ├── templates.go               # HCL template definitions
│   │   ├── support.go                 # Support files (CI/CD, tflint, etc.)
│   │   ├── validator.go               # HCL validation
│   │   └── naming.go                  # Naming conventions
│   ├── config/
│   │   ├── config.go                  # Config file parsing
│   │   ├── env.go                     # Env var loading (INFRA_COMPOSER_*)
│   │   └── defaults.go                # Sensible defaults
│   ├── git/
│   │   ├── remote.go                  # Extract remote URL
│   │   └── tags.go                    # Fetch semver tags
│   └── output/
│       ├── log.go                     # Structured logging
│       ├── json.go                    # JSON marshaling
│       ├── yaml.go                    # YAML formatting
│       └── table.go                   # ASCII table output
├── pkg/
│   ├── terraform/
│   │   └── terraform.go               # Exported types (for library use)
│   └── catalog/
│       └── catalog.go                 # Exported types
├── test/
│   ├── unit/
│   │   ├── config_test.go
│   │   ├── catalog_test.go
│   │   └── ...
│   ├── integration/
│   │   ├── compose_test.go
│   │   ├── catalog_test.go
│   │   └── ...
│   └── fixtures/
│       ├── schemas/
│       │   ├── aws-schema.json        # Mock AWS schema (5-10 modules)
│       │   └── azure-schema.json
│       ├── modules/
│       │   ├── aws_vpc/
│       │   │   ├── variables.tf
│       │   │   └── outputs.tf
│       │   └── ...
│       ├── requests/
│       │   ├── basic-vpc.yaml
│       │   └── advanced-multi-region.yaml
│       └── expected-outputs/
│           └── basic-vpc-stack/
│               ├── providers.tf
│               └── ...
├── scripts/
│   ├── build.sh                       # Cross-platform compilation
│   ├── test.sh                        # Run test suite
│   ├── release.sh                     # Build + release binaries
│   ├── docker.sh                      # Docker image build
│   └── install.sh                     # Install from release
├── build/                             # Generated binaries (gitignored)
│   ├── infra-composer-darwin-amd64
│   ├── infra-composer-darwin-arm64
│   ├── infra-composer-linux-amd64
│   ├── infra-composer-windows-amd64.exe
│   └── checksums.txt
├── docs/
│   ├── README.md                      # Main documentation
│   ├── INSTALL.md                     # Installation guide
│   ├── QUICKSTART.md                  # 5-minute tutorial
│   ├── CLI.md                         # Command reference (auto-gen)
│   ├── CONFIG.md                      # Configuration guide
│   ├── PIPELINE.md                    # CI/CD integration examples
│   └── CONTRIBUTING.md                # Developer guide
├── examples/
│   ├── aws-basic.yaml                 # Example: VPC + subnet
│   ├── azure-advanced.yaml            # Example: AKS multi-region
│   └── outputs/
│       ├── basic-vpc/
│       │   ├── providers.tf
│       │   ├── main.tf
│       │   └── ...
│       └── ...
├── homebrew/
│   └── infra-composer.rb              # Homebrew formula
├── .github/
│   ├── workflows/
│   │   ├── test.yml                   # Unit + integration tests
│   │   ├── release.yml                # Cross-platform binary release
│   │   └── docker.yml                 # Docker image push
│   ├── issue_template.md
│   └── pull_request_template.md
├── Dockerfile                         # Multi-stage Docker build
├── Makefile                           # Build targets
├── go.mod                             # Go module file
├── go.sum
├── package.json                       # npm wrapper
├── .gitignore
├── .golangci.yml                      # Linter config
├── CHANGELOG.md                       # Release notes
├── LICENSE                            # Apache 2.0 or MIT
└── README.md                          # GitHub repo README

```

---

## Todos (Organized by Phase)

### Phase 1: Foundation
- [ ] **project-setup**: Create GitHub repo infra-composer-cli, init Go module, scaffold structure
- [ ] **cobra-framework**: Add Cobra CLI framework, root command
- [ ] **config-system**: Implement env var + YAML config loading
- [ ] **logging**: Add structured logging (levels: debug/info/warn/error)
- [ ] **error-handling**: Define exit codes, error types, structured error output
- [ ] **tests-unit**: Unit tests for config, logging, error handling
- [ ] **makefile-build**: Create Makefile with build, test, lint targets
- [ ] **github-actions-test**: Setup test.yml workflow (runs on push)
- [ ] **docs-readme**: Initial README with project overview

### Phase 2: Catalog Ops
- [ ] **catalog-schema**: Implement schema parsing + types (internal/catalog/schema.go)
- [ ] **catalog-builder**: Build pipeline (discover → crawl → normalize → generate → export)
- [ ] **catalog-exporter**: Export normalized catalog to schema.json
- [ ] **catalog-commands**: Implement all `catalog` subcommands
- [ ] **search-command**: Full module search + filtering
- [ ] **tests-catalog**: Integration tests for catalog ops
- [ ] **fixtures-schemas**: Create mock schemas for testing

### Phase 3: Composition
- [ ] **dependency-resolution**: Build dependency graph, detect cycles
- [ ] **dependencies-command**: Implement `dependencies` command
- [ ] **interface-command**: Implement `interface` command
- [ ] **terraform-generator**: HCL file generation (providers, variables, main, outputs)
- [ ] **support-files**: CI/CD, tflint, terraform-docs, tfvars generation
- [ ] **compose-command**: Orchestrate full stack composition
- [ ] **git-integration**: Auto-detect catalog URL, fetch tags
- [ ] **tests-integration**: E2E tests for compose workflow
- [ ] **fixtures-requests**: Example requests + expected outputs

### Phase 4: Distribution
- [ ] **cross-platform-build**: Build script for macOS/Linux/Windows
- [ ] **release-automation**: Release script + GitHub Actions workflow
- [ ] **docker-build**: Dockerfile + Docker CI/CD workflow
- [ ] **homebrew-tap**: Homebrew formula + auto-update
- [ ] **npm-package**: npm wrapper package
- [ ] **documentation**: Full docs (INSTALL, QUICKSTART, CLI ref, CONFIG, PIPELINE)
- [ ] **examples**: AWS + Azure example requests + outputs
- [ ] **changelog**: CHANGELOG.md setup
- [ ] **smoke-tests**: E2E pipeline tests
- [ ] **performance**: Benchmark startup + compose speed

---

## Dependencies (Blocked On)

- Phase 2 blocks Phase 3 (need catalog to compose)
- Phase 3 blocks Phase 4 (need working CLI to distribute)
- Phase 1 is standalone (no blockers)

---

## Success Criteria

✅ **Phase 1 Complete When:**
- CLI builds and runs: `./bin/infra-composer --version`
- Config loads from `~/.infra-composer/config.yaml` and env vars
- `--help` displays structured help
- Unit tests pass: `make test`
- GitHub Actions test workflow runs on push

✅ **Phase 2 Complete When:**
- `catalog build --provider aws` generates schema.json
- `search network` finds modules
- `catalog list` shows providers
- Integration tests pass (mocked Terraform registry)
- Schema validation catches errors

✅ **Phase 3 Complete When:**
- `compose --modules "vpc subnet" --output-dir ./stack` generates 5 TF files + 5 support files
- `--dry-run --format json` previews without writing
- `dependencies vpc` shows tree with no cycles
- E2E tests pass
- All TODO markers added for user customization

✅ **Phase 4 Complete When:**
- Binary downloads work: curl → binary → $PATH
- Homebrew install: `brew install infra-composer`
- npm install: `npm install -g @tiziano093/infra-composer`
- Docker: `docker run ghcr.io/tiziano093/infra-composer ...`
- Documentation complete + examples working
- 1.0.0 release on GitHub

---

## Known Unknowns

1. **Terraform Registry API Rate Limits:** Phase 2 discovery might hit rate limits; need caching strategy
2. **Module Complexity:** Some modules have complex variable types (objects, for_each maps) — need solid HCL generation
3. **Natural Language Parsing:** High-level requests ("I want a VPC") — scope of implementation?
   - MVP: Explicit modules only (`--modules "vpc subnet"`)
   - Future: NLP or pass to Claude for requests
4. **Cross-Platform Testing:** How thoroughly to test on Windows+WSL?
   - MVP: Automated tests on Linux; manual testing on Windows
   - Future: Add Windows CI/CD agents

---

## Timeline

**Estimate:** 8 weeks (2 weeks per phase, sequential)

**Critical Path:**
- Week 1-2: Phase 1 (foundation must work)
- Week 3-4: Phase 2 (catalog ops foundation)
- Week 5-6: Phase 3 (core value — compose)
- Week 7-8: Phase 4 (release + distribution)

**Parallel Work:** Documentation can start in Week 3 (Phase 2), not blocking later phases.

---

## Open Questions (For User)

1. **Natural Language Requests:** Should `compose --request "vpc with 2 subnets"` work, or explicit modules only?
   - Current plan: MVP with explicit modules, future with LLM integration
2. **Terraform Version Support:** Target 1.0+, or older?
   - Current plan: 1.0+, with notes on 1.6+ for native tests
3. **Catalog Source:** Single source (e.g., always hashicorp/aws registry), or multiple providers?
   - Current plan: Support any provider, CLI detects from schema
4. **Versioning:** Semver or other?
   - Current plan: Semver (MAJOR.MINOR.PATCH)

