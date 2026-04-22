# Infra-Composer CLI — Repository Setup & CI/CD

**Version:** 1.0  
**Status:** Implementation Guide  
**Audience:** DevOps Engineers, Infrastructure Admins  
**Last Updated:** 2026-04-22

---

## Repository Structure

### Core Directories

```
infra-composer-cli/
├── cmd/                          # Application entry points
│   └── infra-composer/
│       ├── main.go               # CLI entry point
│       └── _docs/
│           └── gen.go            # CLI docs generator
│
├── internal/                     # Private packages (not exported)
│   ├── cli/                      # CLI framework
│   │   ├── root.go               # Root command (Cobra)
│   │   └── middleware.go         # Pre/post hooks
│   ├── commands/                 # Subcommand implementations
│   │   ├── catalog.go
│   │   ├── search.go
│   │   ├── dependencies.go
│   │   ├── interface.go
│   │   ├── compose.go
│   │   └── version.go
│   ├── catalog/                  # Catalog domain logic
│   │   ├── schema.go
│   │   ├── builder.go
│   │   ├── exporter.go
│   │   ├── searcher.go
│   │   ├── dependency.go
│   │   └── normalizer.go
│   ├── terraform/                # Terraform generation
│   │   ├── generator.go
│   │   ├── templates.go
│   │   ├── support.go
│   │   ├── validator.go
│   │   └── naming.go
│   ├── config/                   # Configuration
│   │   ├── config.go
│   │   ├── env.go
│   │   └── defaults.go
│   ├── git/                      # Git operations
│   │   ├── remote.go
│   │   └── tags.go
│   └── output/                   # Output formatting
│       ├── log.go
│       ├── json.go
│       ├── yaml.go
│       └── table.go
│
├── pkg/                          # Public packages (exported)
│   ├── terraform/
│   │   └── terraform.go          # Exported types
│   └── catalog/
│       └── catalog.go            # Exported types
│
├── test/                         # Tests & fixtures
│   ├── unit/                     # Unit tests
│   │   ├── config_test.go
│   │   ├── catalog_test.go
│   │   └── ...
│   ├── integration/              # Integration tests
│   │   ├── compose_test.go
│   │   ├── catalog_test.go
│   │   └── ...
│   └── fixtures/                 # Test data
│       ├── schemas/
│       │   ├── aws-schema.json
│       │   └── azure-schema.json
│       ├── modules/
│       │   ├── aws_vpc/
│       │   ├── aws_subnet/
│       │   └── ...
│       ├── requests/
│       │   ├── basic-vpc.yaml
│       │   └── advanced.yaml
│       └── expected-outputs/
│           └── ...
│
├── scripts/                      # Build & utility scripts
│   ├── build.sh                  # Cross-platform compilation
│   ├── test.sh                   # Test runner
│   ├── release.sh                # Release automation
│   ├── docker.sh                 # Docker build
│   └── install.sh                # Installation helper
│
├── build/                        # Generated binaries (gitignored)
│   ├── infra-composer-darwin-amd64
│   ├── infra-composer-darwin-arm64
│   ├── infra-composer-linux-amd64
│   ├── infra-composer-windows-amd64.exe
│   └── checksums.txt
│
├── docs/                         # Documentation
│   ├── PROJECT_OVERVIEW.md       # This document
│   ├── ARCHITECTURE.md           # System design
│   ├── DEVELOPMENT_GUIDELINES.md # Coding standards
│   ├── CONTRIBUTING.md           # How to contribute
│   ├── REPOSITORY_SETUP.md       # Repo setup (this file)
│   ├── TESTING_STRATEGY.md       # Testing approach
│   ├── RELEASE_PROCESS.md        # Release procedures
│   ├── README.md                 # User overview
│   ├── INSTALL.md                # Installation methods
│   ├── QUICKSTART.md             # 5-minute tutorial
│   ├── CLI.md                    # Command reference
│   ├── CONFIG.md                 # Configuration guide
│   └── PIPELINE.md               # CI/CD examples
│
├── examples/                     # Usage examples
│   ├── aws-basic.yaml            # Example: VPC + subnet
│   ├── azure-advanced.yaml       # Example: AKS multi-region
│   └── outputs/
│       ├── basic-vpc/
│       │   ├── providers.tf
│       │   ├── main.tf
│       │   └── ...
│       └── ...
│
├── homebrew/                     # Homebrew formula
│   └── infra-composer.rb
│
├── .github/                      # GitHub-specific config
│   ├── workflows/
│   │   ├── test.yml              # Run tests on push
│   │   ├── lint.yml              # Lint on push
│   │   ├── release.yml           # Build binaries on tag
│   │   └── docker.yml            # Push Docker image
│   ├── issue_templates/
│   │   ├── bug.md
│   │   ├── feature.md
│   │   └── documentation.md
│   └── pull_request_template.md
│
├── .gitignore                    # Git ignore rules
├── .golangci.yml                 # Linter configuration
├── Dockerfile                    # Multi-stage Docker build
├── Makefile                      # Build targets
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
├── package.json                  # npm package wrapper
├── CHANGELOG.md                  # Release notes
├── LICENSE                       # Apache 2.0 or MIT
└── README.md                     # GitHub repo overview
```

