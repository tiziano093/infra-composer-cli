package terraform

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

func planSchema() *catalog.Schema {
	return &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "hashicorp/aws", ProviderVersion: "5.42.0",
		Modules: []catalog.ModuleEntry{
			{Name: "aws_vpc", Type: catalog.ModuleTypeResource,
				Description: "Manages a VPC",
				Variables: []catalog.Variable{
					{Name: "cidr_block", Type: "string", Required: true, Description: "VPC CIDR"},
					{Name: "enable_dns", Type: "bool", Default: true},
					{Name: "tags", Type: "map(string)"},
				},
				Outputs: []catalog.Output{{Name: "id", Description: "VPC ID"}, {Name: "arn"}}},
			{Name: "aws_secret", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "value", Type: "string", Required: true, Sensitive: true},
				},
				Outputs: []catalog.Output{{Name: "id"}, {Name: "secret_value", Sensitive: true}}},
			{Name: "aws_caller", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "name", Type: "string", Required: true},
				},
				Outputs: []catalog.Output{{Name: "id"}}},
			{Name: "aws_caller", Type: catalog.ModuleTypeData,
				Variables: []catalog.Variable{
					{Name: "name", Type: "string", Required: true},
				},
				Outputs: []catalog.Output{{Name: "id"}, {Name: "account_id"}}},
			{Name: "aws_instance", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "ami", Type: "string", Required: true},
					{Name: "ebs_block_device", Type: "list(any)"},
					{Name: "ebs_block_device.size", Type: "number"},
				},
				Outputs: []catalog.Output{{Name: "id"}}},
		},
	}
}

func TestPlan_BareNamePicksResource(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_caller"}})
	require.NoError(t, err)
	require.Len(t, plan.Modules, 1)
	assert.Equal(t, catalog.ModuleTypeResource, plan.Modules[0].Kind)
}

func TestPlan_KindQualifiedPicksData(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"data.aws_caller"}})
	require.NoError(t, err)
	require.Len(t, plan.Modules, 1)
	assert.Equal(t, catalog.ModuleTypeData, plan.Modules[0].Kind)
}

func TestPlan_DedupKeepsFirstAndPreservesOrder(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_secret", "aws_vpc", "aws_secret"}})
	require.NoError(t, err)
	require.Len(t, plan.Modules, 2)
	assert.Equal(t, "aws_secret", plan.Modules[0].ResourceType)
	assert.Equal(t, "aws_vpc", plan.Modules[1].ResourceType)
}

func TestPlan_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := Plan(planSchema(), PlanOptions{Modules: []string{"missing"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, catalog.ErrUnknownModule)
}

func TestPlan_UnknownKindPrefix(t *testing.T) {
	t.Parallel()
	_, err := Plan(planSchema(), PlanOptions{Modules: []string{"weird.aws_vpc"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, catalog.ErrUnknownModule)
}

func TestPlan_NestedBlocksFlaggedCutB(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_instance"}})
	require.NoError(t, err)
	mod := plan.Modules[0]
	for _, v := range mod.Variables {
		assert.NotContains(t, v.Name, ".", "dotted children must not be surfaced")
	}
	var ebs *ModuleVariable
	for i, v := range mod.Variables {
		if v.Name == "ebs_block_device" {
			ebs = &mod.Variables[i]
		}
	}
	require.NotNil(t, ebs, "expected nested block parent to be present")
	assert.True(t, ebs.Nested)
	assert.NotEmpty(t, mod.Warnings, "nested-block module must surface a warning")
}

func TestGenerate_FilesPerModule(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_vpc", "data.aws_caller"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	require.Len(t, files, 10)
	expected := []string{
		path.Join("aws_vpc", "version.tf"),
		path.Join("aws_vpc", "variables.tf"),
		path.Join("aws_vpc", "main.tf"),
		path.Join("aws_vpc", "outputs.tf"),
		path.Join("aws_vpc", "README.md"),
		path.Join("aws_caller", "version.tf"),
		path.Join("aws_caller", "variables.tf"),
		path.Join("aws_caller", "main.tf"),
		path.Join("aws_caller", "outputs.tf"),
		path.Join("aws_caller", "README.md"),
	}
	got := make([]string, 0, len(files))
	for _, f := range files {
		got = append(got, f.Path)
	}
	assert.Equal(t, expected, got)
}

func TestGenerate_ResourceContent(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_vpc"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)

	by := byName(files)

	assert.Contains(t, by["aws_vpc/version.tf"], `source  = "hashicorp/aws"`)
	assert.Contains(t, by["aws_vpc/version.tf"], `version = "~> 5.42"`)
	assert.Contains(t, by["aws_vpc/version.tf"], `required_version = ">= 1.0.0"`)
	assert.NotContains(t, by["aws_vpc/version.tf"], `provider "aws"`)

	assert.Contains(t, by["aws_vpc/variables.tf"], `variable "cidr_block"`)
	assert.Regexp(t, `type\s+= string`, by["aws_vpc/variables.tf"])
	assert.Contains(t, by["aws_vpc/variables.tf"], `default = true`)
	assert.Regexp(t, `type\s+= map\(string\)`, by["aws_vpc/variables.tf"])
	assert.Contains(t, by["aws_vpc/variables.tf"], `description = "VPC CIDR"`)

	main := by["aws_vpc/main.tf"]
	assert.Contains(t, main, `resource "aws_vpc" "this"`)
	assert.NotContains(t, main, `module "`)
	assert.Contains(t, main, `cidr_block = var.cidr_block`)
	assert.Contains(t, main, `enable_dns = var.enable_dns`)

	out := by["aws_vpc/outputs.tf"]
	assert.Contains(t, out, `output "id"`)
	assert.Regexp(t, `value\s+= aws_vpc\.this\.id`, out)
	assert.Contains(t, out, `output "arn"`)

	readme := by["aws_vpc/README.md"]
	assert.Contains(t, readme, "# aws_vpc")
	assert.Contains(t, readme, "## Inputs")
	assert.Contains(t, readme, "## Outputs")
	assert.Contains(t, readme, "## Requirements")
}

func TestGenerate_DataSourceUsesDataPrefix(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"data.aws_caller"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)

	by := byName(files)
	main := by["aws_caller/main.tf"]
	assert.Contains(t, main, `data "aws_caller" "this"`)

	out := by["aws_caller/outputs.tf"]
	assert.Regexp(t, `value\s+= data\.aws_caller\.this\.account_id`, out)
}

func TestGenerate_SensitivePropagates(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_secret"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	by := byName(files)

	vars := by["aws_secret/variables.tf"]
	assert.Regexp(t, `sensitive\s+= true`, vars)

	outs := by["aws_secret/outputs.tf"]
	assert.Contains(t, outs, `output "secret_value"`)
	assert.Regexp(t, `sensitive\s+= true`, outs)
}

func TestGenerate_NestedBlockRendersTODO(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_instance"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	by := byName(files)

	main := by["aws_instance/main.tf"]
	assert.Contains(t, main, `ami = var.ami`)
	assert.Contains(t, main, "TODO: configure nested block")

	vars := by["aws_instance/variables.tf"]
	assert.Contains(t, vars, `variable "ebs_block_device"`)
	assert.Regexp(t, `type\s+= any`, vars)
}

func byName(files []GeneratedFile) map[string]string {
	m := make(map[string]string, len(files))
	for _, f := range files {
		m[f.Path] = string(f.Content)
	}
	return m
}
