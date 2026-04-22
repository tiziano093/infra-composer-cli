package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

// NewCatalogCommand returns the parent `catalog` command. Subcommands
// are added here so the cli root only needs to register the parent.
func NewCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage catalog schemas (build, validate, list, export)",
		Long: `Operate on catalog schemas: build a new catalog from a Terraform
provider, validate an existing schema, list its contents, or export it
to a different location.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newCatalogValidateCommand())
	cmd.AddCommand(newCatalogBuildCommand())
	cmd.AddCommand(newCatalogListCommand())
	cmd.AddCommand(newCatalogExportCommand())
	return cmd
}

type validateFlags struct {
	schema string
	format string
}

// newCatalogValidateCommand returns `catalog validate [path]` which
// loads a schema and reports every validation issue. Exit code 0 means
// the schema is valid; ExitValidationFailed signals a structural error.
func newCatalogValidateCommand() *cobra.Command {
	f := &validateFlags{}
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a catalog schema and report all issues",
		Long: `Parse and validate a catalog schema. Reports every issue in a single
pass so they can be fixed together. The path argument takes precedence
over the --schema flag and the catalog.schema config value.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, _ := FromContext(cmd.Context())
			out := writerOrStdout(rt.Stdout, cmd.OutOrStdout())

			path := f.schema
			if len(args) == 1 {
				path = args[0]
			}
			if path == "" {
				path = resolveSchemaPath("", rt)
			}
			if path == "" {
				return invalidArgs("no catalog schema path provided",
					"pass a positional path, --schema <path>, or set catalog.schema in config")
			}

			s, err := catalog.Load(path)
			format := chooseFormat(f.format, rt)
			if err != nil {
				return renderValidateError(out, path, err, format)
			}
			return renderValidateOK(out, path, s, format)
		},
	}
	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (alternative to positional arg)")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}

// validateReport is the JSON shape for `catalog validate` output.
type validateReport struct {
	Path    string                  `json:"path"`
	Valid   bool                    `json:"valid"`
	Modules int                     `json:"modules"`
	Issues  []validateReportIssue   `json:"issues,omitempty"`
	Schema  *validateReportMetadata `json:"schema,omitempty"`
}

type validateReportIssue struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type validateReportMetadata struct {
	SchemaVersion   string `json:"schema_version"`
	Provider        string `json:"provider"`
	ProviderVersion string `json:"provider_version"`
}

func renderValidateOK(out io.Writer, path string, s *catalog.Schema, format string) error {
	report := validateReport{
		Path:    path,
		Valid:   true,
		Modules: len(s.Modules),
		Schema: &validateReportMetadata{
			SchemaVersion:   s.SchemaVersion,
			Provider:        s.Provider,
			ProviderVersion: s.ProviderVersion,
		},
	}
	if strings.ToLower(format) == "json" {
		return writeJSON(out, report)
	}
	fmt.Fprintf(out, "OK %s\n  provider:        %s@%s\n  schema_version:  %s\n  modules:         %d\n",
		path, s.Provider, s.ProviderVersion, s.SchemaVersion, len(s.Modules))
	return nil
}

// renderValidateError prints (or JSON-encodes) the failure and returns
// a CLIError so the cli root maps it to the correct exit code.
func renderValidateError(out io.Writer, path string, err error, format string) error {
	if errors.Is(err, os.ErrNotExist) {
		return cliError(exitFileNotFound,
			fmt.Sprintf("catalog schema not found: %s", path),
			"check the path or run `infra-composer catalog build` first")
	}
	var pe *catalog.ParseError
	if errors.As(err, &pe) {
		report := validateReport{Path: path, Valid: false, Issues: []validateReportIssue{{
			Message: fmt.Sprintf("parse error: %v", pe.Cause),
		}}}
		if strings.ToLower(format) == "json" {
			_ = writeJSON(out, report)
		} else {
			fmt.Fprintf(out, "INVALID %s\n  parse error: %v\n", path, pe.Cause)
		}
		return cliError(exitValidationFailed,
			fmt.Sprintf("catalog schema is not valid JSON: %s", path),
			"verify the file is valid JSON matching the documented schema")
	}
	if ve, ok := catalog.AsValidationError(err); ok {
		report := validateReport{Path: path, Valid: false, Issues: make([]validateReportIssue, len(ve.Issues))}
		for i, iss := range ve.Issues {
			report.Issues[i] = validateReportIssue{Field: iss.Field, Message: iss.Message}
		}
		if strings.ToLower(format) == "json" {
			_ = writeJSON(out, report)
		} else {
			fmt.Fprintf(out, "INVALID %s\n", path)
			for _, iss := range ve.Issues {
				fmt.Fprintf(out, "  - %s\n", iss.String())
			}
		}
		return cliError(exitValidationFailed,
			fmt.Sprintf("catalog schema has %d validation issue(s): %s", len(ve.Issues), path))
	}
	return err
}

