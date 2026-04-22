# infra-composer CLI — Design Guide

## Executive Summary

Transform `infra-composer` skill (current: hardcoded bash scripts scattered) into portable, modular CLI tool.

**Goals:**
- Single entry point: `infra-composer <command> [args]`
- Zero environment assumptions (works on macOS, Linux, Windows+WSL)
- Embed all scripts + logic (no external dependencies on `~/.claude/`)
- Pipeline-ready (non-interactive, JSON output, exit codes)
- Testable: unit + integration tests
- Distribute as: standalone binary + npm package + Docker image

---

## 1. Architecture Overview

```
infra-composer-cli/
├── cmd/                          # Entry point
│   └── infra-composer/main.go
├── internal/
│   ├── cli/                       # CLI framework
│   │   ├── root.go               # Root command + global flags
│   │   └── middleware.go         # Logging, error handling
│   ├── commands/                  # Command implementations
│   │   ├── catalog.go            # catalog build|export|list|status
│   │   ├── search.go             # search <keyword>
│   │   ├── dependencies.go       # dependencies <module> --depth N
│   │   ├── interface.go          # interface <module> --format json|yaml
│   │   ├── compose.go            # compose --schema ... --modules ...
│   │   └── version.go            # version
│   ├── catalog/                   # Catalog management
│   │   ├── schema.go             # schema parsing + validation
│   │   ├── builder.go            # build-catalog pipeline
│   │   ├── exporter.go           # export to schema.json
│   │   ├── searcher.go           # module search + filtering
│   │   └── dependency.go         # dependency resolution
│   ├── terraform/                 # Terraform generation
│   │   ├── generator.go          # main TF file generator
│   │   ├── templates.go          # HCL templates
│   │   ├── support.go            # support files (CI/CD, etc.)
│   │   └── validator.go          # HCL validation
│   ├── config/                    # Configuration
│   │   ├── env.go                # env var loading (INFRA_COMPOSER_*)
│   │   ├── config.go             # config file parsing (~/.infra-composer/config.yaml)
│   │   └── defaults.go           # sensible defaults
│   ├── git/                       # Git operations
│   │   ├── remote.go             # Extract catalog URL from git remote
│   │   └── tags.go               # Fetch semver tags
│   └── output/                    # Output formatting
│       ├── json.go               # JSON marshaling
│       ├── yaml.go               # YAML formatting
│       ├── table.go              # ASCII table output
│       └── log.go                # Structured logging
├── pkg/                           # Public packages (reusable)
│   ├── terraform/terraform.go    # Exported types for library use
│   └── catalog/catalog.go
├── test/                          # Tests
│   ├── unit/
│   ├── integration/
│   └── fixtures/                  # Test data (example schemas, etc.)
├── scripts/                       # Build + release scripts
│   ├── build.sh                  # Cross-platform compilation
│   ├── test.sh
│   ├── release.sh                # Build binaries + push to GitHub releases
│   └── docker.sh                 # Build Docker image
├── build/                         # Build artifacts (gitignored)
│   └── infra-composer-*
├── docs/                          # User documentation
│   ├── README.md
│   ├── INSTALL.md                # Installation methods
│   ├── QUICKSTART.md             # 5-min start
│   ├── CLI.md                    # Command reference
│   ├── CONFIG.md                 # Configuration guide
│   ├── PIPELINE.md               # CI/CD integration
│   └── CONTRIBUTING.md           # Developer guide
├── examples/                      # Example usage
│   ├── aws-basic.yaml            # Example request
│   ├── azure-advanced.yaml
│   └── outputs/                  # Sample generated stacks
├── Dockerfile                     # Multi-stage Docker build
├── .github/
│   └── workflows/
│       ├── test.yml              # Run tests on push
│       ├── release.yml           # Cross-platform binary release on tag
│       └── docker.yml            # Push to ghcr.io on release
├── go.mod                         # Go module definition
├── go.sum
├── Makefile                       # Build targets
├── CHANGELOG.md
└── LICENSE

```

---

## 2. Command Structure

