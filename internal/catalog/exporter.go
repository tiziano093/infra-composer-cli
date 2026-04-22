package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaFileName is the canonical filename used by Export when a target
// directory is provided.
const SchemaFileName = "schema.json"

// ExportOptions parameterises Export. Path takes precedence over Dir;
// when both are empty Export returns an error.
type ExportOptions struct {
	// Path is an explicit destination file. Parent directories are
	// created with permission 0o755 if missing.
	Path string

	// Dir is a destination directory. The schema is written to
	// filepath.Join(Dir, SchemaFileName).
	Dir string

	// FileMode controls the final file permission. Defaults to 0o644.
	FileMode os.FileMode
}

// Export serialises s as indented JSON and writes it to disk atomically:
// the payload is first written to a sibling temporary file in the target
// directory, then renamed into place so partial writes never leave a
// half-formed schema.json behind.
//
// The returned string is the absolute (or as-given) path the file was
// written to so callers can surface it in user-facing output.
func Export(s *Schema, opts ExportOptions) (string, error) {
	if s == nil {
		return "", fmt.Errorf("export catalog: schema is nil")
	}
	dest, err := resolveExportPath(opts)
	if err != nil {
		return "", err
	}
	mode := opts.FileMode
	if mode == 0 {
		mode = 0o644
	}

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("export catalog: mkdir %s: %w", dir, err)
	}

	payload, err := marshalSchema(s)
	if err != nil {
		return "", fmt.Errorf("export catalog: marshal: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".schema-*.json")
	if err != nil {
		return "", fmt.Errorf("export catalog: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("export catalog: write temp %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("export catalog: chmod %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("export catalog: close temp %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		cleanup()
		return "", fmt.Errorf("export catalog: rename %s -> %s: %w", tmpPath, dest, err)
	}
	return dest, nil
}

// marshalSchema produces the canonical on-disk representation of s. We
// indent with two spaces to match the validate fixtures and append a
// trailing newline so editors and POSIX tools play well with the file.
func marshalSchema(s *Schema) ([]byte, error) {
	buf, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}

func resolveExportPath(opts ExportOptions) (string, error) {
	switch {
	case opts.Path != "" && opts.Dir != "":
		return "", fmt.Errorf("export catalog: provide either Path or Dir, not both")
	case opts.Path != "":
		return opts.Path, nil
	case opts.Dir != "":
		return filepath.Join(opts.Dir, SchemaFileName), nil
	default:
		return "", fmt.Errorf("export catalog: no destination (Path or Dir required)")
	}
}
