# CI/CD Integration

## GitHub Actions

### Build catalog and compose on push

```yaml
# .github/workflows/infra-compose.yml
name: Infra Compose

on:
  push:
    branches: [main]
    paths:
      - 'infra/**'

jobs:
  compose:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install infra-composer
        run: |
          VERSION=v1.0.0
          curl -sSL \
            "https://github.com/tiziano093/infra-composer-cli/releases/download/${VERSION}/infra-composer_${VERSION}_linux_amd64.tar.gz" \
            | tar -xz -C /usr/local/bin infra-composer

      - name: Set up Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: "1.7.0"
          terraform_wrapper: false

      - name: Build catalog
        run: |
          infra-composer catalog build \
            --registry-source terraform \
            --provider hashicorp/aws@5.50.0 \
            --output-dir ./catalog
        env:
          INFRA_COMPOSER_LOG_FORMAT: json

      - name: Validate catalog
        run: infra-composer catalog validate ./catalog/schema.json

      - name: Compose stack (dry-run)
        run: |
          infra-composer compose \
            --schema ./catalog/schema.json \
            --modules "${{ vars.COMPOSE_MODULES }}" \
            --output-dir ./infrastructure \
            --dry-run --format json

      - name: Compose stack
        run: |
          infra-composer compose \
            --schema ./catalog/schema.json \
            --modules "${{ vars.COMPOSE_MODULES }}" \
            --output-dir ./infrastructure \
            --force

      - name: Commit generated infrastructure
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "chore: regenerate infrastructure stack"
          file_pattern: "infrastructure/**"
```

---

## Azure Pipelines

```yaml
# azure-pipelines-infra.yml
trigger:
  branches:
    include: [main]
  paths:
    include: [infra/*]

pool:
  vmImage: ubuntu-latest

variables:
  INFRA_COMPOSER_VERSION: v1.0.0
  INFRA_COMPOSER_LOG_FORMAT: json

steps:
  - script: |
      curl -sSL \
        "https://github.com/tiziano093/infra-composer-cli/releases/download/$(INFRA_COMPOSER_VERSION)/infra-composer_$(INFRA_COMPOSER_VERSION)_linux_amd64.tar.gz" \
        | tar -xz -C /usr/local/bin infra-composer
    displayName: Install infra-composer

  - task: TerraformInstaller@1
    inputs:
      terraformVersion: 1.7.0

  - script: |
      infra-composer catalog build \
        --registry-source terraform \
        --provider hashicorp/azurerm@3.100.0 \
        --output-dir ./catalog
    displayName: Build catalog

  - script: |
      infra-composer catalog validate ./catalog/schema.json
    displayName: Validate catalog

  - script: |
      infra-composer compose \
        --schema ./catalog/schema.json \
        --modules "$(COMPOSE_MODULES)" \
        --output-dir ./infrastructure \
        --force
    displayName: Compose stack

  - task: PublishPipelineArtifact@1
    inputs:
      targetPath: infrastructure
      artifact: terraform-stack
```

---

## Caching the provider schema

Provider schema downloads can be slow. Cache the `--cache-dir` between runs.

### GitHub Actions cache

```yaml
- name: Cache provider schemas
  uses: actions/cache@v4
  with:
    path: ~/.cache/infra-composer
    key: infra-composer-${{ runner.os }}-${{ hashFiles('infra/providers.lock') }}
    restore-keys: infra-composer-${{ runner.os }}-
```

### Azure Pipelines cache

```yaml
- task: Cache@2
  inputs:
    key: '"infra-composer" | "$(Agent.OS)" | infra/providers.lock'
    path: $(HOME)/.cache/infra-composer
    restoreKeys: '"infra-composer" | "$(Agent.OS)"'
```

---

## Validate-only mode (no Terraform required)

If you only need to validate or search an existing `schema.json` (no registry fetch),
Terraform is not required:

```bash
# No terraform needed for these commands:
infra-composer catalog validate ./catalog/schema.json
infra-composer search --schema ./catalog/schema.json vpc
infra-composer interface --schema ./catalog/schema.json aws_vpc
infra-composer compose --schema ./catalog/schema.json --modules aws_vpc --dry-run
```

Use `--registry-source fake` and commit the fixture dir to avoid Terraform in CI
when the schema is stable.
