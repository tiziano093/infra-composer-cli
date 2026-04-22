// Package main is the entry point for the infra-composer CLI.
package main

import (
	"context"
	"os"

	"github.com/tiziano093/infra-composer-cli/internal/cli"
)

// Build-time variables, injected via -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "none"
)

func main() {
	code := cli.Execute(
		context.Background(),
		cli.BuildInfo{Version: Version, BuildTime: BuildTime, GitCommit: GitCommit},
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	)
	os.Exit(int(code))
}
