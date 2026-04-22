# Infra-Composer CLI — Technical Architecture

**Version:** 1.0  
**Status:** Design Phase  
**Audience:** Architects, Senior Developers  
**Last Updated:** 2026-04-22

---

## System Overview

Infra-Composer CLI is a **modular, layered architecture** designed for extensibility, testability, and performance.

```
┌─────────────────────────────────────────────────────┐
│         CLI Layer (User Interface)                   │
│  Root Command → Subcommands (Cobra)                 │
├─────────────────────────────────────────────────────┤
│         Command Layer (Business Logic)               │
│  - catalog    - search      - dependencies           │
│  - interface  - compose     - version                │
├─────────────────────────────────────────────────────┤
│         Domain Layer (Core Processing)               │
│  ┌──────────────┐  ┌──────────────┐ ┌──────────────┐│
│  │  Catalog     │  │ Terraform    │ │ Config       ││
│  │  ├─ Schema   │  │ ├─ Generator │ │ ├─ Env Vars  ││
│  │  ├─ Builder  │  │ ├─ Templates │ │ ├─ YAML      ││
│  │  ├─ Searcher │  │ ├─ Support   │ │ └─ Defaults  ││
│  │  └─ Resolver │  │ └─ Validator │ └──────────────┘│
│  └──────────────┘  └──────────────┘                 │
├─────────────────────────────────────────────────────┤
│         Infrastructure Layer                         │
│  ┌──────────────┐  ┌──────────────┐ ┌──────────────┐│
│  │  Git         │  │ Output       │ │ Logging      ││
│  │  ├─ Remote   │  │ ├─ JSON      │ │ ├─ Debug     ││
│  │  └─ Tags     │  │ ├─ YAML      │ │ ├─ Info      ││
│  │              │  │ └─ Table     │ │ └─ Error     ││
│  └──────────────┘  └──────────────┘ └──────────────┘│
└─────────────────────────────────────────────────────┘
```

---

## Directory Structure & Modules

### `cmd/infra-composer/`
**Purpose:** CLI entry point  
**Responsibility:** Program initialization, version injection at build time

```
cmd/infra-composer/
├── main.go              # Entry point (initialize config, root command)
└── _docs/
    └── gen.go           # Auto-generate markdown CLI reference
```

**Key Design:**
- Minimal, stays in charge of only bootstrapping
- Version + build info injected at build time (`-ldflags`)
- Delegates to `internal/cli` for actual command logic

---

### `internal/cli/`
**Purpose:** CLI framework layer (Cobra setup)  
**Responsibility:** Root command, global flags, middleware

```
internal/cli/
├── root.go              # Root command definition, global flags
└── middleware.go        # Logging, error handling, output formatting
```

**Key Design:**
- `root.go`: All CLI commands are registered here
- Flags defined at root level (--verbose, --quiet, --format, --config)
- Middleware handles pre/post-command logic (logging setup, error conversion)

---

### `internal/commands/`
**Purpose:** Command implementations (business logic entry points)  
**Responsibility:** Each command validates input → calls domain layer → formats output

```
internal/commands/
├── catalog.go           # catalog build|export|list|validate|info
├── search.go            # search <keyword> [--group] [--limit]
├── dependencies.go      # dependencies <module> [--depth] [--check-cycles]
├── interface.go         # interface <modules> [--required-only] [--full]
├── compose.go           # compose [--modules] [--request] [--dry-run]
└── version.go           # version [--format json]
```

**Key Design:**
- Each command is responsible for:
  1. **Parsing** its specific flags
  2. **Validating** inputs (file existence, schema validity)
  3. **Calling** domain layer (catalog, terraform, etc.)
  4. **Error handling** (convert domain errors to CLI errors)
  5. **Output formatting** (delegate to output layer)
- No business logic — orchestration only

**Example Command Structure:**
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

---

### `internal/catalog/`
**Purpose:** Catalog management domain  
**Responsibility:** Build, parse, search, validate catalog schemas

