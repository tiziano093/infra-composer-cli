# Infra-Composer CLI — Implementation Roadmap

**Version:** 1.0  
**Status:** Phase 1 — Complete; Phase 2 — Complete; Phase 3 — Complete (real registry source via `terraform` CLI + `interactive` workflow shipped); Phase 4 (Distribution) ready to start  
**Duration:** 8 Weeks (2 weeks per phase)  
**Last Updated:** 2026-04-22

---

## Roadmap Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                  PHASE 1: Foundation (Weeks 1-2)                │
│  Go Scaffold • Cobra CLI • Config System • Logging & Error      │
└────────────┬────────────────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────────────────────────────┐
│            PHASE 2: Catalog Ops (Weeks 3-4)                     │
│  Build • Export • Search • Schema Validation                    │
└────────────┬────────────────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────────────────────────────┐
│         PHASE 3: Module Composition (Weeks 5-6)                 │
│  Dependencies • Interfaces • Compose • TF Generation            │
└────────────┬────────────────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────────────────────────────┐
│      PHASE 4: Distribution & Polish (Weeks 7-8)                 │
│  Binaries • Releases • Docker • npm • Homebrew • Docs           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Foundation

**Duration:** Weeks 1-2  
**Team:** Lead Developer + 1-2 Developers  
**Goal:** Working CLI scaffold with config & logging

### Tasks

| Task | Owner | Effort | Status |
|------|-------|--------|--------|
| Create GitHub repo + init Go module | DevOps | 1d | ✅ Done (Go module initialized; repo already local) |
| Scaffold directory structure | Lead Dev | 1d | ✅ Done |
| Implement Cobra root command | Dev1 | 2d | ✅ Done (`internal/cli/root.go` + persistent flags) |
| Config system (env vars + YAML) | Dev1 | 2d | ✅ Done (`internal/config`, Viper hierarchy) |
| Structured logging framework | Dev2 | 1d | ✅ Done (`internal/output/log.go`, slog text/json) |
| Error handling + exit codes | Dev2 | 1.5d | ✅ Done (`internal/cli/errors.go`, codes 1–10) |
| Unit tests (config, logging, errors) | Dev1 | 2d | ✅ Done (errors, logger, config hierarchy, version cmd) |
| Makefile + build scripts | DevOps | 1d | ✅ Done (Makefile + `scripts/release.sh`) |
| GitHub Actions test workflow | DevOps | 1.5d | ✅ Done (test, lint, release, docker workflows) |
| Initial README + setup docs | Tech Writer | 1d | 🟡 Partial (README stub; full user docs in Phase 4) |

### Deliverables

✅ **Binary builds and runs:**
```bash
./bin/infra-composer --version
# Output: infra-composer v1.0.0-dev, Built: 2026-04-24, Commit: abc123
```

✅ **Config loads from multiple sources:**
```bash
# Default values
infra-composer --help

# From config file (~/.infra-composer/config.yaml)
infra-composer --verbose

# From env vars
INFRA_COMPOSER_LOG_LEVEL=debug infra-composer --help

# From flags (overrides all)
infra-composer --log-level error
```

✅ **Tests pass:**
```bash
make test
# Output: ok      github.com/tiziano093/infra-composer-cli/internal/config    2.345s
#         ok      github.com/tiziano093/infra-composer-cli/internal/output    1.234s
```

✅ **GitHub Actions runs on push**
- Tests execute
- Linting passes
- Coverage reported

### Success Criteria

- ✅ CLI builds without errors
- ✅ `--help` displays coherent structure
- ✅ Config loading works (all 3 sources)
- ✅ Unit tests pass with >80% coverage
- ✅ GitHub Actions workflow executes
- ✅ Team can build + run locally

### Known Unknowns

- Exact startup time (measure after Phase 1)
- Config file format (YAML vs JSON vs TOML)

---

## Phase 2: Catalog Operations

**Duration:** Weeks 3-4  
**Team:** Lead Developer + 2 Developers  
**Depends On:** Phase 1 complete  
**Goal:** Catalog build, export, search, validation

### Tasks

