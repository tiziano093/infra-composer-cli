package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

const composeSchemaJSON = `{
  "schema_version": "1.0",
  "provider": "hashicorp/aws",
  "provider_version": "5.42.0",
  "modules": [
    {"name": "aws_vpc", "type": "resource",
     "source": "git::https://example.com/aws-vpc.git",
     "variables": [{"name": "cidr_block", "type": "string", "required": true}],
     "outputs": [{"name": "id"}]},
    {"name": "aws_subnet", "type": "resource",
     "variables": [
       {"name": "vpc_id", "type": "string", "required": true,
        "references": [{"module": "aws_vpc", "output": "id"}]},
       {"name": "cidr_block", "type": "string", "required": true}],
     "outputs": [{"name": "id"}]}
  ]
}`

func runCompose(t *testing.T, schemaBody string, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	path := writeSchema(t, schemaBody)
	stdout := &bytes.Buffer{}
	cmd := NewComposeCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs(append([]string{"--schema", path}, args...))
	return stdout, cmd.Execute()
}

func TestCompose_DryRunJSON(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc,aws_subnet", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	assert.True(t, summary.DryRun)
	assert.Equal(t, []string{"aws_vpc", "aws_subnet"}, summary.Modules)
	require.Len(t, summary.Files, 5)
	for _, f := range summary.Files {
		assert.Len(t, f.SHA256, 64, "expected hex sha256")
		assert.Greater(t, f.Bytes, 0)
	}
}

func TestCompose_WritesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc aws_subnet", "--output-dir", dir)
	require.NoError(t, err)
	for _, name := range []string{"providers.tf", "variables.tf", "locals.tf", "main.tf", "outputs.tf"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "missing %s", name)
		require.NotEmpty(t, body)
	}
	main, _ := os.ReadFile(filepath.Join(dir, "main.tf"))
	assert.Contains(t, string(main), `module "this_aws_vpc"`)
	assert.Contains(t, string(main), "module.this_aws_vpc.id")
}

func TestCompose_RefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# pre-existing\n"), 0o644))
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc", "--output-dir", dir)
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Contains(t, ce.Error(), "already contains generated files")
	body, _ := os.ReadFile(filepath.Join(dir, "main.tf"))
	assert.Equal(t, "# pre-existing\n", string(body), "no file must be overwritten without --force")
}

func TestCompose_ForceOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# stale"), 0o644))
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc", "--output-dir", dir, "--force")
	require.NoError(t, err)
	body, _ := os.ReadFile(filepath.Join(dir, "main.tf"))
	assert.Contains(t, string(body), `module "this_aws_vpc"`)
}

func TestCompose_DryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc", "--output-dir", dir, "--dry-run")
	require.NoError(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "dry-run must not touch the filesystem")
}

func TestCompose_NoModules(t *testing.T) {
	t.Parallel()
	_, err := runCompose(t, composeSchemaJSON, "--dry-run")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCompose_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := runCompose(t, composeSchemaJSON, "--modules", "nope", "--dry-run")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitModuleNotFound, ce.Code)
}

func TestCompose_PartialSelectionWarnsInJSON(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_subnet", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	require.NotEmpty(t, summary.Warnings)
	hasRefWarning := false
	for _, w := range summary.Warnings {
		if strings.Contains(w, "aws_subnet.vpc_id references aws_vpc.id") {
			hasRefWarning = true
		}
	}
	assert.True(t, hasRefWarning, "expected partial-selection warning, got %v", summary.Warnings)
}
