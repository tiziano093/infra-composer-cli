# infra-composer

> Portable CLI for composing Terraform stacks from provider catalogs.

**Status:** 🚧 Phase 1 (Foundation) complete — Cobra CLI, Viper config,
slog logging, error/exit-code framework and `version` subcommand are in
place. Catalog/compose commands land in Phase 2+.

## Quick links

- [Project Overview](docs/PROJECT_OVERVIEW.md)
- [Architecture](docs/ARCHITECTURE.md)
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
