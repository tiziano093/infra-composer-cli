// Package output centralises user-facing output: structured logging and
// result formatting (json, yaml, table). Logging is separated from result
// output: results go to stdout, logs go to stderr.
package output

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// LogFormat selects the slog handler used by NewLogger.
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// LoggerOptions configures NewLogger.
type LoggerOptions struct {
	Level  string    // debug|info|warn|error (case-insensitive); empty = info
	Format LogFormat // text (default) or json
	Writer io.Writer // defaults to os.Stderr
	Quiet  bool      // if true, level forced to error
}

// ParseLevel converts a string log level to slog.Level. Unknown values
// fall back to slog.LevelInfo and return an error so callers can warn.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}

// NewLogger builds a slog.Logger from LoggerOptions. Errors parsing the
// level are swallowed (info is used as fallback) — callers wanting strict
// validation should call ParseLevel directly first.
func NewLogger(opts LoggerOptions) *slog.Logger {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	level, _ := ParseLevel(opts.Level)
	if opts.Quiet {
		level = slog.LevelError
	}
	handlerOpts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if opts.Format == LogFormatJSON {
		h = slog.NewJSONHandler(w, handlerOpts)
	} else {
		h = slog.NewTextHandler(w, handlerOpts)
	}
	return slog.New(h)
}
