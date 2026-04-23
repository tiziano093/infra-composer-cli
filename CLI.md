# CLI Reference

## Global flags

These flags are accepted by every command.

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.infra-composer/config.yaml` | Path to YAML config file |
| `--format` / `-f` | `table` | Output format: `table\|json\|yaml` |
| `--log-level` | `info` | `debug\|info\|warn\|error` |
| `--log-format` | `text` | `text\|json` |
| `--verbose` / `-v` | false | Sets log-level to `debug` |
| `--quiet` / `-q` | false | Suppress non-error logs |

---

## `version`

Print version information.

```bash
infra-composer version [--format text|json]
```

Example:
```bash
infra-composer version --format json
# {"version":"v1.0.0","build_time":"2026-04-24T00:00:00Z","git_commit":"abc1234"}
```

---

## `interactive`

Guided workflow: pick provider → version → resources → build catalog → optional compose.

Requires `terraform` >= 1.0 on PATH.

```bash
infra-composer interactive --output-dir <dir> [flags]
```

| Flag | Description |
|------|-------------|
| `--output-dir` | **Required.** Where to write `schema.json` |
| `--compose-dir` | After building, run `compose --output-dir <dir>` |
| `--no-compose-prompt` | Skip the post-build compose prompt |
| `--cache-dir` | Override the schema cache directory |
| `--terraform-binary` | Path to `terraform` binary (default: PATH lookup) |

---

## `catalog`

Manage catalog schemas.

### `catalog build`

Build a catalog schema from a Terraform provider.

```bash
infra-composer catalog build \
  --provider <addr> \
  --output-dir <dir> \
  [--registry-source terraform|http|file] \
  [--registry-dir <dir>] \
  [--include <patterns>] \
  [--exclude <patterns>] \
  [--format text|json]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | | Provider address, e.g. `hashicorp/aws` or `hashicorp/aws@5.0.0` |
| `--output-dir` | | Directory for the generated `schema.json` |
| `--registry-source` | `terraform` | Source: `terraform` (shells out to `terraform providers schema`), `http` (Terraform Registry API), `fake` (local fixture dir) |
| `--registry-dir` | `./catalog/registry` | Fixture dir (for `file` source) |
| `--include` | | Comma-separated resource name patterns to include |
| `--exclude` | | Comma-separated resource name patterns to exclude |

### `catalog validate`

Validate a schema and report all issues.

```bash
infra-composer catalog validate [path] [--format text|json]
```

Exit codes: `0` OK, `4` validation failed.

### `catalog list`

List all modules in a schema.

```bash
infra-composer catalog list [path] [--group <group>] [--format table|json]
```

### `catalog export`

Re-serialise a schema to a new location.

```bash
infra-composer catalog export [path] --output <file|dir> [--format text|json]
```

---

## `search`

Search modules by keyword (AND logic).

```bash
infra-composer search [keywords...] \
  [--schema <path>] \
  [--group <group>] \
  [--type resource|data] \
  [--limit <n>] \
  [--format table|json]
```

Ranking: exact name > substring > group > description > fuzzy subsequence.

---

## `dependencies`

Show the dependency tree of a module.

```bash
infra-composer dependencies <module> \
  [--schema <path>] \
  [--depth <n>] \
  [--check-cycles] \
  [--format text|json]
```

| Flag | Description |
|------|-------------|
| `--depth` | Max tree depth (0 = unlimited) |
| `--check-cycles` | Exit with `ExitDependencyFailed` if any cycle exists |

---

## `interface`

Show inputs and outputs of one or more modules.

```bash
infra-composer interface <modules...> \
  [--schema <path>] \
  [--required-only] \
  [--full] \
  [--format text|json|yaml]
```

| Flag | Description |
|------|-------------|
| `--required-only` | Show only required inputs (drops optional and auto-wired) |
| `--full` | Include auto-wired inputs in the aggregate view |

---

## `compose`

Generate Terraform module folders from catalog entries.

```bash
infra-composer compose \
  --schema <path> \
  --modules "<module> [<module>...]" \
  --output-dir <dir> \
  [--dry-run] \
  [--force] \
  [--format text|json]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--modules` | | Space- or comma-separated module names. Qualify with `resource.` or `data.` to disambiguate |
| `--filter` | | Glob pattern(s) to select modules (e.g. `aws_s3*`) |
| `--all` | false | Select every module in the catalog |
| `--output-dir` | | Parent directory for generated module folders |
| `--dry-run` | false | Preview planned files without writing |
| `--force` | false | Overwrite pre-existing generated files |

Alongside the per-module folders the command writes `modules.json` at
the root of `--output-dir`: a machine-readable manifest describing
every composed module (folder path, provider, variables, outputs and
cross-module references). Consumers use it to wire the modules into
their own environment without reading each folder individually.

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Internal error |
| `2` | Invalid arguments |
| `5` | Module not found in catalog |
| `6` | Dependency cycle or ambiguous reference |

---

## Exit codes (all commands)

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | — | Success |
| 1 | `ExitError` | General / internal error |
| 2 | `ExitInvalidArgs` | Bad flags or arguments |
| 3 | `ExitFileNotFound` | Schema or module path not found |
| 4 | `ExitValidationFailed` | Schema validation issues |
| 5 | `ExitModuleNotFound` | Module not in catalog |
| 6 | `ExitDependencyFailed` | Cycle or ambiguous reference |
