package output

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := ParseLevel("nonsense")
	assert.Error(t, err)
}

func TestNewLogger_TextRespectsLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := NewLogger(LoggerOptions{Level: "warn", Writer: &buf, Format: LogFormatText})
	log.Info("ignored")
	log.Warn("kept", "k", "v")
	out := buf.String()
	assert.NotContains(t, out, "ignored")
	assert.Contains(t, out, "kept")
	assert.Contains(t, out, "k=v")
}

func TestNewLogger_QuietForcesError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := NewLogger(LoggerOptions{Level: "debug", Writer: &buf, Quiet: true})
	log.Warn("warn-msg")
	log.Error("err-msg")
	out := buf.String()
	assert.NotContains(t, out, "warn-msg")
	assert.Contains(t, out, "err-msg")
}

func TestNewLogger_JSONFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := NewLogger(LoggerOptions{Format: LogFormatJSON, Writer: &buf})
	log.Info("hello", "n", 1)

	line := strings.TrimSpace(buf.String())
	require.NotEmpty(t, line)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record))
	assert.Equal(t, "hello", record["msg"])
	assert.EqualValues(t, 1, record["n"])
}