---

## Initial Setup

### 1. Create GitHub Repository

```bash
# On GitHub:
- Repository name: infra-composer-cli
- Description: Portable CLI for composing Terraform stacks from catalogs
- Visibility: Public
- Initialize with: None (we'll push existing repo)
- License: Apache 2.0 or MIT (choose one)

# Locally:
git clone https://github.com/tiziano093/infra-composer-cli.git
cd infra-composer-cli
```

### 2. Initialize Go Module

```bash
go mod init github.com/tiziano093/infra-composer-cli

# Add dependencies (Phase 1)
go get github.com/spf13/cobra@v1.7.0
go get github.com/spf13/viper@v1.16.0
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/stretchr/testify@v1.8.4

# Tidy
go mod tidy
```

### 3. Create Initial Directory Structure

```bash
# Core directories
mkdir -p cmd/infra-composer/{_docs}
mkdir -p internal/{cli,commands,catalog,terraform,config,git,output}
mkdir -p pkg/{terraform,catalog}
mkdir -p test/{unit,integration,fixtures/{schemas,modules,requests,expected-outputs}}
mkdir -p scripts
mkdir -p build
mkdir -p docs
mkdir -p examples/outputs
mkdir -p homebrew
mkdir -p .github/workflows
mkdir -p .github/issue_templates
```

### 4. Create Essential Files

#### `.gitignore`
```
# Build artifacts
/bin/
/build/
*.exe
*.dll
*.dylib

# IDE
.idea/
.vscode/
*.swp
*.swo
*~
.DS_Store

# Go
vendor/
go.sum

# Testing
*.out
*.test

# Config
~/.infra-composer/
.env.local

# OS
.DS_Store
Thumbs.db
```

#### `Makefile`
```makefile
.PHONY: help build test lint fmt clean release docker

BINARY_NAME=infra-composer
VERSION?=v1.0.0-dev
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

help:
	@echo "Available targets:"
	@echo "  make build       - Build CLI binary"
	@echo "  make test        - Run all tests"
	@echo "  make lint        - Run linter"
	@echo "  make fmt         - Format code"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make release     - Build release binaries"
	@echo "  make docker      - Build Docker image"

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/infra-composer

test:
	go test ./... -v -cover

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...
	goimports -w .

clean:
	rm -rf bin/ build/

release:
	./scripts/release.sh

docker:
	docker build -t ghcr.io/tiziano093/$(BINARY_NAME):$(VERSION) .

.DEFAULT_GOAL := help
```

#### `go.mod` (starter)
```
module github.com/tiziano093/infra-composer-cli

go 1.21

require (
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
	gopkg.in/yaml.v3 v3.0.1
	github.com/stretchr/testify v1.8.4
)
```

---

## CI/CD Workflows

### GitHub Actions Setup

#### `.github/workflows/test.yml`
```yaml
name: Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.21', '1.22']
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ matrix.go-version }}
    
    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
    
    - name: Install dependencies
      run: go mod download
    
    - name: Run tests
      run: |
        go test ./... -v -cover -coverprofile=coverage.out
        go tool cover -func=coverage.out
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        flags: unittests
```

#### `.github/workflows/lint.yml`
```yaml
name: Lint

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  lint:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --timeout=5m

    - name: go fmt
      run: |
        if [ -n "$(gofmt -s -l .)" ]; then
          echo "Go files need formatting"
          exit 1
        fi
```

