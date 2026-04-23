# Registry sources

Infra-Composer can build a catalog from two backends, selectable via
the `--registry-source` flag on `catalog build` (and used implicitly
by `interactive`):

| Source       | Default | What it does                                          | Requires terraform binary |
| ------------ | :-----: | ----------------------------------------------------- | :-----------------------: |
| `fake`       |   ✓     | Loads schemas from JSON fixtures on disk              | no                        |
| `terraform`  |         | Shells out to `terraform providers schema -json`      | **yes** (>= 1.0)          |

## `fake` (default)

Used in unit tests and to compose stacks from hand-crafted catalogs.
Reads files from the directory passed via `--registry-dir`.

```bash
infra-composer catalog build \
  --registry-source fake \
  --registry-dir ./test/fixtures \
  --provider hashicorp/random \
  --output-dir ./catalog
```

## `terraform` (real registry)

Pulls the provider schema directly from the upstream registry by
delegating to the `terraform` binary. The first run downloads the
provider plugin (cached under `--cache-dir/plugins`); subsequent runs
read from disk.

```bash
infra-composer catalog build \
  --registry-source terraform \
  --provider hashicorp/random@3.6.0 \
  --output-dir ./catalog \
  --cache-dir ~/.cache/infra-composer
```

### Useful flags

| Flag                 | Purpose                                                   |
| -------------------- | --------------------------------------------------------- |
| `--terraform-binary` | Override `$PATH` lookup of `terraform`                    |
| `--cache-dir`        | Root of provider + schema caches (default `$XDG_CACHE_HOME/infra-composer`) |
| `--include`          | glob list (`aws_vpc,aws_subnet*`) of resources to keep    |
| `--exclude`          | glob list applied **after** `--include`                   |

### Filtering

Provider schemas can be huge (AWS exposes 1000+ resources). Use
`--include` / `--exclude` to keep only what you need; both accept comma-
separated [filepath.Match](https://pkg.go.dev/path/filepath#Match)
patterns and are evaluated against the resource name.

```bash
infra-composer catalog build \
  --registry-source terraform \
  --provider hashicorp/aws \
  --include "aws_vpc,aws_subnet*,aws_security_group*" \
  --exclude "*_legacy*" \
  --output-dir ./catalog
```

### Cache layout

```
~/.cache/infra-composer/
├── plugins/      # TF_PLUGIN_CACHE_DIR (managed by terraform itself)
└── schemas/      # JSON dump per <ns>/<name>/<version>.json
```

A schema cache hit lets us skip `terraform init` entirely.

### End-to-end test

A round-trip test against `hashicorp/random` is gated behind
`INFRA_COMPOSER_E2E=1`:

```bash
INFRA_COMPOSER_E2E=1 go test ./test/integration/...
```