### Root Command
```bash
infra-composer [global-flags] <command> [command-flags]

Global flags:
  --verbose (-v)           Verbose logging (repeat for more: -vv, -vvv)
  --quiet (-q)             Suppress output
  --format json|yaml|text  Output format (default: text)
  --config FILE            Config file path (default: ~/.infra-composer/config.yaml)
  --no-color               Disable colored output
```

### Command: `catalog`

Build, export, validate, and list catalog modules.

```bash
# Build catalog from scratch (discover → crawl → normalize → generate → export)
infra-composer catalog build \
  --provider hashicorp/aws \
  --output-dir ./modules \
  [--skip-discover] [--skip-crawl] [--dry-run]

# Export existing normalized catalog → schema.json
infra-composer catalog export \
  --provider hashicorp/aws \
  --source-dir /path/to/normalized \
  --output ./modules/schema.json

# List available providers + their build status
infra-composer catalog list [--status ready|normalized|discovered]

# Validate existing schema.json
infra-composer catalog validate --schema ./modules/schema.json

# Show catalog info
infra-composer catalog info --schema ./modules/schema.json
```

**Output (default: text, alternate: --format json|yaml):**
```
Provider:  hashicorp/aws
Version:   6.35.1
Modules:   247
Status:    ready
Updated:   2026-04-22T11:22:00Z
```

---

### Command: `search`

Find modules by keyword or group.

```bash
# Search by keyword (AND logic, fuzzy match)
infra-composer search \
  --schema ./modules/schema.json \
  network subnet

# Search by group
infra-composer search \
  --schema ./modules/schema.json \
  --group network

# Show matching modules + metadata
infra-composer search \
  --schema ./modules/schema.json \
  vpc \
  --output-format json

# Limit results
infra-composer search --schema ./modules/schema.json vpc --limit 5
```

**Output (default: text table):**
```
Name                  Provider        Type      Description
─────────────────────────────────────────────────────────────
aws_vpc_peering       hashicorp/aws   resource  Peering connection
aws_vpc               hashicorp/aws   resource  Virtual Private Cloud
```

---

### Command: `dependencies`

Resolve and display module dependency trees.

```bash
# Show all dependencies for a module
infra-composer dependencies \
  --schema ./modules/schema.json \
  aws_vpc \
  --depth 2

# Output as JSON (for programmatic use)
infra-composer dependencies \
  --schema ./modules/schema.json \
  aws_vpc \
  --format json

# Check for circular deps
infra-composer dependencies \
  --schema ./modules/schema.json \
  aws_instance \
  --check-cycles
```

**Output (text tree):**
```
aws_instance
├── aws_security_group (required)
│   └── aws_vpc (required)
└── aws_subnet (required)
    └── aws_vpc (required)
```

---

### Command: `interface`

Extract module interface (variables, outputs, required vs optional).

```bash
# Get interface for one module
infra-composer interface \
  --modules-dir ./modules \
  aws_vpc

# Multiple modules at once
infra-composer interface \
  --modules-dir ./modules \
  aws_vpc aws_subnet aws_instance

# YAML format, required-only (token-efficient)
infra-composer interface \
  --modules-dir ./modules \
  aws_vpc \
  --format yaml \
  --required-only

# Full interface (all variables + outputs)
infra-composer interface \
  --modules-dir ./modules \
  aws_vpc \
  --full

# Output as JSON (for programmatic composition)
infra-composer interface \
  --modules-dir ./modules \
  aws_vpc aws_subnet \
  --format json
```

**Output (YAML, --required-only):**
```yaml
aws_vpc:
  source: "github.com/example/terraform-aws-modules//modules/vpc?ref=v5.1.2"
  required_variables:
    cidr_block:
      type: string
      description: "CIDR block for VPC"
  outputs:
    id:
      type: string
      description: "VPC ID"
```

---

### Command: `compose`

Core workflow: Search modules, resolve deps, read interfaces, generate TF consumer stack.