```
internal/catalog/
├── schema.go            # Schema types, parsing, validation
├── builder.go           # Build pipeline (discover → crawl → normalize → export)
├── exporter.go          # Export normalized catalog to schema.json
├── searcher.go          # Module search + filtering (fuzzy, AND logic)
├── dependency.go        # Dependency graph resolution + cycle detection
└── normalizer.go        # Terraform registry normalization
```

**Key Design:**

**Schema types** (`schema.go`):
```go
type Schema struct {
    Provider  string          // e.g., "hashicorp/aws"
    Version   string          // Semver
    Modules   []ModuleEntry   // All modules in catalog
}

type ModuleEntry struct {
    Name        string        // e.g., "aws_vpc"
    Type        string        // resource|data
    Source      string        // GitHub URL
    Description string
    Variables   []Variable
    Outputs     []Output
}
```

**Builder pipeline** (`builder.go`):
1. **Discover:** Query Terraform Registry API for provider metadata
2. **Crawl:** Extract all resource/data source definitions
3. **Normalize:** Convert to internal schema format
4. **Generate:** Create module stubs (variable/output definitions)
5. **Export:** Write to `schema.json`

**Searcher** (`searcher.go`):
- Keyword search with fuzzy matching
- AND logic (all keywords must match)
- Group filtering (`--group network`)
- Type filtering (resource, data, etc.)
- Limit results

**Dependency Resolver** (`dependency.go`):
- Build directed graph from variable references
- DFS-based cycle detection
- Return dependency tree with depth info

---

### `internal/terraform/`
**Purpose:** Terraform code generation  
**Responsibility:** Generate HCL files + support files

```
internal/terraform/
├── generator.go         # Main generator: orchestrates file generation
├── templates.go         # HCL template definitions
├── support.go           # Support files (CI/CD, tflint, terraform-docs, tfvars)
├── validator.go         # HCL syntax validation
└── naming.go            # Naming conventions (snake_case, "this" pattern)
```

**Key Design:**

**Generator** (`generator.go`):
Orchestrates generation of 5 core Terraform files:
1. `providers.tf` — Provider block with version constraints
2. `variables.tf` — Input variables with validation
3. `locals.tf` — Local values for cross-module wiring
4. `main.tf` — Module calls with variable references
5. `outputs.tf` — Output values for stack consumers

**Templates** (`templates.go`):
- Go text/template-based HCL generation
- Embedded templates for each file type
- Template variables: module info, variables, outputs

**Support Files** (`support.go`):
- `azure-pipelines.yml` or `github-actions.yml` (based on provider flag)
- `.pre-commit-config.yaml` (tflint, terraform-fmt)
- `.tflint.hcl` (linting rules)
- `.terraform-docs.yml` (auto-doc generation)
- `environments/<ENV>/<COMPONENT>.tfvars` (environment-specific variables)

**Naming Conventions** (`naming.go`):
- Variables: snake_case
- Local values: snake_case
- Module calls: `this_<module_name>` or `<purpose>_<number>`
- Data sources: `this_<resource_type>_data`

---

### `internal/config/`
**Purpose:** Configuration loading & defaults  
**Responsibility:** Load config from env vars, YAML, apply defaults

```
internal/config/
├── config.go            # Config struct, loading from YAML
├── env.go               # Env var loading (INFRA_COMPOSER_*)
└── defaults.go          # Default values for all settings
```

**Key Design:**

**Config loading order** (highest priority last):
1. **Defaults** → hardcoded sensible defaults
2. **Config file** → `~/.infra-composer/config.yaml`
3. **Env vars** → `INFRA_COMPOSER_*` variables
4. **Flags** → CLI flags (highest priority)

**Config structure:**
```go
type Config struct {
    Logging LogConfig
    Catalog CatalogConfig
    Terraform TerraformConfig
    Git GitConfig
    Profiles map[string]ProfileConfig
}
```

**Environment variables:**
```
INFRA_COMPOSER_LOG_LEVEL=debug
INFRA_COMPOSER_SCHEMA=/path/to/schema.json
INFRA_COMPOSER_MODULES_DIR=/path/to/modules
INFRA_COMPOSER_OUTPUT_DIR=./stack
INFRA_COMPOSER_CI_PROVIDER=azure|github
```