| Task | Owner | Effort | Status |
|------|-------|--------|--------|
| Schema types + parsing | Dev1 | 2d | ✅ Done (`internal/catalog` schema/parse/validate, `pkg/catalog` re-exports, fixtures) |
| Catalog builder pipeline | Dev1 | 3d | ✅ Done (`internal/catalog/builder.go`, discover→list→fetch→normalize→validate, deterministic ordering) |
| Terraform Registry API integration (mock) | Dev2 | 2d | ✅ Done (`internal/catalog/registry`, `Client` interface + `FakeClient` backed by JSON fixtures) |
| Catalog exporter (to schema.json) | Dev2 | 1.5d | ✅ Done (`internal/catalog/exporter.go`, atomic tmp+rename, Path/Dir options) |
| Module search + filtering | Dev1 | 2d | ✅ Done (`internal/catalog/search.go`, AND logic, group/type filters, weighted scoring, fuzzy subsequence) |
| `catalog build` command | Dev1 | 1d | ✅ Done (`internal/commands/catalog.go`, `--provider/--output-dir/--registry-dir`, text+JSON) |
| `catalog export` command | Dev2 | 0.5d | ✅ Done (`internal/commands/catalog.go`, `[path] --output <file\|dir>`, text+JSON) |
| `catalog list` command | Dev2 | 1d | ✅ Done (`internal/commands/catalog.go`, table+JSON, `--group` filter) |
| `catalog validate` command | Dev1 | 1d | ✅ Done (`internal/commands/catalog.go`, text + JSON output, exits 4 on issues) |
| `search` command | Dev1 | 1d | ✅ Done (`internal/commands/search.go`, table + JSON, group/type/limit filters) |
| Integration tests + fixtures | QA | 3d | ✅ Done (`test/integration/cli_e2e_test.go`, build→validate→list→search→export, error-path exit codes) |

### Deliverables

✅ **Catalog build works:**
```bash
infra-composer catalog build --provider hashicorp/aws --output-dir ./catalog
# Output: Building catalog for hashicorp/aws (247 modules)
#         Discovering provider metadata... ✓
#         Crawling resources and data sources... ✓
#         Normalizing to schema format... ✓
#         Generating module stubs... ✓
#         Exporting to schema.json... ✓
#         Done in 28.3s
```

✅ **Search works:**
```bash
infra-composer search --schema ./catalog/schema.json network vpc
# Output: Name              Type      Provider      Description
#         ───────────────────────────────────────────────────────
#         aws_vpc           resource  hashicorp/aws Virtual Private Cloud
#         aws_vpc_peering   resource  hashicorp/aws VPC Peering Connection
```

✅ **List providers:**
```bash
infra-composer catalog list --schema ./catalog/schema.json
# Output: Provider         Modules  Status   Updated
#         ─────────────────────────────────────────────
#         hashicorp/aws    247      ready    2026-04-24T10:30:00Z
```

✅ **Integration tests pass with mocked API**

### Success Criteria

- ✅ `catalog build` executes in <30s for AWS
- ✅ `search` returns accurate results with AND logic
- ✅ `catalog validate` catches schema errors
- ✅ All fixtures in place for later phases
- ✅ Integration tests cover happy paths + error cases

### Known Unknowns

- Terraform Registry API rate limits
- Performance with large catalogs (>500 modules)
- Provider version handling

---

## Phase 3: Module Composition

**Duration:** Weeks 5-6  
**Team:** Lead Developer + 2-3 Developers  
**Depends On:** Phase 2 complete  
**Goal:** Core value — compose stacks from modules

### Tasks

