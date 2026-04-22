package commands

import (
	"github.com/tiziano093/infra-composer-cli/internal/clierr"
)

// Local aliases for the most-used clierr identifiers so command files
// stay terse while still routing through the shared error package.
const (
	exitInvalidArgs      = clierr.ExitInvalidArgs
	exitFileNotFound     = clierr.ExitFileNotFound
	exitValidationFailed = clierr.ExitValidationFailed
)

// cliError builds a *clierr.CLIError with optional remediation hints.
// It returns the concrete pointer type so callers can chain
// WithSuggestions if they need more than the variadic shortcut here.
func cliError(code clierr.ExitCode, message string, hints ...string) *clierr.CLIError {
	e := clierr.New(code, message)
	if len(hints) > 0 {
		e.Suggestions = append(e.Suggestions, hints...)
	}
	return e
}

// invalidArgs is shorthand for argument-validation failures.
func invalidArgs(message string, hints ...string) *clierr.CLIError {
	return cliError(exitInvalidArgs, message, hints...)
}