func writeJSON(out io.Writer, payload any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// ----------------------------------------------------------------------------
// catalog build
// ----------------------------------------------------------------------------

type buildFlags struct {
	provider    string
	outputDir   string
	registryDir string
	format      string
}

// DefaultRegistryDir is the relative directory consulted by the FakeClient
// when --registry-dir is not provided. It mirrors the layout used by the
// integration test fixtures so contributors can experiment locally.
const DefaultRegistryDir = "./catalog/registry"

// newCatalogBuildCommand returns `catalog build --provider <addr>
// --output-dir <dir>` which materialises a fresh schema.json by walking
// the registry fixtures under --registry-dir.
func newCatalogBuildCommand() *cobra.Command {
	f := &buildFlags{}
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a catalog schema from a provider's registry data",
		Long: `Build a catalog schema for the given Terraform provider. Provider
metadata is loaded from the registry fixtures directory (defaults to
./catalog/registry). The resulting schema.json is written to
<output-dir>/schema.json and validated before exit.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, _ := FromContext(cmd.Context())
			out := writerOrStdout(rt.Stdout, cmd.OutOrStdout())
			if f.provider == "" {
				return invalidArgs("--provider is required",
					"example: infra-composer catalog build --provider hashicorp/aws --output-dir ./catalog")
			}
			if f.outputDir == "" {
				return invalidArgs("--output-dir is required",
					"the resulting schema.json will be written to <output-dir>/schema.json")
			}
			registryDir := f.registryDir
			if registryDir == "" {
				registryDir = DefaultRegistryDir
			}

			client := registry.NewFakeClient(registryDir)
			builder := catalog.NewBuilder(client)
			s, err := builder.Build(cmd.Context(), catalog.BuildOptions{
				Provider: f.provider,
				Now:      func() time.Time { return time.Now().UTC() },
			})
			if err != nil {
				return mapBuildError(f.provider, registryDir, err)
			}

			dest, err := catalog.Export(s, catalog.ExportOptions{Dir: f.outputDir})
			if err != nil {
				return cliError(clierr.ExitGeneric,
					fmt.Sprintf("failed to export catalog: %v", err),
					"check that --output-dir is writable")
			}
			return renderBuildResult(out, s, dest, chooseFormat(f.format, rt))
		},
	}
	cmd.Flags().StringVar(&f.provider, "provider", "", "Provider address (e.g. hashicorp/aws). Required.")
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "Directory where schema.json will be written. Required.")
	cmd.Flags().StringVar(&f.registryDir, "registry-dir", "", "Directory containing provider fixtures (default ./catalog/registry)")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}

// buildReport is the JSON shape for `catalog build` output.
type buildReport struct {
	Provider        string `json:"provider"`
	ProviderVersion string `json:"provider_version"`
	SchemaPath      string `json:"schema_path"`
	Modules         int    `json:"modules"`
	GeneratedAt     string `json:"generated_at,omitempty"`
}

func renderBuildResult(out io.Writer, s *catalog.Schema, dest, format string) error {
	report := buildReport{
		Provider:        s.Provider,
		ProviderVersion: s.ProviderVersion,
		SchemaPath:      dest,
		Modules:         len(s.Modules),
	}
	if !s.GeneratedAt.IsZero() {
		report.GeneratedAt = s.GeneratedAt.UTC().Format(time.RFC3339)
	}
	if strings.ToLower(format) == "json" {
		return writeJSON(out, report)
	}
	fmt.Fprintf(out, "Built catalog for %s@%s (%d modules)\n  written to: %s\n",
		s.Provider, s.ProviderVersion, len(s.Modules), dest)
	return nil
}

// mapBuildError converts builder/registry failures into CLIError values
// with stable exit codes and remediation hints.
func mapBuildError(provider, registryDir string, err error) error {
	if errors.Is(err, registry.ErrProviderNotFound) {
		return cliError(exitFileNotFound,
			fmt.Sprintf("provider %s not found under %s", provider, registryDir),
			fmt.Sprintf("add fixtures at %s/<namespace>/<name>/provider.json or pass --registry-dir <dir>", registryDir))
	}
	if errors.Is(err, registry.ErrResourceNotFound) {
		return cliError(exitFileNotFound,
			fmt.Sprintf("registry fixture for %s is missing a referenced resource: %v", provider, err),
			"check the resources block of provider.json against ListResources output")
	}
	var ve *catalog.ValidationError
	if errors.As(err, &ve) {
		return cliError(exitValidationFailed,
			fmt.Sprintf("registry produced invalid catalog for %s (%d issue(s))", provider, len(ve.Issues)),
			"verify the provider fixture matches the catalog schema")
	}
	return cliError(clierr.ExitGeneric,
		fmt.Sprintf("failed to build catalog for %s: %v", provider, err))
}

// ----------------------------------------------------------------------------
// catalog list
// ----------------------------------------------------------------------------

type listFlags struct {
	schema string
	format string
	group  string
}

// newCatalogListCommand returns `catalog list [path]` which prints the
// modules contained in a schema along with provider metadata.
func newCatalogListCommand() *cobra.Command {
	f := &listFlags{}
	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List modules contained in a catalog schema",
		Long: `Load a catalog schema and list its provider metadata plus every
module entry. The path argument takes precedence over --schema and the
catalog.schema config value.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, _ := FromContext(cmd.Context())
			out := writerOrStdout(rt.Stdout, cmd.OutOrStdout())

			path := f.schema
			if len(args) == 1 {
				path = args[0]
			}
			if path == "" {
				path = resolveSchemaPath("", rt)
			}
			if path == "" {
				return invalidArgs("no catalog schema path provided",
					"pass a positional path, --schema <path>, or set catalog.schema in config")
			}
			s, err := catalog.Load(path)
			if err != nil {
				return mapCatalogLoadError(path, err)
			}
			modules := filterByGroup(s.Modules, f.group)
			return renderList(out, path, s, modules, chooseFormat(f.format, rt))
		},
	}
	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (alternative to positional arg)")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: table|json (overrides global --format)")
	cmd.Flags().StringVar(&f.group, "group", "", "Filter modules by group (e.g. network, compute)")
	return cmd
}