| Task | Owner | Effort | Status |
|------|-------|--------|--------|
| Dependency graph builder | Dev1 | 2d | ✅ Done |
| Cycle detection | Dev1 | 1.5d | ✅ Done |
| `dependencies` command | Dev1 | 1d | ✅ Done |
| Interface extraction (variables + outputs) | Dev2 | 1.5d | ✅ Done |
| `interface` command | Dev2 | 0.5d | ✅ Done |
| HCL template engine | Dev2 | 2d | ✅ Done |
| Terraform file generation (5 core files) | Dev3 | 2d | ✅ Done |
| Support file generation (CI/CD, tflint, etc.) | Dev3 | 2d | ✅ Done |
| `compose` command | Dev1 | 1.5d | ✅ Done |
| Real registry source (`terraform providers schema -json`) | Dev1 | 2d | ✅ Done (`internal/catalog/registry/terraform_exec.go`, schema cache, `--include/--exclude`) |
| `interactive` command (provider/version/resource picker) | Dev2 | 1.5d | ✅ Done (`internal/commands/interactive.go`, survey/v2) |
| Git integration (remote detection, tags) | Dev2 | 1d | ✅ Done (`internal/git/{remote,tags}.go`) |
| E2E integration tests | QA | 3d | ✅ Done (gated by `INFRA_COMPOSER_E2E=1`, target hashicorp/random) |
| Error handling + suggestions | Dev1 | 1d | ✅ Done |

### Deliverables

✅ **Compose generates full stack:**
```bash
infra-composer compose \
  --schema ./catalog/schema.json \
  --modules-dir ./catalog/modules \
  --modules "aws_vpc aws_subnet aws_instance" \
  --output-dir ./infrastructure \
  --dry-run --format json

# Output JSON shows all files that would be generated
```

✅ **Generated files are valid Terraform:**
```
infrastructure/
├── providers.tf
├── variables.tf
├── locals.tf
├── main.tf
├── outputs.tf
├── .tflint.hcl
├── .pre-commit-config.yaml
├── .terraform-docs.yml
├── github-actions.yml (or azure-pipelines.yml)
└── environments/dev/app.tfvars
```

✅ **Dependencies command shows trees:**
```bash
infra-composer dependencies \
  --schema ./catalog/schema.json \
  aws_instance \
  --format json

# Output: Dependency tree in JSON/tree format
```

✅ **Interface command optimized for LLMs:**
```bash
infra-composer interface \
  --modules-dir ./catalog/modules \
  aws_vpc \
  --required-only \
  --format yaml

# Output: Only required variables, minimal verbosity
```

### Success Criteria

- ✅ `compose --modules "X Y Z"` generates working Terraform
- ✅ Generated code includes TODO markers for customization
- ✅ Dry-run previews without side effects
- ✅ Dependency cycles detected and reported
- ✅ E2E tests pass for typical stacks
- ✅ Performance: compose finishes in <5s

### Known Unknowns

- Complex variable types (objects, maps)
- Module source URL auto-detection
- Support file customization per provider

---

## Phase 4: Distribution & Polish

**Duration:** Weeks 7-8  
**Team:** DevOps + Lead Developer + Tech Writer  
**Depends On:** Phase 3 complete  
**Goal:** Production-ready release, multiple distributions

### Tasks

| Task | Owner | Effort | Status |
|------|-------|--------|--------|
| Cross-platform build script | DevOps | 1d | Pending |
| Release automation script | DevOps | 1.5d | Pending |
| GitHub Actions release workflow | DevOps | 1.5d | Pending |
| Dockerfile + docker workflow | DevOps | 1.5d | Pending |
| Homebrew formula + tap | DevOps | 1d | Pending |
| npm package wrapper | DevOps | 1d | Pending |
| User documentation (7 guides) | Tech Writer | 4d | Pending |
| Example requests + outputs | Tech Writer | 1.5d | Pending |
| CHANGELOG setup | DevOps | 0.5d | Pending |
| Smoke tests (E2E) | QA | 1.5d | Pending |
| Performance benchmarks | QA | 1d | Pending |
| Issue + PR templates | DevOps | 0.5d | Pending |

### Deliverables

✅ **Binaries for all platforms:**
```
build/
├── infra-composer-darwin-amd64 (macOS Intel)
├── infra-composer-darwin-arm64 (macOS Apple Silicon)
├── infra-composer-linux-amd64
├── infra-composer-windows-amd64.exe
└── checksums.txt (with signatures)
```

