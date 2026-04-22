package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
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
