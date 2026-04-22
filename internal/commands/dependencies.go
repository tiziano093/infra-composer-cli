package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

type dependenciesFlags struct {
	schema      string
	depth       int
	checkCycles bool
	format      string
}

// NewDependenciesCommand returns the `dependencies <module>` subcommand
// which prints the dependency tree of a module from the loaded catalog
// schema, optionally running an up-front cycle check.
func NewDependenciesCommand() *cobra.Command {
	f := &dependenciesFlags{}
	cmd := &cobra.Command{
		Use:   "dependencies <module>",
		Short: "Show the dependency tree of a catalog module",
		Long: `Resolve and print the cross-module dependency tree of a module, derived
from the explicit Variable.references declarations in the catalog
schema.

Use --depth to truncate deep trees, and --check-cycles to fail with
ExitDependencyFailed if any dependency cycle exists in the catalog.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, _ := FromContext(cmd.Context())
			out := writerOrStdout(rt.Stdout, cmd.OutOrStdout())

			schemaPath := resolveSchemaPath(f.schema, rt)
			if schemaPath == "" {
				return invalidArgs("no catalog schema configured",
					"pass --schema <path> or set catalog.schema in config")
			}

			s, err := catalog.Load(schemaPath)
			if err != nil {
				return mapCatalogLoadError(schemaPath, err)
			}

			g := catalog.BuildGraph(s)
			root := args[0]
			if !g.Has(root) {
				return cliError(clierr.ExitModuleNotFound,
					fmt.Sprintf("module %q not found in catalog %s", root, schemaPath),
					"run `infra-composer search` to list available modules")
			}

			if f.checkCycles {
				if cyc := g.Cycles(); len(cyc) > 0 {
					return cliError(clierr.ExitDependencyFailed,
						fmt.Sprintf("catalog contains %d dependency cycle(s)", len(cyc)),
						"break the cycle by removing or restructuring one of the references").
						WithSuggestions(formatCyclesHints(cyc)...)
				}
			}

			tree, err := g.Resolve(root, f.depth)
			if err != nil {
				var ce *catalog.CycleError
				if errors.As(err, &ce) {
					return cliError(clierr.ExitDependencyFailed,
						fmt.Sprintf("dependency cycle detected: %s", strings.Join(ce.Cycle, " → ")+" → "+ce.Cycle[0]),
						"use --check-cycles to enumerate every cycle in the catalog")
				}
				return err
			}

			format := chooseFormat(f.format, rt)
			return renderDependencies(out, tree, format)
		},
	}

	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (defaults to catalog.schema in config)")
	cmd.Flags().IntVar(&f.depth, "depth", 0, "Maximum tree depth (0 = unlimited)")
	cmd.Flags().BoolVar(&f.checkCycles, "check-cycles", false, "Fail with ExitDependencyFailed if any cycle exists in the catalog")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}

func formatCyclesHints(cycles [][]string) []string {
	out := make([]string, 0, len(cycles))
	for _, c := range cycles {
		if len(c) == 0 {
			continue
		}
		out = append(out, "cycle: "+strings.Join(c, " → ")+" → "+c[0])
	}
	return out
}

func renderDependencies(out io.Writer, tree *catalog.DependencyNode, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return renderDependenciesJSON(out, tree)
	default:
		return renderDependenciesText(out, tree)
	}
}

func renderDependenciesText(out io.Writer, tree *catalog.DependencyNode) error {
	fmt.Fprintln(out, tree.Module)
	writeDepText(out, tree.Children, "")
	return nil
}

func writeDepText(out io.Writer, nodes []catalog.DependencyNode, prefix string) {
	for i, n := range nodes {
		last := i == len(nodes)-1
		branch := "├── "
		next := prefix + "│   "
		if last {
			branch = "└── "
			next = prefix + "    "
		}
		via := ""
		if n.EdgeFromParent.Variable != "" {
			via = fmt.Sprintf("  (via %s ← %s.%s)", n.EdgeFromParent.Variable, n.EdgeFromParent.To, n.EdgeFromParent.Output)
		}
		fmt.Fprintf(out, "%s%s%s%s\n", prefix, branch, n.Module, via)
		writeDepText(out, n.Children, next)
	}
}

// dependencyJSONNode is the JSON shape exported by the command. We keep
// it isolated from catalog.DependencyNode to evolve internal types
// without breaking the wire format.
type dependencyJSONNode struct {
	Module   string               `json:"module"`
	Depth    int                  `json:"depth"`
	Edge     *dependencyJSONEdge  `json:"edge,omitempty"`
	Children []dependencyJSONNode `json:"children,omitempty"`
}

type dependencyJSONEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Variable string `json:"variable"`
	Output   string `json:"output"`
}

func renderDependenciesJSON(out io.Writer, tree *catalog.DependencyNode) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(toDependencyJSON(tree))
}

func toDependencyJSON(n *catalog.DependencyNode) dependencyJSONNode {
	out := dependencyJSONNode{Module: n.Module, Depth: n.Depth}
	if n.EdgeFromParent.Variable != "" {
		out.Edge = &dependencyJSONEdge{
			From:     n.EdgeFromParent.From,
			To:       n.EdgeFromParent.To,
			Variable: n.EdgeFromParent.Variable,
			Output:   n.EdgeFromParent.Output,
		}
	}
	if len(n.Children) > 0 {
		out.Children = make([]dependencyJSONNode, len(n.Children))
		for i := range n.Children {
			out.Children[i] = toDependencyJSON(&n.Children[i])
		}
	}
	return out
}
