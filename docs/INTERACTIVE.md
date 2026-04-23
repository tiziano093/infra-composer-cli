# Interactive mode

`infra-composer interactive` is the recommended entry point when you
want to **explore a provider and pick only the resources you actually
need** without materialising a multi-megabyte catalog.

It is a thin wrapper around the `terraform` registry source plus the
`compose` pipeline, with a guided UI built on
[AlecAivazis/survey](https://github.com/AlecAivazis/survey).

## Requirements

- `terraform` binary (>= 1.0) available on `PATH` (or via
  `--terraform-binary`)
- Network access to `registry.terraform.io` (only on a cache miss)

## Usage

```bash
infra-composer interactive --output-dir ./catalog
```

You will be prompted, in order:

1. **Provider** — pick from a preset list (hashicorp/random, aws,
   azurerm, google, kubernetes, null, local) or `Other` for a freeform
   `<namespace>/<name>` input.
2. **Version** — `latest` (default) or any specific version published
   on the registry.
3. The CLI then downloads the schema (with a spinner-style log line)
   and caches it under `--cache-dir`.
4. **Resources** — multi-select with incremental search; each entry is
   labelled `<name>  [resource|data]`.
5. The catalog is written to `--output-dir/schema.json`.
6. **Compose now?** — opt-in follow-up that runs `compose
   --output-dir <dir>` against the just-built catalog and your selection.

## Flags

| Flag                       | Purpose                                                     |
| -------------------------- | ----------------------------------------------------------- |
| `--output-dir` (required)  | Where the filtered catalog is written                       |
| `--cache-dir`              | Override cache root (default `$XDG_CACHE_HOME/infra-composer`) |
| `--terraform-binary`       | Path to a specific `terraform` binary                       |
| `--compose-dir`            | Default directory used in the compose follow-up prompt      |
| `--no-compose-prompt`      | Skip the post-build compose step entirely                   |

## Non-interactive equivalent

For scripting/CI, the same outcome can be achieved with the standard
`catalog build` command:

```bash
infra-composer catalog build \
  --registry-source terraform \
  --provider hashicorp/random@3.6.0 \
  --include "random_id,random_string" \
  --output-dir ./catalog
```
