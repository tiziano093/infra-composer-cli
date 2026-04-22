package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

type interfaceFlags struct {
	schema       string
	requiredOnly bool
	full         bool
	format       string
}

// NewInterfaceCommand returns the `interface <modules...>` subcommand.
// It prints the composer-facing input/output surface of the requested
// modules so users (and LLMs) can decide which values they need to
// provide and which outputs the resulting stack will expose.
func NewInterfaceCommand() *cobra.Command {
	f := &interfaceFlags{}
	cmd := &cobra.Command{
		Use:   "interface <modules...>",
		Short: "Show inputs and outputs of one or more catalog modules",
		Long: `Inspect the public interface of the requested modules: required and
optional inputs, and exposed outputs. Inputs that are auto-wired between
modules in the selection (via Variable.references) are flagged as wired
and hidden from the aggregate view by default.

Use --required-only for the minimal user-facing surface (drops optional
and wired inputs), or --full to show wired inputs alongside everything
else.`,
		Args:          cobra.MinimumNArgs(1),
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

			if f.requiredOnly && f.full {
				return invalidArgs("--required-only and --full are mutually exclusive")
			}

			iface, err := catalog.ExtractInterface(s, catalog.ExtractOptions{
				Modules:      args,
				RequiredOnly: f.requiredOnly,
				Full:         f.full,
			})
			if err != nil {
				if errors.Is(err, catalog.ErrUnknownModule) {
					return cliError(clierr.ExitModuleNotFound, err.Error(),
						"run `infra-composer search` to list available modules")
				}
				return err
			}

			format := chooseFormat(f.format, rt)
			return renderInterface(out, iface, format)
		},
	}

	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (defaults to catalog.schema in config)")
	cmd.Flags().BoolVar(&f.requiredOnly, "required-only", false, "Show only required inputs (drops optional + auto-wired)")
	cmd.Flags().BoolVar(&f.full, "full", false, "Include auto-wired inputs in the aggregate view")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json|yaml (overrides global --format)")
	return cmd
}

func renderInterface(out io.Writer, iface *catalog.Interface, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return renderInterfaceStructured(out, iface, json.NewEncoder(out), true)
	case "yaml", "yml":
		return renderInterfaceYAML(out, iface)
	default:
		return renderInterfaceText(out, iface)
	}
}

func renderInterfaceText(out io.Writer, iface *catalog.Interface) error {
	for i, mi := range iface.Modules {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "module %s\n", mi.Module)
		if len(mi.Inputs) == 0 {
			fmt.Fprintln(out, "  inputs: (none)")
		} else {
			fmt.Fprintln(out, "  inputs:")
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "    NAME\tTYPE\tREQUIRED\tWIRED\tDESCRIPTION")
			for _, in := range mi.Inputs {
				fmt.Fprintf(tw, "    %s\t%s\t%v\t%s\t%s\n",
					in.Name, in.Type, in.Required, wiredLabel(in), truncate(in.Description, 60))
			}
			_ = tw.Flush()
		}
		if len(mi.Outputs) == 0 {
			fmt.Fprintln(out, "  outputs: (none)")
		} else {
			fmt.Fprintln(out, "  outputs:")
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "    NAME\tSENSITIVE\tDESCRIPTION")
			for _, o := range mi.Outputs {
				fmt.Fprintf(tw, "    %s\t%v\t%s\n", o.Name, o.Sensitive, truncate(o.Description, 60))
			}
			_ = tw.Flush()
		}
	}
	return nil
}

func wiredLabel(in catalog.InputView) string {
	if !in.Wired || in.Source == nil {
		return "-"
	}
	return in.Source.Module + "." + in.Source.Output
}

// interfaceJSONInput / interfaceJSONOutput / interfaceJSONModule form
// the wire shape used by both the JSON and YAML encoders so the two
// formats stay aligned automatically.
type interfaceJSONInput struct {
	Module      string `json:"module" yaml:"module"`
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required" yaml:"required"`
	Sensitive   bool   `json:"sensitive,omitempty" yaml:"sensitive,omitempty"`
	Default     any    `json:"default,omitempty" yaml:"default,omitempty"`
	Wired       bool   `json:"wired" yaml:"wired"`
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`
}

type interfaceJSONOutput struct {
	Module      string `json:"module" yaml:"module"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty" yaml:"sensitive,omitempty"`
}

type interfaceJSONModule struct {
	Module  string                `json:"module" yaml:"module"`
	Inputs  []interfaceJSONInput  `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs []interfaceJSONOutput `json:"outputs,omitempty" yaml:"outputs,omitempty"`
}

type interfaceJSON struct {
	Modules    []interfaceJSONModule `json:"modules" yaml:"modules"`
	AllInputs  []interfaceJSONInput  `json:"all_inputs" yaml:"all_inputs"`
	AllOutputs []interfaceJSONOutput `json:"all_outputs" yaml:"all_outputs"`
}

func toInterfaceJSON(iface *catalog.Interface) interfaceJSON {
	mapInput := func(in catalog.InputView) interfaceJSONInput {
		v := interfaceJSONInput{
			Module: in.Module, Name: in.Name, Type: in.Type,
			Description: in.Description, Required: in.Required,
			Sensitive: in.Sensitive, Default: in.Default, Wired: in.Wired,
		}
		if in.Source != nil {
			v.Source = in.Source.Module + "." + in.Source.Output
		}
		return v
	}
	mapOutput := func(o catalog.OutputView) interfaceJSONOutput {
		return interfaceJSONOutput{
			Module: o.Module, Name: o.Name,
			Description: o.Description, Sensitive: o.Sensitive,
		}
	}
	out := interfaceJSON{
		Modules:    make([]interfaceJSONModule, 0, len(iface.Modules)),
		AllInputs:  make([]interfaceJSONInput, 0, len(iface.AllInputs)),
		AllOutputs: make([]interfaceJSONOutput, 0, len(iface.AllOutputs)),
	}
	for _, mi := range iface.Modules {
		mod := interfaceJSONModule{Module: mi.Module}
		for _, in := range mi.Inputs {
			mod.Inputs = append(mod.Inputs, mapInput(in))
		}
		for _, o := range mi.Outputs {
			mod.Outputs = append(mod.Outputs, mapOutput(o))
		}
		out.Modules = append(out.Modules, mod)
	}
	for _, in := range iface.AllInputs {
		out.AllInputs = append(out.AllInputs, mapInput(in))
	}
	for _, o := range iface.AllOutputs {
		out.AllOutputs = append(out.AllOutputs, mapOutput(o))
	}
	return out
}

func renderInterfaceStructured(out io.Writer, iface *catalog.Interface, enc *json.Encoder, indent bool) error {
	if indent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(toInterfaceJSON(iface))
}

func renderInterfaceYAML(out io.Writer, iface *catalog.Interface) error {
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(toInterfaceJSON(iface))
}
