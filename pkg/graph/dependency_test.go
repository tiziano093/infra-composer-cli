package graph

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiziano093/infra-composer-cli/pkg/catalog"
)

func depSchema(t *testing.T) *catalog.Schema {
	t.Helper()
	return &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "hashicorp/aws", ProviderVersion: "5.42.0",
		Modules: []catalog.ModuleEntry{
			{Name: "aws_vpc", Type: catalog.ModuleTypeResource,
				Outputs: []catalog.Output{{Name: "id"}}},
			{Name: "aws_subnet", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{{
					Name: "vpc_id", Type: "string", Required: true,
					References: []catalog.VariableReference{{Module: "aws_vpc", Output: "id"}},
				}},
				Outputs: []catalog.Output{{Name: "id"}}},
			{Name: "aws_instance", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{{
					Name: "subnet_id", Type: "string", Required: true,
					References: []catalog.VariableReference{{Module: "aws_subnet", Output: "id"}},
				}},
				Outputs: []catalog.Output{{Name: "id"}}},
			{Name: "aws_caller_identity", Type: catalog.ModuleTypeData,
				Outputs: []catalog.Output{{Name: "account_id"}}},
		},
	}
}

func TestBuildGraph_NodesAndEdges(t *testing.T) {
	t.Parallel()
	g := BuildGraph(depSchema(t))
	assert.Equal(t, []string{"aws_caller_identity", "aws_instance", "aws_subnet", "aws_vpc"}, g.Modules())

	subnetEdges := g.Edges("aws_subnet")
	require.Len(t, subnetEdges, 1)
	assert.Equal(t, Edge{From: "aws_subnet", To: "aws_vpc", Variable: "vpc_id", Output: "id"}, subnetEdges[0])

	assert.Empty(t, g.Edges("aws_vpc"))
	assert.Empty(t, g.Edges("aws_caller_identity"))
}

func TestBuildGraph_NilSchema(t *testing.T) {
	t.Parallel()
	g := BuildGraph(nil)
	assert.Empty(t, g.Modules())
	assert.Empty(t, g.Edges("anything"))
	assert.Empty(t, g.Cycles())
}

func TestBuildGraph_SkipsUnknownTargets(t *testing.T) {
	t.Parallel()
	s := &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "x/y", ProviderVersion: "1.0.0",
		Modules: []catalog.ModuleEntry{{
			Name: "m", Type: catalog.ModuleTypeResource,
			Variables: []catalog.Variable{{
				Name: "v", Type: "string",
				References: []catalog.VariableReference{{Module: "missing", Output: "id"}},
			}},
		}},
	}
	g := BuildGraph(s)
	assert.Empty(t, g.Edges("m"))
}

func TestGraph_Resolve_Tree(t *testing.T) {
	t.Parallel()
	g := BuildGraph(depSchema(t))
	tree, err := g.Resolve("aws_instance", 0)
	require.NoError(t, err)
	assert.Equal(t, "aws_instance", tree.Module)
	require.Len(t, tree.Children, 1)
	assert.Equal(t, "aws_subnet", tree.Children[0].Module)
	require.Len(t, tree.Children[0].Children, 1)
	assert.Equal(t, "aws_vpc", tree.Children[0].Children[0].Module)
	assert.Equal(t, 2, tree.Children[0].Children[0].Depth)
}

func TestGraph_Resolve_DepthLimit(t *testing.T) {
	t.Parallel()
	g := BuildGraph(depSchema(t))
	tree, err := g.Resolve("aws_instance", 1)
	require.NoError(t, err)
	require.Len(t, tree.Children, 1)
	assert.Equal(t, "aws_subnet", tree.Children[0].Module)
	assert.Empty(t, tree.Children[0].Children, "depth limit must stop expansion")
}

func TestGraph_Resolve_UnknownModule(t *testing.T) {
	t.Parallel()
	g := BuildGraph(depSchema(t))
	_, err := g.Resolve("nope", 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, catalog.ErrUnknownModule))
}

func TestGraph_Cycles(t *testing.T) {
	t.Parallel()
	s := &catalog.Schema{
		SchemaVersion: catalog.SchemaVersion, Provider: "x/y", ProviderVersion: "1.0.0",
		Modules: []catalog.ModuleEntry{
			{Name: "a", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{{Name: "in", Type: "string",
					References: []catalog.VariableReference{{Module: "b", Output: "out"}}}},
				Outputs: []catalog.Output{{Name: "out"}}},
			{Name: "b", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{{Name: "in", Type: "string",
					References: []catalog.VariableReference{{Module: "c", Output: "out"}}}},
				Outputs: []catalog.Output{{Name: "out"}}},
			{Name: "c", Type: catalog.ModuleTypeResource,
				Variables: []catalog.Variable{{Name: "in", Type: "string",
					References: []catalog.VariableReference{{Module: "a", Output: "out"}}}},
				Outputs: []catalog.Output{{Name: "out"}}},
		},
	}
	g := BuildGraph(s)
	cycles := g.Cycles()
	require.Len(t, cycles, 1)
	assert.Equal(t, []string{"a", "b", "c"}, cycles[0])

	_, err := g.Resolve("a", 0)
	require.Error(t, err)
	var ce *CycleError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, []string{"a", "b", "c"}, ce.Cycle)
}

func TestGraph_DAG_NoCycles(t *testing.T) {
	t.Parallel()
	g := BuildGraph(depSchema(t))
	assert.Empty(t, g.Cycles())
}