type listReport struct {
	Path            string            `json:"path"`
	Provider        string            `json:"provider"`
	ProviderVersion string            `json:"provider_version"`
	SchemaVersion   string            `json:"schema_version"`
	Modules         int               `json:"modules"`
	GeneratedAt     string            `json:"generated_at,omitempty"`
	Entries         []listReportEntry `json:"entries"`
}

type listReportEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
}

func renderList(out io.Writer, path string, s *catalog.Schema, modules []catalog.ModuleEntry, format string) error {
	if strings.ToLower(format) == "json" {
		report := listReport{
			Path:            path,
			Provider:        s.Provider,
			ProviderVersion: s.ProviderVersion,
			SchemaVersion:   s.SchemaVersion,
			Modules:         len(modules),
			Entries:         make([]listReportEntry, len(modules)),
		}
		if !s.GeneratedAt.IsZero() {
			report.GeneratedAt = s.GeneratedAt.UTC().Format(time.RFC3339)
		}
		for i, m := range modules {
			report.Entries[i] = listReportEntry{
				Name: m.Name, Type: string(m.Type), Group: m.Group,
				Description: m.Description, Source: m.Source,
			}
		}
		return writeJSON(out, report)
	}

	fmt.Fprintf(out, "%s@%s — %d module(s)\n", s.Provider, s.ProviderVersion, len(modules))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tGROUP\tDESCRIPTION")
	for _, m := range modules {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			m.Name, string(m.Type), dashIfEmpty(m.Group), truncate(m.Description, 60))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(modules) == 0 {
		fmt.Fprintln(out, "(no modules)")
	}
	return nil
}

