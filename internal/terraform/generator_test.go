package terraform

import (
	"strings"
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
				Source: "git::https://example.com/aws-vpc.git",
				Variables: []catalog.Variable{
					{Name: "cidr_block", Type: "string", Required: true, Description: "VPC CIDR"},
					{Name: "enable_dns", Type: "bool", Default: true},
				},
				Outputs: []catalog.Output{{Name: "id", Description: "VPC ID"}}},
			{Name: "aws_subnet", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "vpc_id", Type: "string", Required: true,
						References: []catalog.VariableReference{{Module: "aws_vpc", Output: "id"}}},
					{Name: "cidr_block", Type: "string", Required: true},
				},
				Outputs: []catalog.Output{{Name: "id"}}},
		},
	}
}

func TestPlan_TopologicalDepsFirst(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_subnet", "aws_vpc"}})
	require.NoError(t, err)
	require.Len(t, plan.Modules, 2)
	assert.Equal(t, "aws_vpc", plan.Modules[0].Module.Name, "deps must come first")
	assert.Equal(t, "aws_subnet", plan.Modules[1].Module.Name)
}

func TestPlan_WiringAndExternals(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_vpc", "aws_subnet"}})
	require.NoError(t, err)
	subnet := plan.Modules[1]
	require.Len(t, subnet.WiredInputs, 1)
	assert.Equal(t, "vpc_id", subnet.WiredInputs[0].VarName)
	assert.Equal(t, "this_aws_vpc", subnet.WiredInputs[0].FromInstance)
	require.Len(t, subnet.ExternalInputs, 1)
	assert.Equal(t, "aws_subnet_cidr_block", subnet.ExternalInputs[0].VarName)
}

func TestPlan_PartialSelectionWarning(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_subnet"}})
	require.NoError(t, err)
	require.NotEmpty(t, plan.Warnings, "missing referenced module should warn")
	require.Len(t, plan.Modules, 1)
	// vpc_id falls through to external input.
	hasVpcId := false
	for _, in := range plan.Modules[0].ExternalInputs {
		if in.LocalName == "vpc_id" {
			hasVpcId = true
		}
	}
	assert.True(t, hasVpcId, "wired input must materialise as external when source is unselected")
}

func TestPlan_AmbiguousReference(t *testing.T) {
	t.Parallel()
	s := planSchema()
	s.Modules = append(s.Modules, catalog.ModuleEntry{
		Name: "alt_vpc", Type: catalog.ModuleTypeResource,
		Outputs: []catalog.Output{{Name: "id"}},
	})
	// Make subnet.vpc_id reference both aws_vpc and alt_vpc.
	s.Modules[1].Variables[0].References = []catalog.VariableReference{
		{Module: "aws_vpc", Output: "id"},
		{Module: "alt_vpc", Output: "id"},
	}
	_, err := Plan(s, PlanOptions{Modules: []string{"aws_vpc", "alt_vpc", "aws_subnet"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAmbiguousReference)
}

func TestPlan_UnknownModule(t *testing.T) {
	t.Parallel()
	_, err := Plan(planSchema(), PlanOptions{Modules: []string{"missing"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, catalog.ErrUnknownModule)
}

func TestGenerate_FiveCoreFiles(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_vpc", "aws_subnet"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	require.Len(t, files, 5)
	names := []string{}
	for _, f := range files {
		names = append(names, f.Path)
	}
	assert.Equal(t, []string{"providers.tf", "variables.tf", "locals.tf", "main.tf", "outputs.tf"}, names)
}

func TestGenerate_ContentSnapshots(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{Modules: []string{"aws_vpc", "aws_subnet"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	by := map[string]string{}
	for _, f := range files {
		by[f.Path] = string(f.Content)
	}

	// providers.tf
	assert.Contains(t, by["providers.tf"], `source  = "hashicorp/aws"`)
	assert.Contains(t, by["providers.tf"], `~> 5.42`)
	assert.Contains(t, by["providers.tf"], `provider "aws"`)

	// variables.tf — every external input prefixed by module name
	assert.Contains(t, by["variables.tf"], `variable "aws_vpc_cidr_block"`)
	assert.Contains(t, by["variables.tf"], `variable "aws_vpc_enable_dns"`)
	assert.Contains(t, by["variables.tf"], `default = true`)
	assert.Contains(t, by["variables.tf"], `variable "aws_subnet_cidr_block"`)
	// type rendered raw, not quoted
	assert.Contains(t, by["variables.tf"], `type = string`)

	// main.tf — wiring and external var refs
	assert.Contains(t, by["main.tf"], `module "this_aws_vpc"`)
	assert.Contains(t, by["main.tf"], `module "this_aws_subnet"`)
	assert.Contains(t, by["main.tf"], `vpc_id     = module.this_aws_vpc.id`)
	assert.Contains(t, by["main.tf"], `var.aws_subnet_cidr_block`)
	assert.Contains(t, by["main.tf"], `"git::https://example.com/aws-vpc.git"`)
	// no version attr for git source
	assert.False(t, strings.Contains(by["main.tf"], `version = "5.42.0"`),
		"git modules must not get a version attribute")

	// outputs.tf — namespaced exports
	assert.Contains(t, by["outputs.tf"], `output "aws_vpc_id"`)
	assert.Contains(t, by["outputs.tf"], `output "aws_subnet_id"`)
	assert.Contains(t, by["outputs.tf"], `value       = module.this_aws_vpc.id`)
}

func TestGenerate_PlaceholderSourceComment(t *testing.T) {
	t.Parallel()
	s := planSchema()
	s.Modules[0].Source = "" // strip git URL → placeholder
	plan, err := Plan(s, PlanOptions{Modules: []string{"aws_vpc"}})
	require.NoError(t, err)
	files, err := Generate(plan)
	require.NoError(t, err)
	for _, f := range files {
		if f.Path == "main.tf" {
			assert.Contains(t, string(f.Content), "TODO: replace placeholder source")
			assert.Contains(t, string(f.Content), `"TODO: set module source"`)
			return
		}
	}
	t.Fatal("main.tf not found")
}
