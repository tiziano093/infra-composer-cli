package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

const depSchemaJSON = `{
  "schema_version": "1.0",
  "provider": "hashicorp/aws",
  "provider_version": "5.42.0",
  "modules": [
    {"name": "aws_vpc", "type": "resource", "outputs": [{"name": "id"}]},
    {"name": "aws_subnet", "type": "resource",
     "variables": [{"name": "vpc_id", "type": "string", "required": true,
                    "references": [{"module": "aws_vpc", "output": "id"}]}],
     "outputs": [{"name": "id"}]},
    {"name": "aws_instance", "type": "resource",
     "variables": [{"name": "subnet_id", "type": "string", "required": true,
                    "references": [{"module": "aws_subnet", "output": "id"}]}]}
  ]
}`

const cyclicSchemaJSON = `{
  "schema_version": "1.0",
  "provider": "x/y",
  "provider_version": "1.0.0",
  "modules": [
    {"name": "a", "type": "resource",
     "variables": [{"name": "in", "type": "string",
                    "references": [{"module": "b", "output": "out"}]}],
     "outputs": [{"name": "out"}]},
    {"name": "b", "type": "resource",
     "variables": [{"name": "in", "type": "string",
                    "references": [{"module": "a", "output": "out"}]}],
     "outputs": [{"name": "out"}]}
  ]
}`

func runDependencies(t *testing.T, schemaBody string, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	path := writeSchema(t, schemaBody)
	stdout := &bytes.Buffer{}
	cmd := NewDependenciesCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs(append([]string{"--schema", path}, args...))
	return stdout, cmd.Execute()
}

func TestDependencies_TextTree(t *testing.T) {
	t.Parallel()
	out, err := runDependencies(t, depSchemaJSON, "aws_instance")
	require.NoError(t, err)
	body := out.String()
	assert.Contains(t, body, "aws_instance")
	assert.Contains(t, body, "aws_subnet")
	assert.Contains(t, body, "aws_vpc")
	assert.Contains(t, body, "via subnet_id ← aws_subnet.id")
}

func TestDependencies_JSONTree(t *testing.T) {
	t.Parallel()
	out, err := runDependencies(t, depSchemaJSON, "aws_instance", "--format", "json")
	require.NoError(t, err)
	var node dependencyJSONNode
	require.NoError(t, json.Unmarshal(out.Bytes(), &node))
	assert.Equal(t, "aws_instance", node.Module)
	require.Len(t, node.Children, 1)
	assert.Equal(t, "aws_subnet", node.Children[0].Module)
	require.Len(t, node.Children[0].Children, 1)
	assert.Equal(t, "aws_vpc", node.Children[0].Children[0].Module)
}

func TestDependencies_DepthLimit(t *testing.T) {
	t.Parallel()
	out, err := runDependencies(t, depSchemaJSON, "aws_instance", "--depth", "1")
	require.NoError(t, err)
	body := out.String()
	assert.Contains(t, body, "aws_subnet")
	assert.NotContains(t, body, "aws_vpc")
}

func TestDependencies_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := runDependencies(t, depSchemaJSON, "nope")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitModuleNotFound, ce.Code)
}

func TestDependencies_CycleDetectedDuringResolve(t *testing.T) {
	t.Parallel()
	_, err := runDependencies(t, cyclicSchemaJSON, "a")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitDependencyFailed, ce.Code)
	assert.True(t, strings.Contains(ce.Error(), "cycle"), "expected cycle in error: %s", ce.Error())
}

func TestDependencies_CheckCyclesFlag(t *testing.T) {
	t.Parallel()
	_, err := runDependencies(t, cyclicSchemaJSON, "a", "--check-cycles")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitDependencyFailed, ce.Code)
	require.NotEmpty(t, ce.Suggestions)
}

func TestDependencies_NoSchemaConfigured(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewDependenciesCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs([]string{"aws_vpc"})
	err := cmd.Execute()
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}
