# infra-composer

> Portable CLI for composing Terraform stacks from provider catalogs.

**Status:** 🚧 Scaffolding — Phase 1 (Foundation) in progress. No functional commands yet.

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
```

## Development

```bash
make help     # list available targets
make test     # run tests with coverage
make lint     # run golangci-lint
make fmt      # format code
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
