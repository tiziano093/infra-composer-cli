package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

// presetProviders are the addresses suggested by the interactive picker
// before falling back to a freeform prompt. Order matters: the most
// commonly used providers come first.
var presetProviders = []string{
	"hashicorp/random",
	"hashicorp/aws",
	"hashicorp/azurerm",
	"hashicorp/google",
	"hashicorp/kubernetes",
	"hashicorp/null",
	"hashicorp/local",
	"Other (type address)",
}

// interactiveFlags collects the flags accepted by `infra-composer
// interactive`.
type interactiveFlags struct {
	outputDir    string
	cacheDir     string
	terraformBin string
	composeDir   string
	skipCompose  bool
}

// NewInteractiveCommand returns the `infra-composer interactive` command
// which guides the user through provider selection, version pick,
// resource multi-select and catalog persistence.
func NewInteractiveCommand() *cobra.Command {
	f := &interactiveFlags{}
	cmd := &cobra.Command{
		Use:   "interactive",
		Short: "Guided workflow: pick provider, pick resources, build catalog",
		Long: `Run an interactive session that walks you through:
 1. Choosing a Terraform provider (preset list + freeform).
 2. Selecting the version to pin (defaults to latest).
 3. Downloading the provider schema (cached on disk).
 4. Multi-selecting the resources and data sources you want.
 5. Persisting a filtered catalog schema.
 6. Optionally composing a Terraform stack from the selection.

This command requires terraform >= 1.0 to be installed because it shells
out to "terraform providers schema -json" under the hood.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInteractive(cmd.Context(), cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "Where to write the generated schema.json (required)")
	cmd.Flags().StringVar(&f.cacheDir, "cache-dir", "", "Override cache directory (default $XDG_CACHE_HOME/infra-composer)")
	cmd.Flags().StringVar(&f.terraformBin, "terraform-binary", "", "Path to terraform binary (defaults to PATH lookup)")
	cmd.Flags().StringVar(&f.composeDir, "compose-dir", "", "If set, run `compose --output-dir <dir>` after building")
	cmd.Flags().BoolVar(&f.skipCompose, "no-compose-prompt", false, "Skip the post-build compose prompt entirely")
	return cmd
}

// runInteractive coordinates the interactive workflow. It is exposed
// for tests via package-internal helpers (interactiveIO).
func runInteractive(ctx context.Context, cmd *cobra.Command, f *interactiveFlags) error {
	stdin, stdout, stderr := interactiveIO(cmd)

	if f.outputDir == "" {
		return invalidArgs("--output-dir is required",
			"example: infra-composer interactive --output-dir ./catalog")
	}

	provider, err := promptProvider(stdin, stdout, stderr)
	if err != nil {
		return mapInteractiveError(err)
	}

	versions, err := registry.NewHTTPDiscovery().AvailableVersions(ctx, provider)
	if err != nil {
		return cliError(clierr.ExitGeneric,
			fmt.Sprintf("could not list versions for %s: %v", provider, err),
			"check the provider address and your network connectivity")
	}
	if len(versions) == 0 {
		return cliError(clierr.ExitGeneric,
			fmt.Sprintf("provider %s has no published versions", provider))
	}
	pinned, err := promptVersion(stdin, stdout, stderr, versions)
	if err != nil {
		return mapInteractiveError(err)
	}

	fmt.Fprintf(stdout, "→ fetching schema for %s@%s (this may take a moment on first run)…\n", provider, pinned)
	client := buildInteractiveClient(f)
	resolvedProvider := provider + "@" + pinned
	prov, err := client.DiscoverProvider(ctx, resolvedProvider)
	if err != nil {
		return cliError(clierr.ExitGeneric,
			fmt.Sprintf("discover %s failed: %v", resolvedProvider, err))
	}
	summaries, err := client.ListResources(ctx, *prov)
	if err != nil {
		return cliError(clierr.ExitGeneric,
			fmt.Sprintf("list resources for %s failed: %v", prov.Address(), err),
			"is `terraform` on PATH? try --terraform-binary <path>")
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })

	picked, err := promptResources(stdin, stdout, stderr, summaries)
	if err != nil {
		return mapInteractiveError(err)
	}
	if len(picked) == 0 {
		return cliError(exitInvalidArgs,
			"no resources selected; nothing to do",
			"re-run and pick at least one resource or data source")
	}

	include := append([]string(nil), picked...)

	builder := catalog.NewBuilder(client)
	s, err := builder.Build(ctx, catalog.BuildOptions{
		Provider: provider,
		Include:  include,
		Now:      func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return mapBuildError(provider, "terraform-exec", err)
	}
	dest, err := catalog.Export(s, catalog.ExportOptions{Dir: f.outputDir})
	if err != nil {
		return cliError(clierr.ExitGeneric,
			fmt.Sprintf("failed to export catalog: %v", err))
	}
	fmt.Fprintf(stdout, "✓ catalog written to %s (%d module(s))\n", dest, len(s.Modules))

	if f.skipCompose {
		return nil
	}
	composeNow, composeOut, err := promptCompose(stdin, stdout, stderr, f.composeDir)
	if err != nil {
		return mapInteractiveError(err)
	}
	if !composeNow {
		return nil
	}
	fmt.Fprintf(stdout, "→ running compose into %s …\n", composeOut)
	return runComposeFromInteractive(ctx, dest, composeOut, picked)
}

// promptProvider asks for a provider address using a select list with a
// freeform fallback when the user picks "Other".
func promptProvider(in terminal.FileReader, out terminal.FileWriter, errW io.Writer) (string, error) {
	choice := ""
	q := &survey.Select{
		Message: "Pick a Terraform provider:",
		Options: presetProviders,
		Default: presetProviders[0],
	}
	if err := survey.AskOne(q, &choice, withStdio(in, out, errW)); err != nil {
		return "", err
	}
	if choice != "Other (type address)" {
		return choice, nil
	}
	custom := ""
	freeform := &survey.Input{Message: "Provider address (<namespace>/<name>):"}
	if err := survey.AskOne(freeform, &custom, withStdio(in, out, errW), survey.WithValidator(validateProviderAddress)); err != nil {
		return "", err
	}
	return strings.TrimSpace(custom), nil
}

// validateProviderAddress is a survey.Validator that rejects malformed
// "<ns>/<name>" addresses early.
func validateProviderAddress(ans any) error {
	s, ok := ans.(string)
	if !ok {
		return errors.New("expected string answer")
	}
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("address must be \"<namespace>/<name>\"")
	}
	return nil
}

// promptVersion lets the user pick from the list of published versions.
// The latest is offered as the default; "latest" is shown explicitly.
func promptVersion(in terminal.FileReader, out terminal.FileWriter, errW io.Writer, versions []string) (string, error) {
	options := append([]string{"latest"}, versions...)
	pick := ""
	q := &survey.Select{
		Message:  "Pick a version:",
		Options:  options,
		Default:  "latest",
		PageSize: 12,
	}
	if err := survey.AskOne(q, &pick, withStdio(in, out, errW)); err != nil {
		return "", err
	}
	if pick == "latest" {
		// Resolve "latest" against the supplied list using the same
		// algorithm HTTPDiscovery uses internally.
		return registry.PickLatestVersion(versions), nil
	}
	return pick, nil
}

// promptResources runs a survey.MultiSelect with incremental filtering
// over the supplied summaries.
func promptResources(in terminal.FileReader, out terminal.FileWriter, errW io.Writer, summaries []registry.ResourceSummary) ([]string, error) {
	options := make([]string, len(summaries))
	for i, s := range summaries {
		options[i] = formatSummary(s)
	}
	picked := []string{}
	q := &survey.MultiSelect{
		Message:  fmt.Sprintf("Pick resources (%d available; type to filter):", len(options)),
		Options:  options,
		PageSize: 15,
	}
	if err := survey.AskOne(q, &picked, withStdio(in, out, errW)); err != nil {
		return nil, err
	}
	out2 := make([]string, 0, len(picked))
	for _, p := range picked {
		out2 = append(out2, parseSummaryName(p))
	}
	return out2, nil
}

// formatSummary renders a ResourceSummary as "<name>  [resource|data]".
func formatSummary(s registry.ResourceSummary) string {
	return fmt.Sprintf("%s  [%s]", s.Name, s.Kind)
}

// parseSummaryName recovers the bare resource name from a label
// produced by formatSummary.
func parseSummaryName(label string) string {
	if i := strings.Index(label, "  ["); i > 0 {
		return label[:i]
	}
	return label
}

// promptCompose asks whether the user wants to follow up with a
// `compose` invocation and returns the chosen output dir.
func promptCompose(in terminal.FileReader, out terminal.FileWriter, errW io.Writer, defaultDir string) (bool, string, error) {
	confirm := false
	q := &survey.Confirm{Message: "Generate Terraform files now?", Default: true}
	if err := survey.AskOne(q, &confirm, withStdio(in, out, errW)); err != nil {
		return false, "", err
	}
	if !confirm {
		return false, "", nil
	}
	if defaultDir == "" {
		defaultDir = "./infrastructure"
	}
	dir := defaultDir
	d := &survey.Input{Message: "Output directory for generated Terraform files:", Default: defaultDir}
	if err := survey.AskOne(d, &dir, withStdio(in, out, errW)); err != nil {
		return false, "", err
	}
	return true, dir, nil
}

// buildInteractiveClient constructs a TerraformExecClient honouring the
// CLI flags (cache dir, custom terraform binary).
func buildInteractiveClient(f *interactiveFlags) registry.Client {
	opts := []registry.TerraformExecOption{}
	if f.terraformBin != "" {
		opts = append(opts, registry.WithTerraformBinary(f.terraformBin))
	}
	cacheRoot := f.cacheDir
	if cacheRoot == "" {
		cacheRoot = defaultCacheDir()
	}
	if cacheRoot != "" {
		opts = append(opts,
			registry.WithPluginCacheDir(filepath.Join(cacheRoot, "plugins")),
			registry.WithSchemaCacheDir(filepath.Join(cacheRoot, "schemas")),
		)
	}
	return registry.NewTerraformExecClient(opts...)
}

// runComposeFromInteractive shells the compose path with the catalog
// just produced and the resources the user selected.
func runComposeFromInteractive(ctx context.Context, schemaPath, outputDir string, picked []string) error {
	composeCmd := NewComposeCommand()
	args := []string{
		"--schema", schemaPath,
		"--output-dir", outputDir,
		"--modules", strings.Join(picked, ","),
	}
	composeCmd.SetArgs(args)
	composeCmd.SetContext(ctx)
	return composeCmd.Execute()
}

// interactiveIO returns the I/O streams to use with survey, defaulting
// to os.Stdin/os.Stdout/os.Stderr but honouring any cobra-level stderr
// override so test harnesses can capture diagnostics.
func interactiveIO(cmd *cobra.Command) (terminal.FileReader, terminal.FileWriter, io.Writer) {
	var in terminal.FileReader = os.Stdin
	var out terminal.FileWriter = os.Stdout
	var errW io.Writer = os.Stderr
	if cmd != nil && cmd.ErrOrStderr() != nil {
		errW = cmd.ErrOrStderr()
	}
	return in, out, errW
}

// withStdio is the survey.AskOpt that wires our IO streams into
// AskOne calls, allowing tests to feed scripted answers.
func withStdio(in terminal.FileReader, out terminal.FileWriter, errW io.Writer) survey.AskOpt {
	return survey.WithStdio(in, out, errW)
}

// mapInteractiveError converts survey-specific errors (notably the
// Ctrl+C interrupt) into stable CLI errors.
func mapInteractiveError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, terminal.InterruptErr) {
		return cliError(clierr.ExitGeneric, "interactive session aborted by user")
	}
	return cliError(clierr.ExitGeneric, fmt.Sprintf("interactive prompt failed: %v", err))
}
