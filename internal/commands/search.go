package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// searchFlags collects the local flags accepted by the `search` command.
type searchFlags struct {
	schema string
	group  string
	typ    string
	limit  int
	format string
}

// NewSearchCommand returns the `search` subcommand which queries a
// catalog schema for modules matching the provided keywords.
func NewSearchCommand() *cobra.Command {
	f := &searchFlags{}
	cmd := &cobra.Command{
		Use:   "search [keywords...]",
		Short: "Search the catalog for modules matching the given keywords",
		Long: `Search modules in a catalog schema using AND keyword logic.
Keywords match (case-insensitive) against the module name, group and
description; results are ranked with name matches outranking group and
description matches. Use --group and --type to narrow the result set.`,
		Args:          cobra.ArbitraryArgs,
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

			typ, err := parseModuleType(f.typ)
			if err != nil {
				return invalidArgs(err.Error(),
					"valid values: resource, data (or empty for any)")
			}

			results := catalog.Search(s, catalog.SearchOptions{
				Keywords: args,
				Group:    f.group,
				Type:     typ,
				Limit:    f.limit,
			})

			format := chooseFormat(f.format, rt)
			return renderSearchResults(out, results, format)
		},
	}

	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (defaults to catalog.schema in config)")
	cmd.Flags().StringVar(&f.group, "group", "", "Filter by module group (e.g. network, compute)")
	cmd.Flags().StringVar(&f.typ, "type", "", "Filter by module type: resource|data")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Maximum number of results (0 = no limit)")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: table|json (overrides global --format)")
	return cmd
}

// renderSearchResults dispatches on format. Empty result sets still
// produce a header (table) or an empty array (JSON) so downstream
// tooling can rely on a stable shape.
func renderSearchResults(out io.Writer, results []catalog.SearchResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return renderSearchJSON(out, results)
	default:
		return renderSearchTable(out, results)
	}
}

func renderSearchTable(out io.Writer, results []catalog.SearchResult) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tGROUP\tSCORE\tDESCRIPTION")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			r.Module.Name,
			string(r.Module.Type),
			dashIfEmpty(r.Module.Group),
			r.Score,
			truncate(r.Module.Description, 60),
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "(no matches)")
	}
	return nil
}

// searchJSONEntry is the on-wire shape for JSON output. It is kept
// distinct from catalog.SearchResult so we can evolve internal types
// without breaking the JSON contract.
type searchJSONEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Score       int    `json:"score"`
}

func renderSearchJSON(out io.Writer, results []catalog.SearchResult) error {
	payload := make([]searchJSONEntry, 0, len(results))
	for _, r := range results {
		payload = append(payload, searchJSONEntry{
			Name:        r.Module.Name,
			Type:        string(r.Module.Type),
			Group:       r.Module.Group,
			Description: r.Module.Description,
			Source:      r.Module.Source,
			Score:       r.Score,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// parseModuleType validates the --type flag value. Empty input is
// treated as "no filter" and yields an empty ModuleType.
func parseModuleType(s string) (catalog.ModuleType, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return "", nil
	case "resource":
		return catalog.ModuleTypeResource, nil
	case "data":
		return catalog.ModuleTypeData, nil
	default:
		return "", fmt.Errorf("invalid module type %q", s)
	}
}

// resolveSchemaPath picks the schema location: explicit flag wins, then
// config (when a runtime is available), otherwise empty.
func resolveSchemaPath(flag string, rt Runtime) string {
	if flag != "" {
		return flag
	}
	if rt.Config != nil {
		return rt.Config.Catalog.SchemaPath
	}
	return ""
}

// chooseFormat applies the precedence: command flag > global config > "table".
func chooseFormat(flag string, rt Runtime) string {
	if flag != "" {
		return flag
	}
	if rt.Config != nil && rt.Config.OutputFormat != "" {
		return rt.Config.OutputFormat
	}
	return "table"
}

func writerOrStdout(primary, fallback io.Writer) io.Writer {
	if primary != nil {
		return primary
	}
	return fallback
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// truncate shortens long descriptions for table layout. Cuts on rune
// boundaries via string slicing on bytes; descriptions are ASCII-ish in
// practice, but we still guard against mid-multibyte cuts by falling
// back to the original string when slicing would split a rune.
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := s[:max]
	// Defend against splitting a multi-byte rune by trimming any
	// trailing continuation bytes (10xxxxxx).
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}

// mapCatalogLoadError converts catalog.Load errors into CLIError values
// with appropriate exit codes and remediation hints. We import the cli
// package indirectly through error helpers defined below to avoid a
// cycle (cli -> commands -> cli).
func mapCatalogLoadError(path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return cliError(exitFileNotFound,
			fmt.Sprintf("catalog schema not found: %s", path),
			"run `infra-composer catalog build` first, or pass --schema <path>")
	}
	if ve, ok := catalog.AsValidationError(err); ok {
		details := make([]string, 0, len(ve.Issues))
		for _, iss := range ve.Issues {
			details = append(details, "  - "+iss.String())
		}
		return cliError(exitValidationFailed,
			fmt.Sprintf("catalog schema is invalid: %s\n%s", path, strings.Join(details, "\n")),
			"run `infra-composer catalog validate` to inspect all issues")
	}
	var pe *catalog.ParseError
	if errors.As(err, &pe) {
		return cliError(exitValidationFailed,
			fmt.Sprintf("failed to parse catalog schema %s: %v", path, pe.Cause),
			"verify the file is valid JSON matching the documented schema")
	}
	return err
}