✅ **Installation works via multiple methods:**
```bash
# GitHub releases
curl -sSL https://github.com/tiziano093/infra-composer-cli/releases/download/v1.0.0/infra-composer-darwin-amd64 \
  -o /usr/local/bin/infra-composer && chmod +x /usr/local/bin/infra-composer

# Homebrew
brew tap tiziano093/infra-composer
brew install infra-composer

# npm
npm install -g @tiziano093/infra-composer

# Docker
docker run ghcr.io/tiziano093/infra-composer:v1.0.0 --version
```

✅ **Full documentation set:**
- README.md — Overview
- INSTALL.md — Installation methods
- QUICKSTART.md — 5-minute tutorial
- CLI.md — Command reference
- CONFIG.md — Configuration guide
- PIPELINE.md — CI/CD examples
- CONTRIBUTING.md — Developer guide

✅ **GitHub release page:**
- Release notes
- Pre-built binaries
- Checksums + signatures
- Upgrade guide

✅ **Benchmarks:**
- CLI startup: <100ms
- Catalog build: <30s
- Compose: <5s

### Success Criteria

- ✅ All installation methods work
- ✅ `v1.0.0` released on GitHub
- ✅ Docker image pushed to ghcr.io
- ✅ npm package published
- ✅ Homebrew formula working
- ✅ Documentation complete + examples verified
- ✅ Zero critical bugs found in smoke tests

---

## Cross-Phase Activities

### Documentation (Ongoing from Week 3)
- Week 3-4: Command reference, architecture diagrams
- Week 5-6: Configuration guide, pipeline integration examples
- Week 7-8: User guides, troubleshooting, examples

### Testing (Throughout all phases)
- Phase 1: Unit tests (config, logging)
- Phase 2: Integration tests (catalog ops) + fixtures
- Phase 3: E2E tests (compose workflow)
- Phase 4: Smoke tests + performance benchmarks

### Code Review
- Every PR: 2 reviewers minimum
- Before release: Architecture review
- After release: Performance analysis

---

## Team & Roles

| Role | Responsibilities | From |
|------|---|---|
| **Project Manager** | Timeline, blockers, communication | Week 1 |
| **Architecture Lead** | Design decisions, code review | Week 1 |
| **Lead Developer** | Phase ownership, mentoring | Week 1 |
| **Developers** | Feature implementation, unit tests | Week 1 |
| **DevOps/Infrastructure** | CI/CD, builds, releases, Docker | Week 1 + Week 7 |
| **QA/Testing** | Test strategy, fixtures, coverage | Week 2 + ongoing |
| **Tech Writer** | Documentation, examples, guides | Week 3 |

---

## Dependencies & Blockers

```
Phase 1  (no blockers)
  ├─► Phase 2 (blocked on Phase 1 complete)
  │    ├─► Phase 3 (blocked on Phase 2 complete)
  │    │    └─► Phase 4 (blocked on Phase 3 complete)
  │
  └─► Parallel: Documentation (starts Week 3, doesn't block)
```

**Critical Path:**
- Week 2: Phase 1 must be complete for Phase 2 to start
- Week 4: Phase 2 must be complete for Phase 3 to start
- Week 6: Phase 3 must be complete for Phase 4 to start

---

## Metrics & Success

### Code Quality
- **Coverage:** >80% for all packages
- **Linting:** Zero warnings in main branches
- **Tests:** 100% pass rate before release

### Performance
- **Startup:** <100ms from invocation to ready
- **Catalog Build:** <30s for 247-module provider
- **Compose:** <5s for typical 5-module stack

### Release Quality
- **Smoke Tests:** 100% pass
- **Cross-Platform:** Works on macOS, Linux, Windows+WSL
- **Distribution:** All 4 methods work (binary, npm, brew, docker)

### User Adoption
- **Documentation:** Complete, with examples
- **Discoverability:** `--help` sufficient for self-service
- **Error Messages:** Helpful suggestions for >90% errors

---

## Escalation & Decision Making

| Decision Type | Owner | Timeline |
|---|---|---|
| Architecture design | Lead Architect | Before Phase 1 ends |
| API/CLI design changes | Project Lead | Before implementation |
| Performance targets | Tech Lead | Week 1 |
| Scope changes | Project Manager | Weekly review |
| Release timeline | Project Manager | Before Phase 4 |

