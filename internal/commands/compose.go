package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
	"github.com/tiziano093/infra-composer-cli/internal/terraform"
)

type composeFlags struct {
	schema    string
	modules   string
	outputDir string
	dryRun    bool
	force     bool
	format    string
}

// NewComposeCommand returns the `compose` subcommand which renders a
// Terraform stack from the catalog and writes it to disk (or previews
// the result in dry-run mode).
func NewComposeCommand() *cobra.Command {
	f := &composeFlags{}
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Compose a Terraform stack from catalog modules",
		Long: `Resolve module dependencies, choose sources, and render the five core
Terraform files (providers.tf, variables.tf, locals.tf, main.tf,
outputs.tf) into --output-dir.

Use --dry-run to preview the planned files (paths + sha256) without
touching the filesystem. By default the command refuses to write into
a non-empty directory unless --force is given.`,
		Args:          cobra.NoArgs,
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
			modules := splitModules(f.modules)
			if len(modules) == 0 {
				return invalidArgs("no modules selected",
					"pass --modules \"<name1> <name2>\" or a comma-separated list")
			}
			if !f.dryRun && f.outputDir == "" {
				return invalidArgs("--output-dir is required unless --dry-run is set")
			}

			s, err := catalog.Load(schemaPath)
			if err != nil {
				return mapCatalogLoadError(schemaPath, err)
			}

			plan, err := terraform.Plan(s, terraform.PlanOptions{
				Modules:  modules,
				Resolver: terraform.PlaceholderResolver{},
			})
			if err != nil {
				return mapPlanError(err)
			}

			files, err := terraform.Generate(plan)
			if err != nil {
				return clierr.Wrap(clierr.ExitGeneric, "generate terraform files", err)
			}

			format := chooseFormat(f.format, rt)
			if f.dryRun {
				return renderComposeDryRun(out, plan, files, format)
			}
			if err := writeComposeFiles(f.outputDir, files, f.force); err != nil {
				return err
			}
			return renderComposeWriteSummary(out, plan, files, f.outputDir, format)
		},
	}

	cmd.Flags().StringVar(&f.schema, "schema", "", "Path to catalog schema.json (defaults to catalog.schema in config)")
	cmd.Flags().StringVar(&f.modules, "modules", "", "Modules to compose (space- or comma-separated, e.g. \"aws_vpc aws_subnet\")")
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "Directory to write generated files into")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "Print planned files (paths + sha256) without writing")
	cmd.Flags().BoolVar(&f.force, "force", false, "Overwrite generated files in a non-empty output-dir")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}

// splitModules accepts both space- and comma-separated lists and trims
// blanks so users can pass --modules "a b" or "a,b" interchangeably.
func splitModules(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func mapPlanError(err error) error {
	switch {
	case errors.Is(err, catalog.ErrUnknownModule):
		return cliError(clierr.ExitModuleNotFound, err.Error(),
			"run `infra-composer search` to list available modules")
	case errors.Is(err, terraform.ErrAmbiguousReference):
		return cliError(clierr.ExitDependencyFailed, err.Error(),
			"either narrow the module selection or remove one of the conflicting references in the catalog")
	}
	if strings.Contains(err.Error(), "dependency cycle") {
		return cliError(clierr.ExitDependencyFailed, err.Error(),
			"run `infra-composer dependencies <module> --check-cycles` to enumerate cycles")
	}
	return clierr.Wrap(clierr.ExitGeneric, "build compose plan", err)
}

// writeComposeFiles materialises files atomically. When the target dir
// does not yet exist it is created. For an existing dir we refuse
// non-empty overwrites unless --force is set; in that mode we write
// each file via tmp+rename so a failed write does not leave a half
// updated stack.
func writeComposeFiles(dir string, files []terraform.GeneratedFile, force bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return clierr.Wrap(clierr.ExitGeneric, "create output directory", err)
	}
	if !force {
		clashes, err := detectClashes(dir, files)
		if err != nil {
			return clierr.Wrap(clierr.ExitGeneric, "inspect output directory", err)
		}
		if len(clashes) > 0 {
			return cliError(clierr.ExitGeneric,
				fmt.Sprintf("output directory %s already contains generated files: %s", dir, strings.Join(clashes, ", ")),
				"re-run with --force to overwrite the listed files")
		}
	}
	for _, f := range files {
		full := filepath.Join(dir, f.Path)
		if err := atomicWriteFile(full, f.Content); err != nil {
			return clierr.Wrap(clierr.ExitGeneric, fmt.Sprintf("write %s", f.Path), err)
		}
	}
	return nil
}

