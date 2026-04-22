// Package commands hosts the implementations of every CLI subcommand.
// Commands receive shared runtime state (config, logger, build info) via
// the context-bound Runtime value injected by the cli root.
package commands

import (
	"context"
	"io"

	"log/slog"

	"github.com/tiziano093/infra-composer-cli/internal/config"
)

// BuildInfo mirrors cli.BuildInfo to avoid a cycle between the two
// packages (cli depends on commands, never the reverse).
type BuildInfo struct {
	Version   string
	BuildTime string
	GitCommit string
}

// Runtime is the per-invocation set of dependencies passed to commands.
type Runtime struct {
	Config *config.Config
	Logger *slog.Logger
	Build  BuildInfo
	Stdout io.Writer
	Stderr io.Writer
}

type runtimeKey struct{}

// WithRuntime returns a context carrying the given Runtime.
func WithRuntime(ctx context.Context, r Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, r)
}

// FromContext retrieves the Runtime previously stored with WithRuntime.
// Commands run via the cobra root always have a Runtime; ok is false only
// when the function is called outside that flow (e.g., in unit tests).
func FromContext(ctx context.Context) (Runtime, bool) {
	r, ok := ctx.Value(runtimeKey{}).(Runtime)
	return r, ok
}