func filterByGroup(modules []catalog.ModuleEntry, group string) []catalog.ModuleEntry {
	if group == "" {
		return modules
	}
	out := make([]catalog.ModuleEntry, 0, len(modules))
	for _, m := range modules {
		if strings.EqualFold(m.Group, group) {
			out = append(out, m)
		}
	}
	return out
}

// ----------------------------------------------------------------------------
// catalog export
// ----------------------------------------------------------------------------

type exportFlags struct {
	schema string
	output string
	format string
}

// newCatalogExportCommand returns `catalog export [path] --output <file>`
// which re-serialises a schema (after validation) to a new location.
// Useful for converting between checkout layouts and for normalising
// hand-edited schemas.
func newCatalogExportCommand() *cobra.Command {
	f := &exportFlags{}
	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Re-serialise a catalog schema to a new location",
		Long: `Load a catalog schema, validate it, and write it back out using the
canonical formatter. The destination is taken from --output and may be
either a file path or a directory (in which case schema.json is appended).`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, _ := FromContext(cmd.Context())
			out := writerOrStdout(rt.Stdout, cmd.OutOrStdout())

			path := f.schema
			if len(args) == 1 {
				path = args[0]
			}
			if path == "" {
				path = resolveSchemaPath("", rt)
			}
			if path == "" {
				return invalidArgs("no source catalog schema provided",
					"pass a positional path, --schema <path>, or set catalog.schema in config")
			}
			if f.output == "" {
				return invalidArgs("--output is required",
					"pass --output <file> or --output <dir>")
			}
			s, err := catalog.Load(path)
			if err != nil {
				return mapCatalogLoadError(path, err)
			}
			opts, err := exportOptionsFor(f.output)
			if err != nil {
				return invalidArgs(err.Error())
			}
			dest, err := catalog.Export(s, opts)
			if err != nil {
				return cliError(clierr.ExitGeneric,
					fmt.Sprintf("failed to export catalog: %v", err),
					"check that --output is writable")
			}
			return renderExport(out, path, dest, s, chooseFormat(f.format, rt))
		},
	}
	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to source catalog schema.json (alternative to positional arg)")
	cmd.Flags().StringVar(&f.output, "output", "", "Destination file or directory (required)")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}

type exportReport struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Provider    string `json:"provider"`
	Modules     int    `json:"modules"`
}

func renderExport(out io.Writer, source, dest string, s *catalog.Schema, format string) error {
	report := exportReport{Source: source, Destination: dest, Provider: s.Provider, Modules: len(s.Modules)}
	if strings.ToLower(format) == "json" {
		return writeJSON(out, report)
	}
	fmt.Fprintf(out, "Exported %s -> %s (%d modules)\n", source, dest, len(s.Modules))
	return nil
}

// exportOptionsFor decides whether --output is a file or a directory.
// Existing directories are treated as Dir (so schema.json is appended);
// anything else (including non-existent paths) is treated as Path. The
// "trailing-slash means dir" convention is also honoured for paths that
// do not exist yet.
func exportOptionsFor(output string) (catalog.ExportOptions, error) {
	if output == "" {
		return catalog.ExportOptions{}, fmt.Errorf("--output is required")
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) || strings.HasSuffix(output, "/") {
		return catalog.ExportOptions{Dir: strings.TrimRight(output, "/"+string(os.PathSeparator))}, nil
	}
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return catalog.ExportOptions{Dir: output}, nil
	}
	return catalog.ExportOptions{Path: output}, nil
}

// (no extra local helpers needed; we use clierr.ExitGeneric directly)

// Compile-time: ensure imports referenced for godoc stay used.
var _ = context.Background
