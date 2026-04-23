# Configuration

## Priority order (highest wins)

1. CLI flags
2. Environment variables (`INFRA_COMPOSER_*`)
3. Config file (`~/.infra-composer/config.yaml`)
4. Built-in defaults

---

## Config file

Default location: `~/.infra-composer/config.yaml`

Override: `infra-composer --config /path/to/config.yaml`

### Full example

```yaml
# ~/.infra-composer/config.yaml

# Logging
log_level: info       # debug | info | warn | error
log_format: text      # text | json

# Output
format: table         # table | json | yaml

# Catalog defaults
catalog:
  schema: ./catalog/schema.json   # default --schema path
  registry_source: terraform      # terraform | http | fake
  registry_dir: ./catalog/registry
  cache_dir: ""                   # defaults to $XDG_CACHE_HOME/infra-composer

# Terraform binary
terraform_binary: terraform       # path or name on PATH
```

---

## Environment variables

Every config key maps to `INFRA_COMPOSER_<KEY>` (uppercase, dots → underscores).

| Variable | Equivalent flag | Description |
|----------|----------------|-------------|
| `INFRA_COMPOSER_LOG_LEVEL` | `--log-level` | `debug\|info\|warn\|error` |
| `INFRA_COMPOSER_LOG_FORMAT` | `--log-format` | `text\|json` |
| `INFRA_COMPOSER_FORMAT` | `--format` | `table\|json\|yaml` |
| `INFRA_COMPOSER_CATALOG_SCHEMA` | `--schema` | Default schema path |
| `INFRA_COMPOSER_CATALOG_REGISTRY_SOURCE` | `--registry-source` | Registry source |
| `INFRA_COMPOSER_CATALOG_REGISTRY_DIR` | `--registry-dir` | Fixture dir |
| `INFRA_COMPOSER_CATALOG_CACHE_DIR` | `--cache-dir` | Cache directory |
| `INFRA_COMPOSER_TERRAFORM_BINARY` | `--terraform-binary` | Terraform binary |

---

## CI / non-interactive usage

```bash
# Structured JSON output + no interactive prompts
export INFRA_COMPOSER_FORMAT=json
export INFRA_COMPOSER_LOG_FORMAT=json
export INFRA_COMPOSER_LOG_LEVEL=warn

infra-composer catalog build \
  --provider hashicorp/aws@5.50.0 \
  --output-dir ./catalog
```

---

## Schema caching

`catalog build --registry-source terraform` invokes `terraform providers schema -json`
and caches the raw output under:

```
$XDG_CACHE_HOME/infra-composer/<provider>/<version>/schema.json
```

On Linux: `~/.cache/infra-composer/`
On macOS: `~/Library/Caches/infra-composer/`

Override the root with `INFRA_COMPOSER_CATALOG_CACHE_DIR` or `--cache-dir`.
