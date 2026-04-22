package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

const ifaceSchemaJSON = `{
  "schema_version": "1.0",
  "provider": "hashicorp/aws",
  "provider_version": "5.42.0",
  "modules": [
    {"name": "aws_vpc", "type": "resource",
     "variables": [
       {"name": "cidr_block", "type": "string", "required": true, "description": "CIDR for VPC"},
       {"name": "enable_dns", "type": "bool", "default": true}
     ],
     "outputs": [{"name": "id"}]},
    {"name": "aws_subnet", "type": "resource",
     "variables": [
       {"name": "vpc_id", "type": "string", "required": true,
        "references": [{"module": "aws_vpc", "output": "id"}]},
       {"name": "cidr_block", "type": "string", "required": true},
       {"name": "az", "type": "string"}
     ],
     "outputs": [{"name": "id"}]}
  ]
}`

func runInterface(t *testing.T, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	path := writeSchema(t, ifaceSchemaJSON)
	stdout := &bytes.Buffer{}
	cmd := NewInterfaceCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(runtimeCtx(stdout, &bytes.Buffer{}, nil))
	cmd.SetArgs(append([]string{"--schema", path}, args...))
	return stdout, cmd.Execute()
}

func TestInterface_TextDefault(t *testing.T) {
	t.Parallel()
	out, err := runInterface(t, "aws_vpc", "aws_subnet")
	require.NoError(t, err)
	body := out.String()
	assert.Contains(t, body, "module aws_vpc")
	assert.Contains(t, body, "module aws_subnet")
	assert.Contains(t, body, "aws_vpc.id", "wired source must be visible in text view")
	assert.Contains(t, body, "cidr_block")
}

func TestInterface_RequiredOnly(t *testing.T) {
	t.Parallel()
	out, err := runInterface(t, "aws_vpc", "aws_subnet", "--required-only", "--format", "json")
	require.NoError(t, err)
	var payload interfaceJSON
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	for _, in := range payload.AllInputs {
		assert.True(t, in.Required)
		assert.False(t, in.Wired)
	}
	// vpc_id (wired) and enable_dns (optional) and az (optional) must be gone.
	names := []string{}
	for _, in := range payload.AllInputs {
		names = append(names, in.Module+"."+in.Name)
	}
	assert.ElementsMatch(t, []string{"aws_subnet.cidr_block", "aws_vpc.cidr_block"}, names)
}

func TestInterface_FullIncludesWired(t *testing.T) {
	t.Parallel()
	out, err := runInterface(t, "aws_vpc", "aws_subnet", "--full", "--format", "json")
	require.NoError(t, err)
	var payload interfaceJSON
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	wired := false
	for _, in := range payload.AllInputs {
		if in.Wired && in.Source == "aws_vpc.id" {
			wired = true
		}
	}
	assert.True(t, wired)
}

func TestInterface_YAML(t *testing.T) {
	t.Parallel()
	out, err := runInterface(t, "aws_vpc", "--format", "yaml")
	require.NoError(t, err)
	var payload interfaceJSON
	require.NoError(t, yaml.Unmarshal(out.Bytes(), &payload))
	require.Len(t, payload.Modules, 1)
	assert.Equal(t, "aws_vpc", payload.Modules[0].Module)
	require.NotEmpty(t, payload.Modules[0].Inputs)
	// YAML output should not contain JSON braces.
	assert.False(t, strings.HasPrefix(strings.TrimSpace(out.String()), "{"))
}

func TestInterface_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := runInterface(t, "missing")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitModuleNotFound, ce.Code)
}

func TestInterface_RequiredAndFullExclusive(t *testing.T) {
	t.Parallel()
	_, err := runInterface(t, "aws_vpc", "--required-only", "--full")
	require.Error(t, err)
	var ce *clierr.CLIError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, clierr.ExitInvalidArgs, ce.Code)
}
