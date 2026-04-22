package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

// registryFixturesDir resolves the test/fixtures/registry directory
// from this package; kept package-local so the build/list/export tests
// can share a single helper.
func registryFixturesDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "test", "fixtures", "registry")
}

func TestCatalogBuild_Text(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{
		"build",
		"--provider", "hashicorp/aws",
		"--output-dir", outDir,
		"--registry-dir", registryFixturesDir(t),
	})
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "Built catalog for hashicorp/aws@5.42.0")
	assert.Contains(t, out, "3 modules")
	assert.FileExists(t, filepath.Join(outDir, "schema.json"))
}

func TestCatalogBuild_JSON(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{
		"build",
		"--provider", "hashicorp/aws",
		"--output-dir", outDir,
		"--registry-dir", registryFixturesDir(t),
		"--format", "json",
	})
	require.NoError(t, cmd.Execute())

	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, "hashicorp/aws", report["provider"])
	assert.Equal(t, "5.42.0", report["provider_version"])
	assert.EqualValues(t, 3, report["modules"])
}

func TestCatalogBuild_RequiresProvider(t *testing.T) {
	t.Parallel()
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"build", "--output-dir", t.TempDir()})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCatalogBuild_RequiresOutputDir(t *testing.T) {
	t.Parallel()
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"build", "--provider", "hashicorp/aws", "--registry-dir", registryFixturesDir(t)})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCatalogBuild_ProviderNotFound(t *testing.T) {
	t.Parallel()
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{
		"build",
		"--provider", "hashicorp/missing",
		"--output-dir", t.TempDir(),
		"--registry-dir", registryFixturesDir(t),
	})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitFileNotFound, ce.Code)
}

func TestCatalogList_Table(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"list", path})
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "hashicorp/aws@5.42.0")
	assert.Contains(t, out, "4 module")
	assert.Contains(t, out, "aws_vpc")
	assert.Contains(t, out, "aws_caller_identity")
}

func TestCatalogList_GroupFilter(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"list", path, "--group", "compute"})
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "aws_instance")
	assert.NotContains(t, out, "aws_vpc")
}

func TestCatalogList_JSON(t *testing.T) {
	t.Parallel()
	path := writeSchema(t, validSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"list", path, "--format", "json"})
	require.NoError(t, cmd.Execute())

	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, "hashicorp/aws", report["provider"])
	assert.EqualValues(t, 4, report["modules"])
	entries, _ := report["entries"].([]any)
	assert.Len(t, entries, 4)
}

func TestCatalogList_NoPath(t *testing.T) {
	t.Parallel()
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCatalogExport_ToDir(t *testing.T) {
	t.Parallel()
	src := writeSchema(t, validSchemaJSON)
	outDir := t.TempDir()
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"export", src, "--output", outDir})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "Exported")
	assert.FileExists(t, filepath.Join(outDir, "schema.json"))
}

func TestCatalogExport_ToFile_JSON(t *testing.T) {
	t.Parallel()
	src := writeSchema(t, validSchemaJSON)
	dest := filepath.Join(t.TempDir(), "out.json")
	stdout := &bytes.Buffer{}
	cmd := NewCatalogCommand()
	cmd.SetOut(stdout)
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"export", src, "--output", dest, "--format", "json"})
	require.NoError(t, cmd.Execute())

	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, dest, report["destination"])
	assert.FileExists(t, dest)
}

func TestCatalogExport_RequiresOutput(t *testing.T) {
	t.Parallel()
	src := writeSchema(t, validSchemaJSON)
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"export", src})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}

func TestCatalogExport_NoSource(t *testing.T) {
	t.Parallel()
	cmd := NewCatalogCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(&bytes.Buffer{}, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"export", "--output", t.TempDir()})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}