---

### `internal/git/`
**Purpose:** Git operations  
**Responsibility:** Extract remote URLs, fetch semver tags

```
internal/git/
├── remote.go            # Extract catalog URL from git remote
└── tags.go              # Fetch and parse semver tags
```

**Key Design:**
- Used by `compose` command to auto-detect catalog source
- Caches git operations to avoid repeated external calls
- Falls back to explicit `--source-template` flag if git fails

---

### `internal/output/`
**Purpose:** Output formatting layer  
**Responsibility:** Format results for JSON, YAML, or human-readable table

```
internal/output/
├── log.go               # Structured logging
├── json.go              # JSON marshaling
├── yaml.go              # YAML formatting
└── table.go             # ASCII table output
```

**Key Design:**
- Global output formatter handles all output consistency
- Supports format override per command
- Logging separated from result output
- Error output always structured (even in text mode)

---

### `pkg/`
**Purpose:** Public API (exported types for library use)  
**Responsibility:** Define public interfaces consumed by external users

```
pkg/
├── terraform/
│   └── terraform.go     # Exported types (Generator, Schema, etc.)
└── catalog/
    └── catalog.go       # Exported types (Catalog, Module, etc.)
```

**Key Design:**
- CLI uses `internal/` packages
- External users import from `pkg/`
- Keeps internal implementation details private
- Enables infra-composer as a Go library

---

## Data Flow & Request Paths

### Path 1: Catalog Build
```
CLI (catalog build --provider aws)
  ↓
Command (catalog.go)
  ├─ Validate flags
  ├─ Load config
  ↓
Domain (catalog/builder.go)
  ├─ Discover: Query Terraform Registry
  ├─ Crawl: Extract resources + data sources
  ├─ Normalize: Convert to internal format
  ├─ Generate: Create module stubs
  ↓
Export (catalog/exporter.go)
  ├─ Write schema.json
  ↓
Output (output/json.go)
  └─ Print result (JSON or text)
```

### Path 2: Module Search
```
CLI (search network subnet)
  ↓
Command (search.go)
  ├─ Parse keywords + flags
  ├─ Load schema.json
  ↓
Domain (catalog/searcher.go)
  ├─ Fuzzy match each keyword
  ├─ AND logic: modules matching all keywords
  ├─ Filter by group/type if specified
  ↓
Output (output/table.go)
  └─ Print results (table, JSON, or YAML)
```

### Path 3: Stack Composition (Core Value)
```
CLI (compose --modules "vpc subnet" --output-dir ./stack)
  ↓
Command (compose.go)
  ├─ Parse modules + flags
  ├─ Load schema.json + module directory
  ├─ Validate inputs
  ↓
Domain:
  ├─ catalog/searcher.go: Find module metadata
  ├─ catalog/dependency.go: Resolve dependencies
  ├─ terraform/generator.go: Generate HCL files
  ├─ terraform/support.go: Generate support files
  ├─ git/remote.go: Auto-detect source template
  ↓
Output:
  ├─ Write files (or dry-run preview)
  └─ Print result (JSON with file list + modules used)
```

---

## Error Handling Strategy

### Error Types (Exit Codes)
```
0   → Success
1   → Generic error (unclassified)
2   → Invalid arguments (flag parsing error)
3   → File not found (schema, modules, config)
4   → Schema validation failed
5   → Module not found
6   → Dependency resolution failed
7   → Git operation failed
8   → Terraform generation error
9   → Network error (downloading catalog)
10  → Permission denied
```

### Error Output (JSON Format)
```json
{
  "error": {
    "code": 5,
    "message": "Module not found",
    "details": "Module 'aws_rds_cluster' not found in schema",
    "suggestions": [
      "Did you mean 'aws_db_cluster'?",
      "Use 'infra-composer search database' to find available modules"
    ]
  },
  "timestamp": "2026-04-22T11:22:00Z"
}
```

### Error Handling Design
- Each domain package defines custom error types
- Command layer converts domain errors to CLI errors with suggestions
- Middleware adds context (timestamp, request ID for tracing)
- Logging captures full stack trace at debug level