```bash
# High-level: pass request + get full stack
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --request "vpc with 2 subnets and ec2 instance" \
  --output-dir ./stack \
  [--format json]

# Explicit modules (skip search)
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --modules "aws_vpc aws_subnet aws_instance" \
  --output-dir ./stack

# With provider + catalog metadata
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --modules "aws_vpc aws_subnet" \
  --output-dir ./stack \
  --provider aws \
  --catalog-version v5.1.2 \
  --source-template "github.com/example/terraform-aws-modules//modules"

# Dry-run: preview without writing files
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --modules "aws_vpc aws_subnet" \
  --output-dir ./stack \
  --dry-run \
  --format json

# Generate support files too (CI/CD, tflint, etc.)
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --modules "aws_vpc aws_subnet" \
  --output-dir ./stack \
  --include-support-files \
  --env prod \
  --component app
```

**Output (JSON, dry-run):**
```json
{
  "status": "ok",
  "dry_run": true,
  "output_dir": "./stack",
  "files": [
    "providers.tf",
    "variables.tf",
    "locals.tf",
    "main.tf",
    "outputs.tf",
    "azure-pipelines.yml",
    ".pre-commit-config.yaml",
    ".tflint.hcl",
    ".terraform-docs.yml",
    "environments/prod/app.tfvars"
  ],
  "modules": [
    {"name": "aws_vpc", "source": "github.com/.../vpc?ref=v5.1.2"},
    {"name": "aws_subnet", "source": "github.com/.../subnet?ref=v5.1.2"}
  ],
  "assumptions": {
    "provider": "aws",
    "provider_version": "6.35.1",
    "catalog_version": "v5.1.2"
  }
}
```

---

### Command: `version`

Show CLI + catalog versions.

```bash
infra-composer version [--format json]
```

**Output:**
```
infra-composer v1.0.0
Built:    2026-04-22
Commit:   abc123def
Schema:   v5.1.2 (from ./modules/schema.json)
```

---

## 3. Configuration

### Environment Variables
```bash
# Catalog paths
INFRA_COMPOSER_SCHEMA=/path/to/schema.json
INFRA_COMPOSER_MODULES_DIR=/path/to/modules

# Terraform generation
INFRA_COMPOSER_OUTPUT_DIR=./stack
INFRA_COMPOSER_SOURCE_TEMPLATE=github.com/example/terraform-aws-modules//modules

# Logging
INFRA_COMPOSER_LOG_LEVEL=debug          # debug|info|warn|error (default: info)
INFRA_COMPOSER_LOG_FORMAT=json          # json|text (default: text)

# CI/CD support
INFRA_COMPOSER_AGENT_POOL=default       # ADO agent pool name
INFRA_COMPOSER_CI_PROVIDER=azure        # azure|github|gitlab
```

### Config File (~/.infra-composer/config.yaml)
```yaml
# Global defaults
logging:
  level: info
  format: text
  colorize: true

# Catalog settings
catalog:
  schema_path: ~/.infra-composer/schemas/aws.json
  modules_dir: ~/.infra-composer/modules
  auto_update: true
  auto_update_interval: 24h

# Terraform generation defaults
terraform:
  output_dir: ./stack
  source_template: github.com/example/terraform-aws-modules//modules
  include_support_files: true
  support_files:
    ci_provider: azure
    agent_pool: default
    environment: prod

# Git settings (for auto-detection of catalog URL)
git:
  remote_name: origin

# Profiles (environment-specific configs)
profiles:
  dev:
    terraform:
      environment: dev
      output_dir: ./stack-dev
  prod:
    terraform:
      environment: prod
      output_dir: ./stack-prod
```

**Profile usage:**
```bash
infra-composer compose --schema ./modules/schema.json ... --profile prod
```

---

## 4. Error Handling

Robust error codes + messages.

```bash
# Exit codes
0   - Success
1   - Generic error
2   - Invalid arguments
3   - File not found (schema, modules, config)
4   - Schema validation failed
5   - Module not found
6   - Dependency resolution failed
7   - Git operation failed
8   - Terraform generation error
9   - Network error (downloading catalog)
10  - Permission denied
```

**Error output (--format json):**
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

---

## 5. Pipeline Integration

