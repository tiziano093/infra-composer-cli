// Package integration runs end-to-end CLI flows against the on-disk
// fake registry fixtures shipped under test/fixtures/registry. It
// exercises the public CLI surface (Cobra commands wired up by the cli
// root) so changes in any layer surface here.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/cli"
)

func registryFixturesDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(here), "..", "fixtures", "registry")
}

// runCLI executes the CLI as if invoked from the shell, returning the
// captured stdout, stderr and process exit code so assertions can lock
// down the public contract.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, code cli.ExitCode) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	c := cli.Execute(context.Background(), cli.BuildInfo{Version: "test"}, args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), c
}

func TestE2E_BuildExportValidateSearchList(t *testing.T) {
	t.Parallel()
	regDir := registryFixturesDir(t)
	stackDir := t.TempDir()

	// 1. Build a fresh schema from the fake registry.
	stdout, _, code := runCLI(t,
		"catalog", "build",
		"--provider", "hashicorp/aws",
		"--output-dir", stackDir,
		"--registry-dir", regDir,
		"--format", "json",
	)
	require.Equal(t, cli.ExitSuccess, code, "build failed: %s", stdout)
	var buildReport map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &buildReport))
	require.EqualValues(t, 3, buildReport["modules"])
	schemaPath := filepath.Join(stackDir, "schema.json")
	require.FileExists(t, schemaPath)

	// 2. Validate the produced schema.
	stdout, _, code = runCLI(t, "catalog", "validate", schemaPath)
	require.Equal(t, cli.ExitSuccess, code, stdout)
	assert.Contains(t, stdout, "OK")
	assert.Contains(t, stdout, "hashicorp/aws@5.42.0")

	// 3. List entries (table) and confirm both kinds are present.
	stdout, _, code = runCLI(t, "catalog", "list", schemaPath)
	require.Equal(t, cli.ExitSuccess, code, stdout)
	assert.Contains(t, stdout, "aws_vpc")
	assert.Contains(t, stdout, "aws_caller_identity")

	// 4. Search by keyword (AND logic).
	stdout, _, code = runCLI(t, "search", "--schema", schemaPath, "vpc")
	require.Equal(t, cli.ExitSuccess, code, stdout)
	assert.Contains(t, stdout, "aws_vpc")
	assert.NotContains(t, stdout, "aws_caller_identity")

	// 5. Re-export to a sibling location and check it round-trips.
	exportDest := filepath.Join(t.TempDir(), "copy", "schema.json")
	stdout, _, code = runCLI(t, "catalog", "export", schemaPath, "--output", exportDest, "--format", "json")
	require.Equal(t, cli.ExitSuccess, code, stdout)
	assert.FileExists(t, exportDest)

	stdout, _, code = runCLI(t, "catalog", "validate", exportDest)
	require.Equal(t, cli.ExitSuccess, code, stdout)
	assert.Contains(t, stdout, "OK")
}

func TestE2E_BuildProviderNotFound_ExitCode(t *testing.T) {
	t.Parallel()
	_, stderr, code := runCLI(t,
		"catalog", "build",
		"--provider", "hashicorp/missing",
		"--output-dir", t.TempDir(),
		"--registry-dir", registryFixturesDir(t),
	)
	assert.Equal(t, cli.ExitFileNotFound, code)
	assert.Contains(t, stderr, "not found")
}

func TestE2E_ListMissingSchema_ExitCode(t *testing.T) {
	t.Parallel()
	_, stderr, code := runCLI(t,
		"catalog", "list",
		filepath.Join(t.TempDir(), "nope.json"),
	)
	assert.Equal(t, cli.ExitFileNotFound, code)
	assert.Contains(t, stderr, "not found")
}
