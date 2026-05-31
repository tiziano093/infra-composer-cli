package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Parse decodes a catalog Schema from r. It returns a *ParseError on
// malformed input. Parse does NOT validate the decoded Schema; callers
// that need full validation should chain Schema.Validate.
func Parse(r io.Reader) (*Schema, error) {
	if r == nil {
		return nil, &ParseError{Cause: fmt.Errorf("nil reader")}
	}
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var s Schema
	if err := dec.Decode(&s); err != nil {
		return nil, &ParseError{Cause: err}
	}
	// Reject trailing junk after the top-level object so accidental
	// concatenation of two catalogs is caught early.
	if dec.More() {
		return nil, &ParseError{Cause: fmt.Errorf("unexpected trailing data after catalog object")}
	}
	return &s, nil
}

// Load reads the catalog file at path, parses it, and validates it.
// The returned *ParseError or *ValidationError carries Path so error
// messages remain actionable for the user.
func Load(path string) (*Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		// Surface the OS error as-is so callers can inspect it with
		// errors.Is(err, os.ErrNotExist) and map to ExitFileNotFound.
		return nil, err
	}
	defer f.Close()

	s, err := Parse(f)
	if err != nil {
		// Enrich ParseError with the path for nicer diagnostics.
		var pe *ParseError
		if asParseError(err, &pe) {
			pe.Path = path
		}
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return s, err
	}
	return s, nil
}

// asParseError is a tiny helper kept local to avoid importing errors in
// every file.
func asParseError(err error, target **ParseError) bool {
	if pe, ok := err.(*ParseError); ok {
		*target = pe
		return true
	}
	return false
}
