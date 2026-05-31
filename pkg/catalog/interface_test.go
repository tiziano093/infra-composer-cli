package catalog

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ifaceSchema() *Schema {
	return &Schema{
		SchemaVersion: SchemaVersion, Provider: "hashicorp/aws", ProviderVersion: "5.42.0",
		Modules: []ModuleEntry{
			{Name: "aws_vpc", Type: ModuleTypeResource,
				Variables: []Variable{
					{Name: "cidr_block", Type: "string", Required: true},
					{Name: "enable_dns", Type: "bool", Default: true},
				},
				Outputs: []Output{{Name: "id"}}},
			{Name: "aws_subnet", Type: ModuleTypeResource,
				Variables: []Variable{
					{Name: "vpc_id", Type: "string", Required: true,
						References: []VariableReference{{Module: "aws_vpc", Output: "id"}}},
					{Name: "cidr_block", Type: "string", Required: true},
					{Name: "az", Type: "string"},
				},
				Outputs: []Output{{Name: "id"}}},
		},
	}
}

func TestExtractInterface_FullView(t *testing.T) {
	t.Parallel()
	got, err := ExtractInterface(ifaceSchema(), ExtractOptions{
		Modules: []string{"aws_vpc", "aws_subnet"},
	})
	require.NoError(t, err)
	require.Len(t, got.Modules, 2)
	subnet := got.Modules[1]
	require.Len(t, subnet.Inputs, 3)
	assert.True(t, subnet.Inputs[0].Wired, "vpc_id should be wired to aws_vpc.id")
	require.NotNil(t, subnet.Inputs[0].Source)
	assert.Equal(t, "aws_vpc", subnet.Inputs[0].Source.Module)

	// AllInputs default view hides wired inputs.
	for _, in := range got.AllInputs {
		assert.False(t, in.Wired, "wired inputs must be excluded from AllInputs")
	}
	assert.Len(t, got.AllOutputs, 2)
}

func TestExtractInterface_RequiredOnly(t *testing.T) {
	t.Parallel()
	got, err := ExtractInterface(ifaceSchema(), ExtractOptions{
		Modules:      []string{"aws_vpc", "aws_subnet"},
		RequiredOnly: true,
	})
	require.NoError(t, err)
	for _, mi := range got.Modules {
		for _, in := range mi.Inputs {
			assert.True(t, in.Required, "RequiredOnly must filter optionals")
			assert.False(t, in.Wired, "RequiredOnly must drop wired inputs")
		}
	}
	// vpc_id (wired) is dropped, only cidr_block (vpc + subnet) remain.
	names := []string{}
	for _, in := range got.AllInputs {
		names = append(names, in.Module+"."+in.Name)
	}
	assert.ElementsMatch(t, []string{"aws_subnet.cidr_block", "aws_vpc.cidr_block"}, names)
}

func TestExtractInterface_FullIncludesWiredInAggregate(t *testing.T) {
	t.Parallel()
	got, err := ExtractInterface(ifaceSchema(), ExtractOptions{
		Modules: []string{"aws_vpc", "aws_subnet"},
		Full:    true,
	})
	require.NoError(t, err)
	wiredFound := false
	for _, in := range got.AllInputs {
		if in.Wired {
			wiredFound = true
		}
	}
	assert.True(t, wiredFound, "Full=true must surface wired inputs in AllInputs")
}

func TestExtractInterface_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := ExtractInterface(ifaceSchema(), ExtractOptions{Modules: []string{"missing"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownModule))
}

func TestExtractInterface_PartialSelectionLeavesWiredInputExternal(t *testing.T) {
	t.Parallel()
	// When aws_vpc is not selected, subnet.vpc_id stays user-facing.
	got, err := ExtractInterface(ifaceSchema(), ExtractOptions{
		Modules: []string{"aws_subnet"},
	})
	require.NoError(t, err)
	require.Len(t, got.Modules, 1)
	for _, in := range got.Modules[0].Inputs {
		assert.False(t, in.Wired, "wiring requires the source module to be in the selection")
	}
}
