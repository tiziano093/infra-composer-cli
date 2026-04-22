// Package main is the entry point for the infra-composer CLI.
package main

import (
	"fmt"
	"os"
	"runtime"
)

// Build-time variables, injected via -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "none"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("infra-composer %s\n  built: %s\n  commit: %s\n  go: %s %s/%s\n",
				Version, BuildTime, GitCommit,
				runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return
		case "--help", "-h", "help":
			printHelp()
			return
		}
	}
	printHelp()
}

func printHelp() {
	fmt.Println(`infra-composer — Portable CLI for composing Terraform stacks from catalogs.

Usage:
  infra-composer [command] [flags]

Commands (planned):
  catalog       Catalog operations (build, export, list, validate)
  search        Search modules in a catalog
  dependencies  Show module dependency tree
  interface     Show module variables and outputs
  compose       Compose a Terraform stack from modules
  version       Show version information

Flags:
  -h, --help     Show help
  -v, --version  Show version

Status: scaffolding (Phase 1 in progress).`)
}