### GitHub Actions Example
```yaml
name: Generate Infrastructure

on:
  workflow_dispatch:
    inputs:
      provider:
        description: "Cloud provider"
        required: true
        default: "aws"

jobs:
  compose:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Download infra-composer
        run: |
          curl -sSL https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-linux-amd64 \
            -o /usr/local/bin/infra-composer
          chmod +x /usr/local/bin/infra-composer
      
      - name: Generate stack
        run: |
          infra-composer compose \
            --schema ./catalog/aws-schema.json \
            --modules-dir ./catalog/modules \
            --modules "aws_vpc aws_subnet aws_instance" \
            --output-dir ./infrastructure \
            --format json > compose-output.json
      
      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: terraform-stack
          path: infrastructure/
```

### Azure Pipelines Example
```yaml
trigger:
  - main

pool:
  vmImage: 'ubuntu-latest'

steps:
  - script: |
      curl -sSL https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-linux-amd64 \
        -o $(Agent.TempDirectory)/infra-composer
      chmod +x $(Agent.TempDirectory)/infra-composer
    displayName: 'Download infra-composer'

  - script: |
      $(Agent.TempDirectory)/infra-composer compose \
        --schema ./catalog/aws-schema.json \
        --modules-dir ./catalog/modules \
        --modules "aws_vpc aws_subnet" \
        --output-dir $(Build.ArtifactStagingDirectory)/stack
    displayName: 'Generate Terraform stack'

  - task: PublishBuildArtifacts@1
    inputs:
      pathToPublish: $(Build.ArtifactStagingDirectory)/stack
      artifactName: terraform-stack
```

---

## 6. Distribution

### Installation Methods

**1. Pre-built Binaries (GitHub Releases)**
```bash
# macOS (Intel)
curl -sSL https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-darwin-amd64 \
  -o /usr/local/bin/infra-composer && chmod +x /usr/local/bin/infra-composer

# macOS (Apple Silicon)
curl -sSL https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-darwin-arm64 \
  -o /usr/local/bin/infra-composer && chmod +x /usr/local/bin/infra-composer

# Linux
curl -sSL https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-linux-amd64 \
  -o /usr/local/bin/infra-composer && chmod +x /usr/local/bin/infra-composer

# Windows (via PowerShell)
Invoke-WebRequest -Uri https://github.com/tiziano093/infra-composer/releases/download/v1.0.0/infra-composer-windows-amd64.exe \
  -OutFile C:\Program Files\infra-composer\infra-composer.exe
```

**2. Homebrew (macOS)**
```bash
brew tap tiziano093/infra-composer
brew install infra-composer
```

**3. Docker**
```bash
docker run --rm \
  -v $(pwd):/workspace \
  ghcr.io/tiziano093/infra-composer:latest \
  compose --schema /workspace/modules/schema.json \
           --modules-dir /workspace/modules \
           --modules "aws_vpc aws_subnet" \
           --output-dir /workspace/stack
```

**4. npm Package**
```bash
npm install -g @tiziano093/infra-composer

infra-composer --version
```

---

## 7. Testing Strategy

### Unit Tests
- Config loading + defaults merging
- Catalog schema parsing + validation
- Dependency graph construction + cycle detection
- HCL template rendering
- JSON/YAML output formatting

### Integration Tests
- Full `catalog build` pipeline (mock Terraform registry)
- Module search + dependency resolution
- TF file generation end-to-end
- Support file generation (CI/CD, tflint, etc.)
- Pipeline smoke tests (GitHub Actions, Azure Pipelines)

### Test Fixtures
```
test/fixtures/
├── schemas/
│   └── aws-schema.json          # Mock schema (5-10 modules)
├── modules/
│   ├── aws_vpc/
│   │   ├── variables.tf
│   │   └── outputs.tf
│   └── aws_subnet/
│       ├── variables.tf
│       └── outputs.tf
├── requests/
│   ├── basic-vpc.yaml
│   └── advanced-multi-region.yaml
└── expected-outputs/
    └── basic-vpc-stack/
        ├── providers.tf
        └── main.tf
```

**Test file naming:**
- `*_test.go` — unit tests
- `*_integration_test.go` — integration tests
- Run: `go test ./... -v` or `make test`

---

## 8. Development Workflow

### Local Build
```bash
# Install dependencies
go mod download

# Build
go build -o bin/infra-composer ./cmd/infra-composer

# Run
./bin/infra-composer --version

# Test
go test ./...

# Lint
golangci-lint run ./...

# Generate docs
make docs
```

