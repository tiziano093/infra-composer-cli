// Package cli wires the Cobra command tree and provides the CLI-wide error
// and exit-code framework. Domain packages return their own error types;
// the CLI layer converts them to CLIError values for consistent UX.
package cli

import (
	"errors"
	"fmt"
)

// ExitCode is the process exit status returned by the CLI. Values follow the
// table documented in docs/ARCHITECTURE.md (Error Handling Strategy).
type ExitCode int

const (
	ExitSuccess           ExitCode = 0
	ExitGeneric           ExitCode = 1
	ExitInvalidArgs       ExitCode = 2
	ExitFileNotFound      ExitCode = 3
	ExitValidationFailed  ExitCode = 4
	ExitModuleNotFound    ExitCode = 5
	ExitDependencyFailed  ExitCode = 6
	ExitGitFailed         ExitCode = 7
	ExitTerraformFailed   ExitCode = 8
	ExitNetworkError      ExitCode = 9
	ExitPermissionDenied  ExitCode = 10
)

// CLIError is the canonical error type surfaced by commands. It carries a
// stable exit code, a human-readable message, optional detail string, and
// optional remediation suggestions printed to the user.
type CLIError struct {
	Code        ExitCode
	Message     string
	Details     string
	Suggestions []string
	Cause       error
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *CLIError) Unwrap() error { return e.Cause }

// New builds a CLIError with the given exit code and message.
func New(code ExitCode, message string) *CLIError {
	return &CLIError{Code: code, Message: message}
}

// Wrap converts an arbitrary error to a CLIError, preserving the cause chain.
// If err is already a *CLIError it is returned unchanged.
func Wrap(code ExitCode, message string, err error) *CLIError {
	if err == nil {
		return New(code, message)
	}
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce
	}
	return &CLIError{Code: code, Message: message, Details: err.Error(), Cause: err}
}

// WithSuggestions returns a copy of the error annotated with remediation hints.
func (e *CLIError) WithSuggestions(s ...string) *CLIError {
	e.Suggestions = append(e.Suggestions, s...)
	return e
}

// CodeOf extracts the exit code from any error. Non-CLIError values map to
// ExitGeneric; nil maps to ExitSuccess.
func CodeOf(err error) ExitCode {
	if err == nil {
		return ExitSuccess
	}
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ExitGeneric
}
