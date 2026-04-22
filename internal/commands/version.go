package commands

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// NewVersionCommand returns the `version` subcommand. The build info is
// captured at construction so the command does not depend on globals.
func NewVersionCommand(info BuildInfo) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Print build version, commit and Go runtime details.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, _ := FromContext(cmd.Context())
			out := cmd.OutOrStdout()
			if rt.Stdout != nil {
				out = rt.Stdout
			}

			effective := format
			if effective == "" && rt.Config != nil {
				effective = rt.Config.OutputFormat
			}

			payload := map[string]string{
				"version":    info.Version,
				"build_time": info.BuildTime,
				"git_commit": info.GitCommit,
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			}

			switch strings.ToLower(effective) {
			case "json":
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(payload)
			default:
				fmt.Fprintf(out,
					"infra-composer %s\n  built: %s\n  commit: %s\n  go: %s %s/%s\n",
					info.Version, info.BuildTime, info.GitCommit,
					runtime.Version(), runtime.GOOS, runtime.GOARCH,
				)
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Output format: text|json (overrides global --format)")
	return cmd
}
