package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// repoFixturesDir resolves test/fixtures/<sub> relative to this test
// file. Shared with builder_test.go so disk-backed tests do not need to
// duplicate the lookup.
func repoFixturesDir(t *testing.T, sub string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "test", "fixtures", sub)
}

func sampleSchema() *Schema {
	return &Schema{
		SchemaVersion:   SchemaVersion,
		Provider:        "hashicorp/aws",
		ProviderVersion: "5.42.0",
		GeneratedAt:     time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
		Modules: []ModuleEntry{
			{Name: "aws_vpc", Type: ModuleTypeResource, Group: "network",
				Variables: []Variable{{Name: "cidr_block", Type: "string", Required: true}},
				Outputs:   []Output{{Name: "id"}}},
		},
	}
}

func TestExport_ToDir_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := sampleSchema()
	dest, err := Export(s, ExportOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if want := filepath.Join(dir, SchemaFileName); dest != want {
		t.Errorf("dest: got %q want %q", dest, want)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("perm: got %o want 0644", info.Mode().Perm())
	}
	loaded, err := Load(dest)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Provider != s.Provider || len(loaded.Modules) != 1 {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestExport_ToPath_CreatesParents(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "deep", "nested", "out.json")
	if _, err := Export(sampleSchema(), ExportOptions{Path: dest}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestExport_NilSchema(t *testing.T) {
	if _, err := Export(nil, ExportOptions{Dir: t.TempDir()}); err == nil {
		t.Fatal("expected error for nil schema")
	}
}

func TestExport_RequiresDestination(t *testing.T) {
	if _, err := Export(sampleSchema(), ExportOptions{}); err == nil {
		t.Fatal("expected error when Path and Dir both empty")
	}
}

func TestExport_PathAndDirMutuallyExclusive(t *testing.T) {
	if _, err := Export(sampleSchema(), ExportOptions{Path: "a", Dir: "b"}); err == nil {
		t.Fatal("expected error when both Path and Dir set")
	}
}

func TestExport_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, SchemaFileName)
	if err := os.WriteFile(dest, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(sampleSchema(), ExportOptions{Dir: dir}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(got) {
		t.Fatalf("output is not JSON: %s", got)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("output should end with newline")
	}
}

func TestExport_NoTempFilesLeftBehind(t *testing.T) {
	dir := t.TempDir()
	if _, err := Export(sampleSchema(), ExportOptions{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".schema-") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}