### Make Targets
```makefile
.PHONY: build test lint docs release clean docker

build:
	go build -o bin/infra-composer ./cmd/infra-composer

test:
	go test ./... -v -cover

lint:
	golangci-lint run ./...

docs:
	go run ./cmd/infra-composer/_docs/gen.go

release:
	./scripts/release.sh

docker:
	docker build -t ghcr.io/tiziano093/infra-composer:latest .

clean:
	rm -rf bin/ build/
```

---

## 9. Versioning + Releases

### Semver Strategy
- `MAJOR`: Breaking CLI changes, config schema changes
- `MINOR`: New commands, new flags, new output formats
- `PATCH`: Bug fixes, small improvements

### Release Process (GitHub Actions)
1. Tag commit: `git tag v1.0.0`
2. Push tag: `git push origin v1.0.0`
3. GitHub Actions `release.yml` workflow:
   - Builds cross-platform binaries (macOS, Linux, Windows)
   - Generates checksums + signature
   - Creates GitHub release + uploads binaries
   - Pushes Docker image to ghcr.io
   - Submits to Homebrew tap

---

## 10. Key Improvements Over Skill

| Aspect | Current (Skill) | CLI |
|--------|-----------------|-----|
| **Entry Point** | Multiple scripts | Single `infra-composer` command |
| **Portability** | Hardcoded `/Users/tiziano/...` paths | Embedded, environment-agnostic |
| **Pipeline Ready** | Non-interactive, text-heavy | JSON output, exit codes, flags |
| **Configuration** | Env vars ad-hoc | config.yaml + env vars + flags |
| **Error Handling** | Generic bash errors | Structured errors w/ suggestions |
| **Discoverability** | Read SKILL.md manually | `infra-composer --help` + man pages |
| **Distribution** | Copy scripts locally | Binary download + package managers |
| **Testing** | Manual verification | Unit + integration tests |
| **Performance** | N/A | ~100ms startup (vs. skill overhead) |

---

## 11. Implementation Phases

**Phase 1 (Weeks 1-2): Core Foundation**
- Go project scaffold + module structure
- Config loading + CLI framework (Cobra)
- Commands: `version`, `help`
- Basic unit tests

**Phase 2 (Weeks 3-4): Catalog Operations**
- Commands: `catalog build|export|list|validate|info`
- Catalog parser + schema validation
- `search` command
- Integration tests for catalog ops

**Phase 3 (Weeks 5-6): Module Composition**
- Commands: `dependencies`, `interface`, `compose`
- Terraform generator + HCL templates
- Support file generation
- End-to-end integration tests

**Phase 4 (Weeks 7-8): Distribution + Polish**
- Cross-platform builds (Makefile + scripts)
- GitHub Actions workflows for CI/CD + releases
- Homebrew tap + Docker image
- npm package
- Documentation + examples

---

## 12. Next Steps

1. **Create GitHub repo:** `infra-composer` (separate from marketplace)
2. **Set up Go module:** `go mod init github.com/tiziano093/infra-composer`
3. **Write Makefile + build scripts**
4. **Scaffold Phase 1:** Root CLI + config
5. **Iterate:** Catalog ops → Module composition → Distribution
6. **Maintain:** Bug fixes, feature requests, semver releases

---

## Files to Create/Update

- [ ] Create `/cmd/infra-composer/main.go` (entry point)
- [ ] Create `/internal/cli/root.go` (Cobra root command)
- [ ] Create `/internal/config/config.go` (config parsing)
- [ ] Create `/internal/commands/*.go` (all commands)
- [ ] Create `/internal/catalog/*.go` (catalog logic)
- [ ] Create `/internal/terraform/*.go` (TF generation)
- [ ] Create `/Dockerfile` (multi-stage build)
- [ ] Create `.github/workflows/*.yml` (CI/CD)
- [ ] Create `/docs/*.md` (user guides)
- [ ] Create `/test/fixtures/*` (test data)
- [ ] Create `/Makefile` (build targets)
- [ ] Create `/go.mod`, `/go.sum`