#### `.github/workflows/release.yml`
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v5
      with:
        distribution: goreleaser
        version: latest
        args: release --clean
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

#### `.github/workflows/docker.yml`
```yaml
name: Docker Build & Push

on:
  push:
    tags:
      - 'v*'

jobs:
  docker:
    runs-on: ubuntu-latest
    
    permissions:
      contents: read
      packages: write
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v2
    
    - name: Login to GitHub Container Registry
      uses: docker/login-action@v2
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    
    - name: Build and push
      uses: docker/build-push-action@v4
      with:
        context: .
        push: true
        tags: ghcr.io/tiziano093/infra-composer:${{ github.ref_name }}
```

---

## Linting & Code Quality

### `.golangci.yml`
```yaml
run:
  timeout: 5m
  tests: true

linters:
  enable:
    - goimports
    - gofmt
    - vet
    - staticcheck
    - errcheck
    - ineffassign
    - unused
    - typecheck
  disable:
    - revive

issues:
  exclude-use-default: false
  exclude:
    - "G101:"  # Gosec: Potential hardcoded credentials
    - "G102:"  # Gosec: SQL injection

output:
  format: colored-line-number
```

---

## Branch Strategy

### Main Branches
- **`main`** — Production-ready code, tagged with releases
- **`develop`** — Integration branch for features

### Feature/Fix Branches
- **`feature/xxx`** — New features
- **`fix/xxx`** — Bug fixes
- **`docs/xxx`** — Documentation only
- **`chore/xxx`** — Refactoring, tooling

### Branch Protection Rules
- Require pull request reviews (2 approvals)
- Require status checks to pass (tests, lint, coverage)
- Dismiss stale pull request approvals
- Require branches to be up to date before merging

---

## Pull Request Process

### PR Template (`.github/pull_request_template.md`)
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation

## Related Issues
Fixes #(issue number)

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Tests pass locally
- [ ] Documentation updated
- [ ] No new warnings generated

## Screenshots (if applicable)
```

---

## Versioning & Releases

### Semver Policy
- **MAJOR**: Breaking CLI/config changes
- **MINOR**: New commands, new flags, new features
- **PATCH**: Bug fixes, small improvements

### Release Process
1. Merge PR to `main`
2. Create git tag: `git tag v1.2.3`
3. Push tag: `git push origin v1.2.3`
4. GitHub Actions automatically:
   - Builds cross-platform binaries
   - Creates checksums + signatures
   - Creates GitHub release
   - Pushes Docker image
   - Updates Homebrew formula

---

## Development Machine Setup

### Prerequisites
- Go 1.21+
- Docker (optional, for testing Docker builds)
- git

### Clone & Setup
```bash
git clone https://github.com/tiziano093/infra-composer-cli.git
cd infra-composer-cli

# Install dependencies
go mod download

# Build
make build

# Test
make test

# Run
./bin/infra-composer --version
```

---

## Monitoring & Observability

### Code Coverage
- Target: 80%+ overall
- GitHub Actions uploads to codecov.io
- Coverage badge in README

### Performance Metrics
- CLI startup time: <100ms
- Catalog build time: <30s for AWS provider
- Stack composition: <5s for typical stack
- Measured in CI/CD before releases

---

## Security Practices

### Dependency Management
- Pin all versions in go.mod
- Regular `go get -u` for patches
- Review changelog before minor/major upgrades
- Automated security scanning (if available)

### Access Control
- GitHub teams for permission management
- Branch protection for main/develop
- Signed commits recommended
- Release signatures with GPG

### Secrets Management
- No hardcoded secrets in code/config
- GitHub Secrets for CI/CD (GITHUB_TOKEN)
- Environment-specific secrets in deployment pipeline

---

## Troubleshooting

### Common Issues

**Issue:** Tests fail locally but pass in CI
- **Solution:** Update go.mod/go.sum and rebuild caches

**Issue:** golangci-lint finds issues locally not in CI
- **Solution:** Ensure local version matches CI (check .golangci.yml)

**Issue:** Docker build fails on M1 Mac
- **Solution:** Use `docker buildx` for cross-platform builds

---

## Contact & Escalation

- **Project Owner:** Tiziano (@tiziano093)
- **DevOps Contact:** [Team Lead]
- **Questions:** Open GitHub issue or discussion

---

**Last Updated:** 2026-04-22  
**Next Review:** After Phase 1 CI/CD setup  
