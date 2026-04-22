package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIError_ErrorString(t *testing.T) {
	t.Parallel()
	e := &CLIError{Code: ExitModuleNotFound, Message: "module not found"}
	assert.Equal(t, "module not found", e.Error())

	e2 := &CLIError{Code: ExitModuleNotFound, Message: "module not found", Details: "aws_rds"}
	assert.Equal(t, "module not found: aws_rds", e2.Error())
}

func TestWrap_PreservesCLIError(t *testing.T) {
	t.Parallel()
	original := New(ExitValidationFailed, "bad schema")
	wrapped := Wrap(ExitGeneric, "ignored", original)
	assert.Same(t, original, wrapped, "Wrap should pass through existing CLIError")
}

func TestWrap_UnwrapsCause(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("disk full")
	wrapped := Wrap(ExitGeneric, "write file", cause)
	assert.Equal(t, ExitGeneric, wrapped.Code)
	assert.Equal(t, "write file", wrapped.Message)
	assert.Equal(t, "disk full", wrapped.Details)
	assert.True(t, errors.Is(wrapped, cause))
}

func TestCodeOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ExitSuccess, CodeOf(nil))
	assert.Equal(t, ExitGeneric, CodeOf(errors.New("plain")))
	assert.Equal(t, ExitFileNotFound, CodeOf(New(ExitFileNotFound, "missing")))

	wrapped := fmt.Errorf("ctx: %w", New(ExitNetworkError, "timeout"))
	assert.Equal(t, ExitNetworkError, CodeOf(wrapped))
}

func TestWithSuggestions(t *testing.T) {
	t.Parallel()
	e := New(ExitModuleNotFound, "nope").WithSuggestions("did you mean foo?", "run `search`")
	require.Len(t, e.Suggestions, 2)
	assert.Equal(t, "did you mean foo?", e.Suggestions[0])
}
