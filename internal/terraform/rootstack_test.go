package terraform

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// normalize collapses runs of spaces/tabs within each line so tests can assert
// against canonical single-space forms regardless of hclwrite's attribute
// alignment padding.
var hclAlignmentRE = regexp.MustCompile(`[ \t]+`)

func normalize(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = hclAlignmentRE.ReplaceAllString(l, " ")
	}
	return strings.Join(lines, "\n")
}

// wiringSchema returns a minimal AWS-like catalog with a VPC + subnet
// pair where `aws_subnet.vpc_id` declares a cross-module reference to
// `aws_vpc.id`. Used by the root-stack wiring tests.
func wiringSchema() *catalog.Schema {
	return &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "hashicorp/aws", ProviderVersion: "5.42.0",
		Modules: []catalog.ModuleEntry{
			{
				Name: "aws_vpc", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "cidr_block", Type: "string", Required: true},
				},
				Outputs: []catalog.Output{{Name: "id"}},
			},
			{
				Name: "aws_subnet", Type: catalog.ModuleTypeResource,
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

func cycleSchema() *catalog.Schema {
	return &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "hashicorp/aws", ProviderVersion: "5.42.0",
		Modules: []catalog.ModuleEntry{
			{
				Name: "aws_a", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "from_b", Type: "string", Required: true,
						References: []catalog.VariableReference{{Module: "aws_b", Output: "id"}}},
				},
				Outputs: []catalog.Output{{Name: "id"}},
			},
			{
				Name: "aws_b", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{
					{Name: "from_a", Type: "string", Required: true,
						References: []catalog.VariableReference{{Module: "aws_a", Output: "id"}}},
				},
				Outputs: []catalog.Output{{Name: "id"}},
			},
		},
	}
}

func TestRootStack_SingleModuleNoReferences(t *testing.T) {
	t.Parallel()
	plan, err := Plan(planSchema(), PlanOptions{
		Modules: []string{"aws_vpc"}, EmitRootStack: true,
	})
	require.NoError(t, err)
	require.True(t, plan.EmitRootStack)

	files, err := Generate(plan)
	require.NoError(t, err)

	by := byName(files)
	require.Contains(t, by, "versions.tf")
	require.Contains(t, by, "providers.tf")
	require.Contains(t, by, "variables.tf")
	require.Contains(t, by, "locals.tf")
	require.Contains(t, by, "main.tf")
	require.Contains(t, by, "outputs.tf")

	assert.Contains(t, normalize(by["versions.tf"]), `source = "hashicorp/aws"`)
	assert.Contains(t, by["providers.tf"], `provider "aws"`)

	vars := by["variables.tf"]
	assert.Contains(t, vars, `variable "cidr_block"`)
	assert.Contains(t, vars, `variable "enable_dns"`)
	assert.Contains(t, vars, `variable "tags"`)

	main := normalize(by["main.tf"])
	assert.Contains(t, main, `module "aws_vpc"`)
	assert.Contains(t, main, `source = "./aws_vpc"`)
	assert.Contains(t, main, `cidr_block = var.cidr_block`)

	outs := normalize(by["outputs.tf"])
	assert.Contains(t, outs, `output "aws_vpc_id"`)
	assert.Contains(t, outs, `value = module.aws_vpc.id`)
	assert.Contains(t, outs, `output "aws_vpc_arn"`)
}

func TestRootStack_WiresCrossModuleReference(t *testing.T) {
	t.Parallel()
	plan, err := Plan(wiringSchema(), PlanOptions{
		Modules: []string{"aws_vpc", "aws_subnet"}, EmitRootStack: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"aws_vpc", "aws_subnet"}, plan.TopoOrder,
		"dependency order must place aws_vpc before aws_subnet")

	files, err := Generate(plan)
	require.NoError(t, err)
	by := byName(files)

	main := normalize(by["main.tf"])
	assert.Contains(t, main, `module "aws_vpc"`)
	assert.Contains(t, main, `module "aws_subnet"`)
	assert.Contains(t, main, `vpc_id = module.aws_vpc.id`)

	vars := by["variables.tf"]
	assert.NotContains(t, vars, `variable "vpc_id"`,
		"wired references must not appear as root variables")

	// cidr_block exists in both modules — must be disambiguated.
	assert.Contains(t, vars, `variable "aws_vpc_cidr_block"`)
	assert.Contains(t, vars, `variable "aws_subnet_cidr_block"`)
	assert.NotContains(t, vars, `variable "cidr_block"`)

	assert.Contains(t, main, `cidr_block = var.aws_vpc_cidr_block`)
	assert.Contains(t, main, `cidr_block = var.aws_subnet_cidr_block`)
}

func TestRootStack_CycleRejected(t *testing.T) {
	t.Parallel()
	_, err := Plan(cycleSchema(), PlanOptions{
		Modules: []string{"aws_a", "aws_b"}, EmitRootStack: true,
	})
	require.Error(t, err)
	var cyc *catalog.CycleError
	assert.ErrorAs(t, err, &cyc)
}

func TestRootStack_DisabledEmitsNoRootFiles(t *testing.T) {
	t.Parallel()
	plan, err := Plan(wiringSchema(), PlanOptions{
		Modules: []string{"aws_vpc", "aws_subnet"},
	})
	require.NoError(t, err)
	assert.False(t, plan.EmitRootStack)
	assert.Empty(t, plan.TopoOrder)

	files, err := Generate(plan)
	require.NoError(t, err)

	for _, f := range files {
		assert.NotEmpty(t, f.Module,
			"root-stack files must not be emitted when EmitRootStack is false (saw %q)", f.Path)
	}
}

func TestRootStack_ReferenceOutsidePlanSurfacesAsVar(t *testing.T) {
	t.Parallel()
	plan, err := Plan(wiringSchema(), PlanOptions{
		Modules: []string{"aws_subnet"}, EmitRootStack: true,
	})
	require.NoError(t, err)

	files, err := Generate(plan)
	require.NoError(t, err)
	by := byName(files)

	vars := by["variables.tf"]
	assert.Contains(t, vars, `variable "vpc_id"`,
		"reference to a module not in the plan must fall back to a user variable")

	main := normalize(by["main.tf"])
	assert.Contains(t, main, `vpc_id = var.vpc_id`)
}
