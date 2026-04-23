# infra-composer

> Portable CLI for composing Terraform stacks from provider catalogs.

**Status:** 🚧 Phases 1–3 complete — CLI scaffolding, catalog operations
(build/search/validate/list), module composition (compose/dependencies/
interface) **and** real-registry support via the `terraform` CLI plus a
guided `interactive` workflow are in place. Phase 4 (distribution) next.

## Quick links

- [Project Overview](docs/PROJECT_OVERVIEW.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Registry sources](docs/REGISTRY.md)
- [Interactive mode](docs/INTERACTIVE.md)
- [Roadmap](docs/ROADMAP.md)
- [Repository Setup](docs/REPOSITORY_SETUP.md)
- [Development Guidelines](docs/DEVELOPMENT_GUIDELINES.md)
- [Contributing](docs/CONTRIBUTING.md)

## Build from source

Requires Go 1.21+.

```bash
git clone https://github.com/tiziano093/infra-composer-cli.git
cd infra-composer-cli
make build
./bin/infra-composer --version
./bin/infra-composer version --format json
```

## Try it

```bash
# Guided: pick a provider, version, and resources interactively.
./bin/infra-composer interactive --output-dir ./catalog

# Or scriptable: pull a real provider schema from registry.terraform.io
# (requires `terraform` >= 1.0 on PATH).
./bin/infra-composer catalog build \
  --registry-source terraform \
  --provider hashicorp/random@3.6.0 \
  --output-dir ./catalog
```

See [`docs/REGISTRY.md`](docs/REGISTRY.md) for the registry source flags
(filters, cache layout, custom binary path) and
[`docs/INTERACTIVE.md`](docs/INTERACTIVE.md) for the guided workflow.

Configuration is resolved in this order (highest priority last):
defaults → `~/.infra-composer/config.yaml` → `INFRA_COMPOSER_*`
environment variables → CLI flags. See
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full schema.

## Development

```bash
make help     # list available targets
make test     # run tests with coverage
make lint     # run golangci-lint
make fmt      # format code
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
