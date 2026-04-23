package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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
	filter    string
	all       bool
	outputDir string
	dryRun    bool
	force     bool
	format    string
	rootStack bool
}

// NewComposeCommand returns the `compose` subcommand. It generates one
// reusable Terraform module folder per selected catalog entry under
// --output-dir (or previews the plan in dry-run mode).
func NewComposeCommand() *cobra.Command {
	f := &composeFlags{rootStack: true}
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Generate reusable Terraform module folders from catalog entries",
		Long: `For each --modules entry, write a self-contained Terraform module folder
under --output-dir containing version.tf, variables.tf, main.tf,
outputs.tf and README.md. Each module wraps a single resource (or data
source) of the catalog provider.

Selection accepts bare names (defaults to the resource entry, falls
back to data) or kind-qualified names (resource.<name>, data.<name>)
to disambiguate when both exist.

Use --dry-run to preview the planned folders/files (paths + sha256)
without touching the filesystem. By default the command refuses to
overwrite pre-existing generated files unless --force is given.`,
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
			filterPatterns := splitModules(f.filter)
			if f.all {
				filterPatterns = []string{"*"}
			}
			if len(modules) == 0 && len(filterPatterns) == 0 {
				return invalidArgs("no modules selected",
					"pass --modules \"<name1> <name2>\", --filter \"<pattern>\", or --all")
			}
			if !f.dryRun && f.outputDir == "" {
				return invalidArgs("--output-dir is required unless --dry-run is set")
			}

			s, err := catalog.Load(schemaPath)
			if err != nil {
				return mapCatalogLoadError(schemaPath, err)
			}

			if len(filterPatterns) > 0 {
				matched, ferr := filterModules(s, filterPatterns)
				if ferr != nil {
					return invalidArgs(ferr.Error(), "check your --filter glob pattern")
				}
				modules = append(modules, matched...)
			}
			if len(modules) == 0 {
				return invalidArgs("no modules matched: filter patterns matched no catalog entries",
					"run `infra-composer search` to list available modules")
			}

			plan, err := terraform.Plan(s, terraform.PlanOptions{
				Modules:       modules,
				EmitRootStack: f.rootStack,
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
	cmd.Flags().StringVar(&f.modules, "modules", "", "Modules to compose (space- or comma-separated; e.g. \"aws_vpc data.aws_ami\")")
	cmd.Flags().StringVar(&f.filter, "filter", "", "Glob pattern(s) to select modules (space- or comma-separated; e.g. \"aws_s3*\" or \"aws_ec2*,data.aws_ami\")")
	cmd.Flags().BoolVar(&f.all, "all", false, "Select every module in the catalog (equivalent to --filter \"*\")")
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "Parent directory under which one folder per module is written")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "Print planned folders/files (paths + sha256) without writing")
	cmd.Flags().BoolVar(&f.force, "force", false, "Overwrite generated files in pre-existing module folders")
	cmd.Flags().StringVar(&f.format, "format", "", "Output format: text|json (overrides global --format)")
	cmd.Flags().BoolVar(&f.rootStack, "root-stack", true, "Also emit a top-level stack (providers.tf, versions.tf, variables.tf, locals.tf, main.tf, outputs.tf) that composes the per-module folders")
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

// filterModules returns the kind-qualified names (e.g. "resource.aws_vpc",
// "data.aws_ami") for every catalog entry whose bare name matches at least
// one of the supplied glob patterns. Patterns follow path.Match rules:
// "*" matches any sequence of characters (including "_"), "?" matches
// exactly one, and "[…]" matches a character class. Results are sorted
// and deduplicated. Patterns that match nothing are silently skipped;
// callers decide whether an empty result is an error.
func filterModules(s *catalog.Schema, patterns []string) ([]string, error) {
	if len(patterns) == 0 || s == nil {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(s.Modules))
	for i := range s.Modules {
		m := &s.Modules[i]
		for _, pat := range patterns {
			ok, err := path.Match(pat, m.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid filter pattern %q: %w", pat, err)
			}
			if ok {
				seen[string(m.Type)+"."+m.Name] = struct{}{}
				break
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func mapPlanError(err error) error {
	var cyc *catalog.CycleError
	switch {
	case errors.Is(err, catalog.ErrUnknownModule):
		return cliError(clierr.ExitModuleNotFound, err.Error(),
			"run `infra-composer search` to list available modules")
	case errors.Is(err, terraform.ErrAmbiguousSelection):
		return cliError(clierr.ExitInvalidArgs, err.Error(),
			"use the kind-qualified form (e.g. \"resource.<name>\" or \"data.<name>\")")
	case errors.As(err, &cyc):
		return cliError(clierr.ExitInvalidArgs, err.Error(),
			"break the cycle by removing one of the references, or disable the root stack with --root-stack=false")
	}
	return clierr.Wrap(clierr.ExitGeneric, "build compose plan", err)
}

// writeComposeFiles materialises files atomically. Per-module folders
// are created under dir; clash detection is per-file (any pre-existing
// file with the same relative path inside dir aborts the write unless
// force is set). Each file is written via tmp+rename so failures do
// not leave a half-updated module.
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
		full := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return clierr.Wrap(clierr.ExitGeneric, fmt.Sprintf("create folder for %s", f.Path), err)
		}
		if err := atomicWriteFile(full, f.Content); err != nil {
			return clierr.Wrap(clierr.ExitGeneric, fmt.Sprintf("write %s", f.Path), err)
		}
	}
	return nil
}

func detectClashes(dir string, files []terraform.GeneratedFile) ([]string, error) {
	out := make([]string, 0)
	for _, f := range files {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.Path)))
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

type composeJSONModule struct {
	Module   string            `json:"module"`
	Kind     string            `json:"kind"`
	Folder   string            `json:"folder"`
	Files    []composeJSONFile `json:"files"`
	Warnings []string          `json:"warnings,omitempty"`
}

type composeJSONRootStack struct {
	Files []composeJSONFile `json:"files"`
}

type composeJSONSummary struct {
	OutputDir string                `json:"output_dir,omitempty"`
	DryRun    bool                  `json:"dry_run"`
	Provider  string                `json:"provider,omitempty"`
	Modules   []composeJSONModule   `json:"modules"`
	RootStack *composeJSONRootStack `json:"root_stack,omitempty"`
	Warnings  []string              `json:"warnings,omitempty"`
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
	byModule := make(map[string][]composeJSONFile, len(plan.Modules))
	var rootFiles []composeJSONFile
	for _, f := range files {
		entry := composeJSONFile{
			Path:   f.Path,
			SHA256: f.SHA256Hex(),
			Bytes:  len(f.Content),
		}
		if f.Module == "" {
			rootFiles = append(rootFiles, entry)
			continue
		}
		byModule[f.Module] = append(byModule[f.Module], entry)
	}
	mods := make([]composeJSONModule, 0, len(plan.Modules))
	for _, m := range plan.Modules {
		mods = append(mods, composeJSONModule{
			Module:   m.ResourceType,
			Kind:     string(m.Kind),
			Folder:   m.ResourceType,
			Files:    byModule[m.ResourceType],
			Warnings: m.Warnings,
		})
	}
	summary := composeJSONSummary{
		OutputDir: dir,
		DryRun:    dryRun,
		Provider:  plan.Provider,
		Modules:   mods,
		Warnings:  plan.Warnings,
	}
	if plan.EmitRootStack && len(rootFiles) > 0 {
		summary.RootStack = &composeJSONRootStack{Files: rootFiles}
	}
	return summary
}

func encodeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderComposeText(out io.Writer, s composeJSONSummary) error {
	totalFiles := 0
	for _, m := range s.Modules {
		totalFiles += len(m.Files)
	}
	rootCount := 0
	if s.RootStack != nil {
		rootCount = len(s.RootStack.Files)
	}
	if s.DryRun {
		fmt.Fprintf(out, "Dry-run: %d module(s), %d file(s) planned (+ %d root-stack file(s)).\n", len(s.Modules), totalFiles, rootCount)
	} else {
		fmt.Fprintf(out, "Wrote %d module(s), %d file(s) to %s (+ %d root-stack file(s)).\n", len(s.Modules), totalFiles, s.OutputDir, rootCount)
	}
	for _, m := range s.Modules {
		fmt.Fprintf(out, "\n%s/  (%s)\n", m.Folder, m.Kind)
		for _, f := range m.Files {
			fmt.Fprintf(out, "  %s  %s  (%d bytes)\n", f.Path, f.SHA256[:12], f.Bytes)
		}
		for _, w := range m.Warnings {
			fmt.Fprintf(out, "  ! %s\n", w)
		}
	}
	if s.RootStack != nil && len(s.RootStack.Files) > 0 {
		fmt.Fprintln(out, "\nRoot stack")
		for _, f := range s.RootStack.Files {
			fmt.Fprintf(out, "  %s  %s  (%d bytes)\n", f.Path, f.SHA256[:12], f.Bytes)
		}
	}
	if len(s.Warnings) > 0 {
		fmt.Fprintln(out, "\nPlan warnings:")
		for _, w := range s.Warnings {
			fmt.Fprintf(out, "  ! %s\n", w)
		}
	}
	return nil
}
