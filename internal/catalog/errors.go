package catalog

import (
	"errors"
	"fmt"
	"strings"
)

// ParseError wraps a low-level decode failure (malformed JSON, unexpected
// EOF, etc.) with the originating source path when known.
type ParseError struct {
	Path  string
	Cause error
}

// Error implements error.
func (e *ParseError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("parse catalog %s: %v", e.Path, e.Cause)
	}
	return fmt.Sprintf("parse catalog: %v", e.Cause)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *ParseError) Unwrap() error { return e.Cause }

// ValidationIssue is a single problem found while validating a Schema.
// Field is a dotted path locating the offending element, e.g.
// "modules[3].variables[0].name".
type ValidationIssue struct {
	Field   string
	Message string
}

// String renders the issue as "field: message" (or just "message" when
// the field path is empty).
func (i ValidationIssue) String() string {
	if i.Field == "" {
		return i.Message
	}
	return i.Field + ": " + i.Message
}

// ValidationError aggregates one or more ValidationIssue values produced
// by Schema.Validate. A non-nil ValidationError always contains at least
// one issue.
type ValidationError struct {
	Issues []ValidationIssue
}

// Error implements error. Issues are joined with "; " in declaration
// order so the message is stable for snapshot tests.
func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, iss := range e.Issues {
		parts[i] = iss.String()
	}
	return "catalog validation failed: " + strings.Join(parts, "; ")
}

// AsValidationError returns the wrapped *ValidationError if err (or any
// error in its chain) is one, otherwise nil + false.
func AsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
