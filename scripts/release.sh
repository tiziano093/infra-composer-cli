#!/usr/bin/env bash
# Cross-platform release build. Produces binaries in ./build/ with checksums.
set -euo pipefail

BINARY="infra-composer"
VERSION="${VERSION:-v0.1.0-dev}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
OUT="build"

rm -rf "$OUT"
mkdir -p "$OUT"

LDFLAGS="-s -w \
  -X main.Version=${VERSION} \
  -X main.BuildTime=${BUILD_TIME} \
  -X main.GitCommit=${GIT_COMMIT}"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

for platform in "${PLATFORMS[@]}"; do
  goos="${platform%/*}"
  goarch="${platform#*/}"
  ext=""
  [[ "$goos" == "windows" ]] && ext=".exe"
  out="${OUT}/${BINARY}-${goos}-${goarch}${ext}"
  echo "==> building $out"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -ldflags "$LDFLAGS" -o "$out" ./cmd/infra-composer
done

(cd "$OUT" && sha256sum * > checksums.txt)
echo "==> done. artifacts in ./$OUT/"
