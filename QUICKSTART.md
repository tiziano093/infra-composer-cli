# Quickstart — 5 minutes

This guide takes you from zero to a composed Terraform stack.

## Prerequisites

- `infra-composer` installed (see [INSTALL.md](INSTALL.md))
- `terraform` >= 1.0 on PATH

---

## Path A — Interactive (recommended for first use)

The guided workflow handles everything: provider selection, version pinning, resource picking, catalog build, and optional compose.

```bash
infra-composer interactive --output-dir ./catalog
```

Follow the prompts:
1. Choose a provider (e.g. `hashicorp/random`)
2. Select the version (defaults to latest)
3. Multi-select the resources you want
4. Confirm compose to generate a Terraform stack

Done. Your stack lands in `./infrastructure/`.

---

## Path B — Scriptable

### 1. Build a catalog

```bash
infra-composer catalog build \
  --registry-source terraform \
  --provider hashicorp/random@3.6.0 \
  --output-dir ./catalog
```

Output:
```
Building catalog for hashicorp/random@3.6.0
Provider: hashicorp/random, 6 modules
Exported to ./catalog/schema.json
```

### 2. Search for modules

```bash
infra-composer search --schema ./catalog/schema.json string
```

Output:
```
Name             Type      Group    Description
───────────────────────────────────────────────────────
random_string    resource  random   Random string generator
random_password  resource  random   Random password generator
```

### 3. Inspect module interface

```bash
infra-composer interface --schema ./catalog/schema.json random_string
```

### 4. Compose a Terraform stack

```bash
infra-composer compose \
  --schema ./catalog/schema.json \
  --modules "random_string random_pet" \
  --output-dir ./infrastructure
```

Generated structure:
```
infrastructure/
├── random_string/
│   ├── version.tf
│   ├── variables.tf
│   ├── main.tf
│   ├── outputs.tf
│   └── README.md
├── random_pet/
│   ├── version.tf
│   ├── variables.tf
│   ├── main.tf
│   ├── outputs.tf
│   └── README.md
└── modules.json
```

Each folder is a self-contained Terraform module (one resource or
data source). Wire them into your own environment stack — the CLI
does not generate an opinionated root module.

### 5. Discover modules via `modules.json`

The manifest at the root of `--output-dir` summarises every composed
module: path, provider, variables (with types, defaults, references),
and outputs. Inspect it with `jq`:

```bash
jq '.modules[] | {path, name: .entry.name, required: [.entry.variables[] | select(.required) | .name]}' \
   infrastructure/modules.json
```

### 6. Use a module in your environment

```hcl
module "pet" {
  source = "../infrastructure/random_pet"
  length = 2
}
```

Run Terraform against the module folder you picked:

```bash
cd infrastructure/random_pet
terraform init
terraform plan
```

---

## Dry run

Preview without writing files:

```bash
infra-composer compose \
  --schema ./catalog/schema.json \
  --modules "random_string random_pet" \
  --output-dir ./infrastructure \
  --dry-run
```

---

## Next steps

- [CLI reference](CLI.md) — all commands and flags
- [Configuration](CONFIG.md) — config file and env vars
- [CI/CD integration](PIPELINE.md) — GitHub Actions and Azure Pipelines examples
