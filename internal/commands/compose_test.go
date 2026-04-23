package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
     "variables": [
       {"name": "cidr_block", "type": "string", "required": true},
       {"name": "tags", "type": "map(string)"}
     ],
     "outputs": [{"name": "id"}, {"name": "arn"}]},
    {"name": "aws_subnet", "type": "resource",
     "variables": [
       {"name": "vpc_id", "type": "string", "required": true},
       {"name": "cidr_block", "type": "string", "required": true}
     ],
     "outputs": [{"name": "id"}]},
    {"name": "aws_caller", "type": "data",
     "variables": [{"name": "name", "type": "string", "required": true}],
     "outputs": [{"name": "id"}, {"name": "account_id"}]}
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

func TestCompose_DryRunJSONHasFolderPerModule(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc,aws_subnet", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	assert.True(t, summary.DryRun)
	assert.Equal(t, "hashicorp/aws", summary.Provider)
	require.Len(t, summary.Modules, 2)
	assert.Equal(t, "aws_vpc", summary.Modules[0].Module)
	assert.Equal(t, "aws_vpc", summary.Modules[0].Folder)
	assert.Equal(t, "resource", summary.Modules[0].Kind)
	require.Len(t, summary.Modules[0].Files, 5)
	for _, f := range summary.Modules[0].Files {
		assert.Len(t, f.SHA256, 64)
		assert.Greater(t, f.Bytes, 0)
		assert.Contains(t, f.Path, "aws_vpc/")
	}
	require.Len(t, summary.Modules[1].Files, 5)
}

func TestCompose_WritesFolders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc aws_subnet", "--output-dir", dir)
	require.NoError(t, err)
	for _, mod := range []string{"aws_vpc", "aws_subnet"} {
		for _, f := range []string{"version.tf", "variables.tf", "main.tf", "outputs.tf", "README.md"} {
			body, err := os.ReadFile(filepath.Join(dir, mod, f))
			require.NoError(t, err, "missing %s/%s", mod, f)
			require.NotEmpty(t, body)
		}
	}
	main, _ := os.ReadFile(filepath.Join(dir, "aws_vpc", "main.tf"))
	assert.Contains(t, string(main), `resource "aws_vpc" "this"`)
	assert.Contains(t, string(main), `cidr_block = var.cidr_block`)
}

func TestCompose_DataSourceSelection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "data.aws_caller", "--output-dir", dir)
	require.NoError(t, err)
	main, err := os.ReadFile(filepath.Join(dir, "aws_caller", "main.tf"))
	require.NoError(t, err)
	assert.Contains(t, string(main), `data "aws_caller" "this"`)
}

func TestCompose_RefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "aws_vpc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aws_vpc", "main.tf"), []byte("# pre-existing\n"), 0o644))
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc", "--output-dir", dir)
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Contains(t, ce.Error(), "already contains generated files")
	body, _ := os.ReadFile(filepath.Join(dir, "aws_vpc", "main.tf"))
	assert.Equal(t, "# pre-existing\n", string(body), "no file must be overwritten without --force")
}

func TestCompose_ForceOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "aws_vpc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aws_vpc", "main.tf"), []byte("# stale"), 0o644))
	_, err := runCompose(t, composeSchemaJSON,
		"--modules", "aws_vpc", "--output-dir", dir, "--force")
	require.NoError(t, err)
	body, _ := os.ReadFile(filepath.Join(dir, "aws_vpc", "main.tf"))
	assert.Contains(t, string(body), `resource "aws_vpc" "this"`)
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

// ── --filter and --all ────────────────────────────────────────────────────────

func TestFilterModules_GlobMatchesBothKinds(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	got, err := filterModules(s, []string{"aws_*"})
	require.NoError(t, err)
	// schema has: aws_vpc (resource), aws_subnet (resource), aws_caller (data)
	assert.Equal(t, []string{"data.aws_caller", "resource.aws_subnet", "resource.aws_vpc"}, got)
}

func TestFilterModules_ExactName(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	got, err := filterModules(s, []string{"aws_vpc"})
	require.NoError(t, err)
	assert.Equal(t, []string{"resource.aws_vpc"}, got)
}

func TestFilterModules_MultiplePatterns(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	got, err := filterModules(s, []string{"aws_vpc", "aws_subnet"})
	require.NoError(t, err)
	assert.Equal(t, []string{"resource.aws_subnet", "resource.aws_vpc"}, got)
}

func TestFilterModules_NoMatch(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	got, err := filterModules(s, []string{"gcp_*"})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFilterModules_InvalidPattern(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	_, err := filterModules(s, []string{"aws_[invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter pattern")
}

func TestFilterModules_StarMatchesAll(t *testing.T) {
	t.Parallel()
	s := schemaFromJSON(t, composeSchemaJSON)
	got, err := filterModules(s, []string{"*"})
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestCompose_FilterFlag(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON,
		"--filter", "aws_vpc,aws_subnet", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	require.Len(t, summary.Modules, 2)
	names := []string{summary.Modules[0].Module, summary.Modules[1].Module}
	assert.ElementsMatch(t, []string{"aws_vpc", "aws_subnet"}, names)
}

func TestCompose_AllFlag(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON, "--all", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	assert.Len(t, summary.Modules, 3)
}

func TestCompose_FilterNoMatch(t *testing.T) {
	t.Parallel()
	_, err := runCompose(t, composeSchemaJSON, "--filter", "gcp_*", "--dry-run")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCompose_FilterAndModulesMerge(t *testing.T) {
	t.Parallel()
	out, err := runCompose(t, composeSchemaJSON,
		"--modules", "data.aws_caller", "--filter", "aws_vpc", "--dry-run", "--format", "json")
	require.NoError(t, err)
	var summary composeJSONSummary
	require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
	assert.Len(t, summary.Modules, 2)
}
