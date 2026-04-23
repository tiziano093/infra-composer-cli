# infra-composer

> Portable CLI for composing Terraform stacks from provider catalogs.

**Status:** v1.0.0 — all 4 phases complete and ready for release.

## Documentation

| Guide | Description |
|-------|-------------|
| [INSTALL.md](INSTALL.md) | Binary, Homebrew, npm, Docker, source |
| [QUICKSTART.md](QUICKSTART.md) | 5-minute tutorial |
| [CLI.md](CLI.md) | Full command reference |
| [CONFIG.md](CONFIG.md) | Config file, env vars, CI usage |
| [PIPELINE.md](PIPELINE.md) | GitHub Actions & Azure Pipelines examples |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Internal design |
| [docs/REGISTRY.md](docs/REGISTRY.md) | Registry source flags |
| [docs/INTERACTIVE.md](docs/INTERACTIVE.md) | Guided interactive workflow |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | Developer guide |

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
make smoke    # smoke tests against local binary
make bench    # performance benchmarks
make lint     # run golangci-lint
make fmt      # format code
make release  # cross-platform binaries in ./build/
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