---

## Post-Release (Week 9+)

- Bug fixes + hotfixes
- Community feedback integration
- Performance monitoring
- Feature requests evaluation
- 1.0.1 patch (if needed)
- Planning for v1.1+

---

## Key Assumptions

1. **Terraform Registry API is stable** — If rate limits hit, implement caching
2. **Go 1.21+ available** — Pinned in go.mod
3. **Module complexity is manageable** — If not, refactor generators in v1.1
4. **Team availability** — Full-time equivalent from all roles

---

## Known Risks

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Registry API rate limits | Medium | High | Implement caching early (Phase 2) |
| Cross-platform test coverage | High | Medium | Automated Linux + manual Windows |
| Complex module types | Medium | High | Test with real modules in Phase 3 |
| Release automation complexity | Medium | Medium | Use GoReleaser (proven tool) |

---

## Next Steps

1. **Week 0 (This Week):** Finalize team assignments, setup repo
2. **Week 1:** Kick off Phase 1, daily standups
3. **Weekly:** Status update, blockers review
4. **After Each Phase:** Retrospective + lessons learned

---

**Last Updated:** 2026-04-22  
**Next Review:** End of Week 2 (Phase 1 complete)

---

## Progress Log

### 2026-04-22 — Phase 1 Scaffolding
- ✅ Repository structure created per `REPOSITORY_SETUP.md`
- ✅ Go module initialized (`github.com/tiziano093/infra-composer-cli`, Go 1.22)
- ✅ `Makefile` with build/test/lint/fmt/clean/release/docker/docs targets
- ✅ `.gitignore`, `.golangci.yml`, `.goreleaser.yml`, `Dockerfile` (distroless)
- ✅ GitHub Actions: `test.yml`, `lint.yml`, `release.yml`, `docker.yml`
- ✅ Apache 2.0 `LICENSE`, `README.md`, `CHANGELOG.md`
- ✅ PR + issue templates (bug, feature, docs)
- ✅ `scripts/release.sh` cross-platform build script
- ✅ Stub `main.go` builds and runs: `./bin/infra-composer --version` works

**Next:** Cobra root command, Viper config, structured logging, error/exit-code framework, unit tests.

### 2026-04-22 — Phase 1 Foundation Complete
- ✅ Dependencies added: `cobra`, `viper`, `testify` (`go.mod` bumped to Go 1.23)
- ✅ `internal/cli/errors.go`: `CLIError` + `ExitCode` constants (1–10) per ARCHITECTURE.md
- ✅ `internal/cli/root.go`: Cobra root with persistent flags (`--config`, `--log-level`, `--log-format`, `--format`, `--verbose`, `--quiet`); flag overrides applied on top of config; `cli.Execute` returns the right exit code
- ✅ `internal/config/{defaults.go,config.go}`: Viper-backed hierarchy defaults → `~/.infra-composer/config.yaml` → `INFRA_COMPOSER_*` env → flags; explicit `--config` errors hard on missing file, default path is best-effort
- ✅ `internal/output/log.go`: slog logger (text/json), `ParseLevel`, quiet mode forces `error`
- ✅ `internal/commands/runtime.go`: shared `Runtime` injected via `context`
- ✅ `internal/commands/version.go`: `version` subcommand with text + JSON output and a local `--format` override
- ✅ `cmd/infra-composer/main.go`: minimal entry, delegates to `cli.Execute` with build-time vars
- ✅ Unit tests: errors mapping, log levels + handler selection, config hierarchy (defaults / env / file / file+env / explicit-missing), version command (text / JSON / local flag override) — all green
- ✅ Smoke tests on built binary: `--help`, `--version`, `version`, `version --format json`, exit code `2` on unknown flag
- ⚠️ `golangci-lint` not installed locally; CI workflow remains the source of truth for lint

**Next:** Phase 2 — catalog schema parsing, builder pipeline, exporter, searcher, and the `catalog` / `search` commands.

