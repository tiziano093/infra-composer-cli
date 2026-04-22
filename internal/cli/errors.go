// Package cli wires the Cobra command tree. Error type and exit codes
// live in internal/clierr to avoid an import cycle between cli and
// commands; this file provides aliases so existing call sites keep
// working.
package cli

import "github.com/tiziano093/infra-composer-cli/internal/clierr"

// ExitCode mirrors clierr.ExitCode.
type ExitCode = clierr.ExitCode

const (
	ExitSuccess          = clierr.ExitSuccess
	ExitGeneric          = clierr.ExitGeneric
	ExitInvalidArgs      = clierr.ExitInvalidArgs
	ExitFileNotFound     = clierr.ExitFileNotFound
	ExitValidationFailed = clierr.ExitValidationFailed
	ExitModuleNotFound   = clierr.ExitModuleNotFound
	ExitDependencyFailed = clierr.ExitDependencyFailed
	ExitGitFailed        = clierr.ExitGitFailed
	ExitTerraformFailed  = clierr.ExitTerraformFailed
	ExitNetworkError     = clierr.ExitNetworkError
	ExitPermissionDenied = clierr.ExitPermissionDenied
)

// CLIError mirrors clierr.CLIError.
type CLIError = clierr.CLIError

// New mirrors clierr.New.
func New(code ExitCode, message string) *CLIError { return clierr.New(code, message) }

// Wrap mirrors clierr.Wrap.
func Wrap(code ExitCode, message string, err error) *CLIError {
	return clierr.Wrap(code, message, err)
}

// CodeOf mirrors clierr.CodeOf.
func CodeOf(err error) ExitCode { return clierr.CodeOf(err) }