---

## Configuration & Dependency Injection

### Configuration Hierarchy
```
1. Defaults (hardcoded)
   ↓
2. Config file (~/.infra-composer/config.yaml)
   ↓
3. Environment variables (INFRA_COMPOSER_*)
   ↓
4. CLI flags (highest priority)
```

### Dependency Injection Pattern
Each command receives injected dependencies:
```go
type CatalogCommand struct {
    logger          Logger
    config          *Config
    catalogService  *catalog.Service
    outputFormatter OutputFormatter
}
```

Benefits:
- Easy to test (mock dependencies)
- Clear dependencies (visible in constructor)
- Flexible (swap implementations for different providers)

---

## Testing Architecture

### Unit Tests
- Config loading + merging
- Catalog parsing + validation
- Dependency graph construction
- HCL template rendering
- Output formatting

**Location:** `test/unit/`  
**Pattern:** `*_test.go` files  
**Mock Strategy:** Hand-written mocks for small surface areas

### Integration Tests
- Full `catalog build` pipeline (mock Terraform registry API)
- Module search + dependency resolution
- TF file generation end-to-end
- Support file generation
- CLI command execution

**Location:** `test/integration/`  
**Pattern:** `*_integration_test.go` files  
**Fixtures:** `test/fixtures/` (schemas, requests, expected outputs)

### Test Coverage Target
- Unit: 80%+ coverage per package
- Integration: Critical paths covered
- E2E: Smoke tests in CI/CD

---

## Performance Considerations

### CLI Startup
- **Target:** <100ms from CLI start to command execution
- **Strategy:**
  - Lazy-load config (only if needed)
  - Lazy-load schema (not until command uses it)
  - Minimal dependencies at startup

### Catalog Build
- **Target:** <30 seconds for aws provider
- **Strategy:**
  - Batch Terraform Registry API calls
  - Cache intermediate results
  - Parallel crawling of provider data

### Stack Composition
- **Target:** <5 seconds to generate full stack
- **Strategy:**
  - Pre-load schema at command startup
  - Efficient dependency resolution (memoization)
  - Parallel file generation (goroutines)

---

## Extension Points

### How to Add a New Cloud Provider
1. Implement provider-specific logic in `catalog/builder.go`
2. Add provider detection in `compose` command
3. Add new template in `terraform/templates.go`
4. Add support file variant in `terraform/support.go`

### How to Add a New Output Format
1. Add formatter in `output/` package
2. Register in command flags
3. Call appropriate formatter in command

### How to Add a New Command
1. Create command file in `internal/commands/`
2. Register in `internal/cli/root.go`
3. Define flags and execute logic
4. Add corresponding domain logic if needed

---

## Key Design Principles

| Principle | Implementation |
|-----------|-----------------|
| **Modularity** | Each package has single responsibility |
| **Testability** | Interfaces for all external dependencies (IO, HTTP, filesystem) |
| **Portability** | No platform-specific code outside build-time flags |
| **Extensibility** | Clear interfaces for adding new commands, providers, formats |
| **User Experience** | Clear error messages, helpful suggestions, structured output |
| **Performance** | Lazy loading, caching, parallel processing where appropriate |

---

## Security Considerations

- **No Hardcoded Credentials:** All credentials loaded from env vars or config
- **File Permissions:** Generated files use sensible defaults (0644 for most, 0600 for tfvars)
- **Input Validation:** All user input validated before processing
- **Dependency Management:** Minimal dependencies, pinned versions in go.mod
- **Error Messages:** No sensitive data (paths, full errors) in JSON output by default

---

## Future Architecture Enhancements

- **Plugin System:** Load custom providers/formatters as plugins
- **Caching Layer:** Persistent cache for Terraform Registry data
- **Observability:** OpenTelemetry integration for tracing
- **Remote Execution:** Execute composition in remote environment (with auth)
- **Multi-Catalog Support:** Compose from multiple catalogs simultaneously

---

**Last Reviewed:** 2026-04-22  
**Next Architecture Review:** After Phase 2 completion  