func detectClashes(dir string, files []terraform.GeneratedFile) ([]string, error) {
	out := make([]string, 0)
	for _, f := range files {
		_, err := os.Stat(filepath.Join(dir, f.Path))
		switch {
		case err == nil:
			out = append(out, f.Path)
		case errors.Is(err, os.ErrNotExist):
			// fine
		default:
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func atomicWriteFile(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".infra-composer-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup if rename never happens.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// composeJSONFile is the on-wire shape for dry-run / write summaries.
type composeJSONFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type composeJSONSummary struct {
	OutputDir string            `json:"output_dir,omitempty"`
	DryRun    bool              `json:"dry_run"`
	Modules   []string          `json:"modules"`
	Files     []composeJSONFile `json:"files"`
	Warnings  []string          `json:"warnings,omitempty"`
}

func renderComposeDryRun(out io.Writer, plan *terraform.ComposePlan, files []terraform.GeneratedFile, format string) error {
	summary := composeSummary("", true, plan, files)
	switch strings.ToLower(format) {
	case "json":
		return encodeJSON(out, summary)
	default:
		return renderComposeText(out, summary)
	}
}

func renderComposeWriteSummary(out io.Writer, plan *terraform.ComposePlan, files []terraform.GeneratedFile, dir, format string) error {
	summary := composeSummary(dir, false, plan, files)
	switch strings.ToLower(format) {
	case "json":
		return encodeJSON(out, summary)
	default:
		return renderComposeText(out, summary)
	}
}

func composeSummary(dir string, dryRun bool, plan *terraform.ComposePlan, files []terraform.GeneratedFile) composeJSONSummary {
	mods := make([]string, 0, len(plan.Modules))
	for _, m := range plan.Modules {
		mods = append(mods, m.Module.Name)
	}
	jf := make([]composeJSONFile, 0, len(files))
	for _, f := range files {
		jf = append(jf, composeJSONFile{Path: f.Path, SHA256: f.SHA256Hex(), Bytes: len(f.Content)})
	}
	return composeJSONSummary{
		OutputDir: dir,
		DryRun:    dryRun,
		Modules:   mods,
		Files:     jf,
		Warnings:  plan.Warnings,
	}
}

func encodeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderComposeText(out io.Writer, s composeJSONSummary) error {
	if s.DryRun {
		fmt.Fprintf(out, "Dry-run: %d module(s), %d file(s) planned.\n", len(s.Modules), len(s.Files))
	} else {
		fmt.Fprintf(out, "Wrote %d file(s) to %s\n", len(s.Files), s.OutputDir)
	}
	if len(s.Modules) > 0 {
		fmt.Fprintln(out, "Modules (deps first):")
		for _, m := range s.Modules {
			fmt.Fprintf(out, "  - %s\n", m)
		}
	}
	if len(s.Files) > 0 {
		fmt.Fprintln(out, "Files:")
		for _, f := range s.Files {
			fmt.Fprintf(out, "  %s  %s  (%d bytes)\n", f.Path, f.SHA256[:12], f.Bytes)
		}
	}
	if len(s.Warnings) > 0 {
		fmt.Fprintln(out, "Warnings:")
		for _, w := range s.Warnings {
			fmt.Fprintf(out, "  ! %s\n", w)
		}
	}
	return nil
}
