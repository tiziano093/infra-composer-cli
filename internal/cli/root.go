package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/tiziano093/infra-composer-cli/internal/commands"
	"github.com/tiziano093/infra-composer-cli/internal/config"
	"github.com/tiziano093/infra-composer-cli/internal/output"
)

// BuildInfo carries values injected by the linker at build time.
type BuildInfo struct {
	Version   string
	BuildTime string
	GitCommit string
}

// globalFlags holds values bound to the persistent root flags. They are
// resolved into a *config.Config in the PersistentPreRunE hook.
type globalFlags struct {
	configFile string
	logLevel   string
	logFormat  string
	format     string
	verbose    bool
	quiet      bool
}

// NewRootCommand assembles the root *cobra.Command and registers all
// subcommands. It is exported so tests can construct an isolated tree.
func NewRootCommand(info BuildInfo) *cobra.Command {
	gf := &globalFlags{}
	var (
		cfg    *config.Config
		logger = output.NewLogger(output.LoggerOptions{}) // overwritten in PreRun
	)

	root := &cobra.Command{
		Use:           "infra-composer",
		Short:         "Portable CLI for composing Terraform stacks from catalogs.",
		Long:          rootLongDescription,
		Version:       info.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			loaded, err := config.Load(config.LoadOptions{ConfigFile: gf.configFile})
			if err != nil {
				return Wrap(ExitFileNotFound, "load configuration", err)
			}
			applyFlagOverrides(loaded, gf, cmd)
			cfg = loaded
			logger = output.NewLogger(output.LoggerOptions{
				Level:  cfg.Logging.Level,
				Format: output.LogFormat(cfg.Logging.Format),
				Quiet:  gf.quiet,
			})
			ctx := commands.WithRuntime(cmd.Context(), commands.Runtime{
				Config: cfg,
				Logger: logger,
				Build: commands.BuildInfo{
					Version:   info.Version,
					BuildTime: info.BuildTime,
					GitCommit: info.GitCommit,
				},
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
			cmd.SetContext(ctx)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&gf.configFile, "config", "", "Path to YAML config file (default ~/.infra-composer/config.yaml)")
	root.PersistentFlags().StringVar(&gf.logLevel, "log-level", "", "Log level: debug|info|warn|error")
	root.PersistentFlags().StringVar(&gf.logFormat, "log-format", "", "Log format: text|json")
	root.PersistentFlags().StringVarP(&gf.format, "format", "f", "", "Output format: table|json|yaml")
	root.PersistentFlags().BoolVarP(&gf.verbose, "verbose", "v", false, "Verbose output (sets log-level=debug if not provided)")
	root.PersistentFlags().BoolVarP(&gf.quiet, "quiet", "q", false, "Suppress non-error logs")

	root.AddCommand(commands.NewVersionCommand(commands.BuildInfo{
		Version:   info.Version,
		BuildTime: info.BuildTime,
		GitCommit: info.GitCommit,
	}))
	root.AddCommand(commands.NewSearchCommand())
	root.AddCommand(commands.NewCatalogCommand())

	return root
}

// applyFlagOverrides layers explicit flag values on top of the config so
// CLI flags always win, per the documented hierarchy.
func applyFlagOverrides(cfg *config.Config, gf *globalFlags, cmd *cobra.Command) {
	if cmd.Flags().Changed("log-level") && gf.logLevel != "" {
		cfg.Logging.Level = gf.logLevel
	} else if gf.verbose && !cmd.Flags().Changed("log-level") {
		cfg.Logging.Level = "debug"
	}
	if cmd.Flags().Changed("log-format") && gf.logFormat != "" {
		cfg.Logging.Format = gf.logFormat
	}
	if cmd.Flags().Changed("format") && gf.format != "" {
		cfg.OutputFormat = gf.format
	}
}

// Execute runs the root command and returns the appropriate process exit
// code. It writes errors (formatted via FormatError) to stderr.
func Execute(ctx context.Context, info BuildInfo, args []string, stdout, stderr io.Writer) ExitCode {
	root := NewRootCommand(info)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetContext(ctx)

	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitSuccess
	}
	// Map cobra flag-parsing errors to ExitInvalidArgs for predictability.
	code := CodeOf(err)
	var ce *CLIError
	if !errors.As(err, &ce) && isFlagError(err) {
		code = ExitInvalidArgs
	}
	fmt.Fprintln(stderr, FormatError(err))
	return code
}

// FormatError renders an error for human consumption. CLIError values get
// suggestion lines; everything else is printed verbatim.
func FormatError(err error) string {
	var ce *CLIError
	if errors.As(err, &ce) {
		out := "Error: " + ce.Error()
		for _, s := range ce.Suggestions {
			out += "\n  hint: " + s
		}
		return out
	}
	return "Error: " + err.Error()
}

// isFlagError checks whether the error originates from cobra/pflag parsing.
// Cobra does not export a sentinel, so we use the canonical message prefix.
func isFlagError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return startsWithAny(msg, "unknown flag", "unknown shorthand flag", "flag needs an argument", "invalid argument", "unknown command")
}

func startsWithAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(s) >= len(p) && s[:len(p)] == p {
			return true
		}
	}
	return false
}

// Ensure os import retained even when unused on some platforms.
var _ = os.Stderr

const rootLongDescription = `infra-composer composes Terraform stacks from a curated module catalog.

Use the catalog commands to build and search a schema, then compose to
generate a ready-to-apply Terraform stack including supporting CI/CD,
linting and documentation files.`
