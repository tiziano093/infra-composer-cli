# Installation

## Requirements

- Go 1.21+ (build from source only)
- Terraform >= 1.0 (required for `catalog build --registry-source terraform` and `interactive`)

---

## Option 1 — Pre-built binary (GitHub Releases)

```bash
# Replace <version> and <os>-<arch> with your target:
#   os:   darwin | linux | windows
#   arch: amd64 | arm64

VERSION=v1.0.0
OS=linux
ARCH=amd64

curl -sSL \
  "https://github.com/tiziano093/infra-composer-cli/releases/download/${VERSION}/infra-composer_${VERSION}_${OS}_${ARCH}.tar.gz" \
  | tar -xz -C /usr/local/bin infra-composer

chmod +x /usr/local/bin/infra-composer
infra-composer --version
```

Checksums are published in `checksums.txt` on every release.

---

## Option 2 — Homebrew (macOS / Linux)

```bash
brew tap tiziano093/infra-composer-cli
brew install infra-composer
```

Upgrade:
```bash
brew upgrade infra-composer
```

---

## Option 3 — npm

```bash
npm install -g @tiziano093/infra-composer
infra-composer --version
```

The postinstall script downloads the correct platform binary from GitHub Releases.
Node >= 16 required.

---

## Option 4 — Docker

```bash
docker run --rm ghcr.io/tiziano093/infra-composer:v1.0.0 --version

# Mount local dirs to read/write schemas and stacks:
docker run --rm \
  -v "$PWD/catalog:/catalog" \
  -v "$PWD/infrastructure:/infrastructure" \
  ghcr.io/tiziano093/infra-composer:v1.0.0 \
  catalog build --provider hashicorp/random@3.6.0 --output-dir /catalog
```

---

## Option 5 — Build from source

```bash
git clone https://github.com/tiziano093/infra-composer-cli.git
cd infra-composer-cli
make build
./bin/infra-composer --version
```

Install to PATH:
```bash
sudo cp bin/infra-composer /usr/local/bin/
```

---

## Shell completion

```bash
# Bash
infra-composer completion bash > /etc/bash_completion.d/infra-composer

# Zsh
infra-composer completion zsh > "${fpath[1]}/_infra-composer"

# Fish
infra-composer completion fish > ~/.config/fish/completions/infra-composer.fish
```

---

## Verify installation

```bash
infra-composer --version
# infra-composer v1.0.0, Built: 2026-04-24T00:00:00Z, Commit: abc1234
```
