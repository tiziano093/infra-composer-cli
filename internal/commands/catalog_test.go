package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/clierr"
	"github.com/tiziano093/infra-composer-cli/internal/config"
)

const validSchemaJSON = `{
  "schema_version": "1.0",
  "provider": "hashicorp/aws",
  "provider_version": "5.42.0",
  "modules": [
    {"name": "aws_vpc", "type": "resource", "group": "network", "description": "Virtual Private Cloud"},
    {"name": "aws_subnet", "type": "resource", "group": "network", "description": "Subnet inside a VPC"},
    {"name": "aws_instance", "type": "resource", "group": "compute", "description": "EC2 instance"},
    {"name": "aws_caller_identity", "type": "data", "group": "identity"}
  ]
}`

func writeSchema(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// runtimeCtx returns a context carrying a Runtime suitable for command
// execution under test. The schema path is plumbed via Config so we can
// exercise the config-fallback code path.
func runtimeCtx(stdout, stderr *bytes.Buffer, cfg *config.Config) context.Context {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return WithRuntime(context.Background(), Runtime{
		Config: cfg,
		Stdout: stdout,
		Stderr: stderr,
	})
}

func TestSearch_TableOutput(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", path, "vpc"})
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "aws_vpc")
	// aws_subnet hits via description ("inside a VPC").
	assert.Contains(t, out, "aws_subnet")
	assert.NotContains(t, out, "aws_instance")
}

func TestSearch_NoMatchesPrintsHint(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", path, "nonexistent_xyz"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "(no matches)")
}

func TestSearch_JSONOutput(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", path, "--format", "json", "vpc"})
	require.NoError(t, cmd.Execute())

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &entries))
	require.NotEmpty(t, entries)
	assert.Equal(t, "aws_vpc", entries[0]["name"])
}

func TestSearch_TypeFilter(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", path, "--type", "data"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "aws_caller_identity")
	assert.NotContains(t, stdout.String(), "aws_vpc")
}

func TestSearch_InvalidTypeFlag(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stderr := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(stderr)
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, stderr, nil))
	cmd.SetArgs([]string{"--schema", path, "--type", "weird"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestSearch_MissingSchemaFile(t *testing.T) {
	t.Parallel()
	cmd := NewSearchCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", filepath.Join(t.TempDir(), "missing.json"), "x"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitFileNotFound, ce.Code)
}

func TestSearch_NoSchemaConfigured(t *testing.T) {
	t.Parallel()
	cmd := NewSearchCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"vpc"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestSearch_SchemaFromConfigFallback(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	cfg := &config.Config{Catalog: config.CatalogConfig{SchemaPath: path}}
	stdout := &bytes.Buffer{}
	cmd := NewSearchCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, cfg))
	cmd.SetArgs([]string{"vpc"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "aws_vpc")
}

func TestSearch_CatalogValidationError(t *testing.T) {
	t.Parallel()
	bad := `{"schema_version":"1.0","provider":"","provider_version":"5.42.0","modules":[]}`
	path := writeSchema(t, bad)
	cmd := NewSearchCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"--schema", path, "x"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitValidationFailed, ce.Code)
}

func TestCatalogValidate_OK(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", path})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "OK ")
	assert.Contains(t, stdout.String(), "hashicorp/aws@5.42.0")
}

func TestCatalogValidate_OK_JSON(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", path, "--format", "json"})
	require.NoError(t, cmd.Execute())

	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, true, report["valid"])
	assert.EqualValues(t, 4, report["modules"])
}

func TestCatalogValidate_ValidationError_TextLists(t *testing.T) {
	t.Parallel()
	bad := `{"schema_version":"1.0","provider":"","provider_version":"latest","modules":[]}`
	path := writeSchema(t, bad)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", path})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitValidationFailed, ce.Code)
	out := stdout.String()
	assert.Contains(t, out, "INVALID")
	assert.Contains(t, out, "provider")
	assert.Contains(t, out, "provider_version")
}

func TestCatalogValidate_ValidationError_JSONLists(t *testing.T) {
	t.Parallel()
	bad := `{"schema_version":"1.0","provider":"","provider_version":"latest","modules":[]}`
	path := writeSchema(t, bad)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", path, "--format", "json"})
	err := cmd.Execute()
	require.Error(t, err)
	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, false, report["valid"])
	issues, _ := report["issues"].([]any)
	assert.NotEmpty(t, issues)
}

func TestCatalogValidate_ParseError(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, "{not json")
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", path})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitValidationFailed, ce.Code)
	assert.Contains(t, stdout.String(), "parse error")
}

func TestCatalogValidate_MissingFile(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate", filepath.Join(t.TempDir(), "missing.json")})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitFileNotFound, ce.Code)
}

func TestCatalogValidate_NoPathProvided(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"validate"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

// (no trailing helpers)
