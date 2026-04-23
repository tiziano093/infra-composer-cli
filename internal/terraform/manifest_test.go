package terraform

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// manifestSchema returns a small AWS-like catalog with a VPC + subnet
// pair so tests cover Cut B folding, references carried over, and
// deterministic module ordering.
func manifestSchema() *catalog.Schema {
	return &catalog.Schema{
		SchemaVersion:   catalog.SchemaVersion,
		Provider:        "hashicorp/aws",
		ProviderVersion: "5.42.0",
		Modules: []catalog.ModuleEntry{
			{
				Name:        "aws_vpc",
				Type:        catalog.ModuleTypeResource,
				Description: "Manages a VPC",
				Variables: []catalog.Variable{
					{Name: "cidr_block", Type: "string", Required: true, Description: "VPC CIDR"},
					{Name: "tags", Type: "map(string)"},
				},
				Outputs: []catalog.Output{{Name: "id", Description: "VPC ID"}},
			},
			{
				Name: "aws_subnet",
				Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "cidr_block", Type: "string", Required: true},
					{Name: "vpc_id", Type: "string", Required: true,
						References: []catalog.VariableReference{{Module: "aws_vpc", Output: "id"}}},
				},
				Outputs: []catalog.Output{{Name: "id"}},
			},
		},
	}
}

func TestBuildManifest_HeaderFromPlan(t *testing.T) {
	t.Parallel()
	plan, err := Plan(manifestSchema(), PlanOptions{Modules: []string{"aws_vpc", "aws_subnet"}})
	require.NoError(t, err)

	m := BuildManifest(plan, "catalog/schema.json")
	require.NotNil(t, m)
	assert.Equal(t, ManifestSchemaVersion, m.SchemaVersion)
	assert.Equal(t, "hashicorp/aws", m.Provider)
	assert.Equal(t, "5.42.0", m.ProviderVersion)
	assert.Equal(t, "catalog/schema.json", m.SourceCatalog)
	assert.WithinDuration(t, time.Now().UTC(), m.GeneratedAt, 2*time.Second)
}

func TestBuildManifest_ModulesCarryPathsAndEntries(t *testing.T) {
	t.Parallel()
	plan, err := Plan(manifestSchema(), PlanOptions{Modules: []string{"aws_vpc", "aws_subnet"}})
	require.NoError(t, err)

	m := BuildManifest(plan, "")
	require.Len(t, m.Modules, 2)

	vpc := m.Modules[0]
	assert.Equal(t, "./aws_vpc", vpc.Path)
	assert.Equal(t, "aws_vpc", vpc.Entry.Name)
	assert.Equal(t, catalog.ModuleTypeResource, vpc.Entry.Type)
	assert.Equal(t, "Manages a VPC", vpc.Entry.Description)
	require.Len(t, vpc.Entry.Variables, 2)
	assert.Equal(t, "cidr_block", vpc.Entry.Variables[0].Name)
	assert.True(t, vpc.Entry.Variables[0].Required)
	require.Len(t, vpc.Entry.Outputs, 1)
	assert.Equal(t, "id", vpc.Entry.Outputs[0].Name)

	subnet := m.Modules[1]
	assert.Equal(t, "./aws_subnet", subnet.Path)
	var vpcID *catalog.Variable
	for i, v := range subnet.Entry.Variables {
		if v.Name == "vpc_id" {
			vpcID = &subnet.Entry.Variables[i]
		}
	}
	require.NotNil(t, vpcID, "vpc_id variable must be present")
	require.Len(t, vpcID.References, 1)
	assert.Equal(t, "aws_vpc", vpcID.References[0].Module)
	assert.Equal(t, "id", vpcID.References[0].Output)
}

func TestBuildManifest_NestedBlockFoldedToAny(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_instance"}})
	require.NoError(t, err)

	m := BuildManifest(plan, "")
	require.Len(t, m.Modules, 1)

	var ebs *catalog.Variable
	for i, v := range m.Modules[0].Entry.Variables {
		if v.Name == "ebs_block_device" {
			ebs = &m.Modules[0].Entry.Variables[i]
		}
	}
	require.NotNil(t, ebs, "parent block variable must be surfaced")
	assert.Equal(t, "any", ebs.Type, "nested block collapses to any in the manifest")

	for _, v := range m.Modules[0].Entry.Variables {
		assert.NotContains(t, v.Name, ".",
			"dotted child attributes must not appear in the manifest (Cut B)")
	}
}

func TestRenderManifestFile_ProducesValidJSON(t *testing.T) {
	t.Parallel()
	plan, err := Plan(manifestSchema(), PlanOptions{Modules: []string{"aws_vpc"}})
	require.NoError(t, err)

	m := BuildManifest(plan, "catalog/schema.json")
	file, err := RenderManifestFile(m)
	require.NoError(t, err)
	assert.Equal(t, ManifestFileName, file.Path)
	assert.Empty(t, file.Module, "manifest is a root-level file")

	var round ComposeManifest
	require.NoError(t, json.Unmarshal(file.Content, &round))
	assert.Equal(t, ManifestSchemaVersion, round.SchemaVersion)
	assert.Equal(t, "hashicorp/aws", round.Provider)
	require.Len(t, round.Modules, 1)
	assert.Equal(t, "./aws_vpc", round.Modules[0].Path)
	assert.Equal(t, "aws_vpc", round.Modules[0].Entry.Name)
}

func TestRenderManifestFile_NilManifest(t *testing.T) {
	t.Parallel()
	_, err := RenderManifestFile(nil)
	require.Error(t, err)
}
