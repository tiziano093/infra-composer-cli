#!/usr/bin/env bash
# Smoke test: verify the installed binary covers the critical paths.
# Exits 0 if all checks pass, 1 on first failure.
set -euo pipefail

BIN="${INFRA_COMPOSER_BIN:-./bin/infra-composer}"
FIXTURES="$(dirname "$0")/../test/fixtures"

pass() { printf '  \033[32m✓\033[0m %s\n' "$*"; }
fail() { printf '  \033[31m✗\033[0m %s\n' "$*"; exit 1; }

echo "==> smoke tests: $BIN"

# 1. Binary runs.
"$BIN" --version > /dev/null 2>&1 && pass "--version" || fail "--version exited non-zero"

# 2. Help exits 0.
"$BIN" --help > /dev/null 2>&1 && pass "--help" || fail "--help exited non-zero"

# 3. version --format json.
out=$("$BIN" version --format json 2>/dev/null)
echo "$out" | grep -q '"version"' && pass "version --format json" || fail "version --format json: missing 'version' key"

# 4. Unknown flag exits 2.
"$BIN" --unknown-flag > /dev/null 2>&1 || code=$?
[ "${code:-0}" -eq 2 ] && pass "unknown flag → exit 2" || fail "unknown flag exited ${code:-0}, want 2"

# 5. catalog build from fake fixture registry.
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
"$BIN" catalog build \
  --provider hashicorp/aws \
  --registry-source fake \
  --registry-dir "$FIXTURES/registry" \
  --output-dir "$tmpdir" > /dev/null 2>&1 && pass "catalog build (fake source)" || fail "catalog build failed"

# 6. Schema file written.
[ -f "$tmpdir/schema.json" ] && pass "schema.json exists" || fail "schema.json not written"

# 7. catalog validate.
"$BIN" catalog validate "$tmpdir/schema.json" > /dev/null 2>&1 && pass "catalog validate" || fail "catalog validate failed"

# 8. search returns results.
out=$("$BIN" search --schema "$tmpdir/schema.json" --format json 2>/dev/null)
count=$(echo "$out" | grep -c '"name"' || true)
[ "$count" -gt 0 ] && pass "search returns $count module(s)" || fail "search returned 0 results"

# 9. compose --dry-run.
first_module=$(echo "$out" | grep '"name"' | head -1 | sed 's/.*: "\([^"]*\)".*/\1/')
"$BIN" compose \
  --schema "$tmpdir/schema.json" \
  --modules "$first_module" \
  --output-dir "$tmpdir/stack" \
  --dry-run > /dev/null 2>&1 && pass "compose --dry-run" || fail "compose --dry-run failed"

# 10. Startup time < 500ms.
start=$(date +%s%3N)
"$BIN" --version > /dev/null 2>&1
end=$(date +%s%3N)
elapsed=$(( end - start ))
[ "$elapsed" -lt 500 ] && pass "startup ${elapsed}ms < 500ms" || fail "startup ${elapsed}ms ≥ 500ms"

echo ""
echo "==> all smoke tests passed"
