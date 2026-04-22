# Infra-Composer CLI — Project Overview

**Version:** 1.0 (Phase 1 — Scaffolding Complete)  
**Last Updated:** 2026-04-22  
**Status:** Phase 1 in progress — repo scaffold complete, CLI implementation pending  
**PM:** Tiziano  
**Team:** Architecture + Development + DevOps + QA

---

## Executive Summary

**Infra-Composer CLI** is a standalone, portable Go command-line tool designed to transform infrastructure-as-code workflows by automating the composition of Terraform consumer stacks from a modular catalog.

### Problem Statement

Currently, infrastructure engineers face multiple friction points:
- **Hardcoded Paths:** Scripts reference `/Users/tiziano/.claude/skills/infra-composer/...` — not portable across machines/OS
- **Fragmented Workflow:** 7 manual steps (parse → search → resolve → interface → generate → support → review) scattered across separate bash scripts
- **Poor Pipeline Integration:** No `--format json`, `--dry-run`, or structured exit codes — difficult to embed in CI/CD
- **Catalog Onboarding:** Phase 0 (catalog build) requires external dependencies and multiple manual steps
- **Distribution Overhead:** Copying scripts locally, no version management, bash compatibility issues

### Solution: infra-composer-cli

A **single, unified CLI tool** that:
✅ **Solves portability:** Embedded Go binary, zero hardcoded paths, works macOS/Linux/Windows/WSL  
✅ **Enables pipelines:** JSON output, dry-run, structured errors, exit codes  
✅ **Simplifies catalog:** Embedded pipeline, `catalog build` handles everything  
✅ **Improves UX:** Single entry point, structured `--help`, man pages, auto-suggestions  
✅ **Distributes easily:** Binary download, Homebrew, npm, Docker  

---

## Vision & Mission

### Vision
Enable infrastructure teams to compose production-ready Terraform stacks from modular, reusable catalogs without leaving the command line, on any OS, from any machine.

### Mission
Build a lightweight, portable CLI that reduces friction in multi-step infrastructure composition workflows and integrates seamlessly into modern CI/CD pipelines.

---

## Goals & Objectives

### Primary Goals

| Goal | Success Criteria |
|------|------------------|
| **Eliminate Portability Barriers** | Single binary works on macOS/Linux/Windows/WSL without setup |
| **Enable Pipeline Integration** | CLI can be embedded into GitHub Actions, Azure Pipelines, and other CI/CD with JSON output and exit codes |
| **Simplify Catalog Onboarding** | `infra-composer catalog build --provider aws` takes 30sec, no external dependencies |
| **Improve Discoverability** | `--help`, man pages, error suggestions make the CLI self-documenting |
| **Reduce Token Usage** | `--required-only --format yaml` outputs optimized for LLM integration |

### Secondary Goals

- Maintain backward compatibility with the existing Claude skill
- Provide multiple distribution methods (binary, npm, Homebrew, Docker)
- Achieve 80%+ test coverage (unit + integration)
- Ensure sub-100ms CLI startup time
- Cross-platform CI/CD automation (macOS, Linux, Windows)

---

## Scope Definition

### In Scope (MVP — Phase 1-4)

#### Phase 1: Foundation (Weeks 1-2)
- Go project scaffold + module structure
- CLI framework (Cobra) + root command
- Config system (env vars + YAML)
- Logging + error handling
- Basic unit tests
- Makefile + build scripts

#### Phase 2: Catalog Operations (Weeks 3-4)
- `catalog build|export|list|validate|info` commands
- Terraform provider discovery + crawling
- Schema parsing + normalization
- Module search + filtering
- Integration tests with mocked registry

#### Phase 3: Module Composition (Weeks 5-6)
- `dependencies`, `interface`, `compose` commands
- Dependency graph resolution + cycle detection
- HCL template generation (providers.tf, main.tf, etc.)
- Support file generation (CI/CD, tflint, terraform-docs)
- End-to-end integration tests
- Git integration (remote detection, semver tags)

#### Phase 4: Distribution & Polish (Weeks 7-8)
- Cross-platform builds (macOS, Linux, Windows)
- GitHub Actions CI/CD workflows
- Docker image + ghcr.io push
- Homebrew formula + auto-update
- npm package wrapper
- Full documentation + examples
- 1.0.0 release

### Out of Scope (Future)

- Natural Language Processing (NLP) for high-level requests ("I want a VPC")
- Support for non-Terraform IaC (CloudFormation, Pulumi, etc.)
- Web UI or GUI
- Real-time catalog synchronization
- Multi-provider federation (combining AWS + Azure in one stack)

---

## Key Capabilities

### 1. **Catalog Management**
```bash
infra-composer catalog build --provider hashicorp/aws
infra-composer catalog list
infra-composer catalog validate --schema ./modules/schema.json
```

### 2. **Module Discovery**
```bash
infra-composer search network subnet
infra-composer search --group compute
```

### 3. **Dependency Resolution**
```bash
infra-composer dependencies aws_instance --depth 2
infra-composer dependencies aws_instance --check-cycles
```

### 4. **Interface Extraction**
```bash
infra-composer interface aws_vpc --required-only --format yaml
```

### 5. **Stack Composition** (Core Value)
```bash
infra-composer compose \
  --schema ./modules/schema.json \
  --modules-dir ./modules \
  --modules "aws_vpc aws_subnet aws_instance" \
  --output-dir ./stack \
  --dry-run --format json
```

---

## Target Users

| User Type | Use Case |
|-----------|----------|
| **Terraform Module Developers** | Build catalogs, publish to registries, maintain module standards |
| **Infrastructure Engineers** | Compose stacks from catalogs, integrate into pipelines, maintain IaC |
| **DevOps/SRE** | Embed CLI into CI/CD, automate stack generation, reduce manual steps |
| **Platform Teams** | Distribute as internal tool (npm, Docker, binary), ensure consistent deployments |

---

## Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Language** | Go 1.21+ | Fast startup, cross-platform builds, single binary |
| **CLI Framework** | Cobra | Rich command/flag parsing, popular in Go ecosystem |
| **Configuration** | YAML + Env Vars | Human-readable, flexible, portable |
| **Testing** | Go testing + testify | Built-in, powerful, good mocking support |
| **CI/CD** | GitHub Actions | Free tier, cross-platform, tight integration |
| **Container** | Docker (Alpine) | Lightweight, easy distribution |
| **Package Managers** | Homebrew, npm | Easy distribution across platforms |

---

## Next Steps

1. **Architecture Review** — Team aligns on system design
2. **Repository Setup** — Create GitHub repo, CI/CD workflows
3. **Development Guidelines** — Finalize coding standards
4. **Phase 1 Kickoff** — Foundation team starts implementation

---

**Last Reviewed:** 2026-04-22  
**Next Review:** After Phase 1 completion  
