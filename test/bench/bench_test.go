// Package bench contains performance benchmarks for the critical paths.
// Run with: go test ./test/bench/... -bench=. -benchmem
package bench

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/cli"
	"github.com/tiziano093/infra-composer-cli/internal/terraform"
)

func fixturesDir() string {
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "fixtures")
}

func validFullSchema(b *testing.B) *catalog.Schema {
	b.Helper()
	path := filepath.Join(fixturesDir(), "schemas", "valid_full.json")
	s, err := catalog.Load(path)
	if err != nil {
		b.Fatalf("load schema: %v", err)
	}
	return s
}

// BenchmarkCatalogLoad measures schema parsing from disk.
func BenchmarkCatalogLoad(b *testing.B) {
	path := filepath.Join(fixturesDir(), "schemas", "valid_full.json")
	b.ResetTimer()
	for range b.N {
		s, err := catalog.Load(path)
		if err != nil || s == nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCatalogValidate measures validation pass on a valid schema.
func BenchmarkCatalogValidate(b *testing.B) {
	s := validFullSchema(b)
	b.ResetTimer()
	for range b.N {
		_ = s.Validate()
	}
}

// BenchmarkSearch measures keyword search with AND logic.
func BenchmarkSearch(b *testing.B) {
	s := validFullSchema(b)
	opts := catalog.SearchOptions{Keywords: []string{"aws"}, Limit: 20}
	b.ResetTimer()
	for range b.N {
		_ = catalog.Search(s, opts)
	}
}

// BenchmarkDependencyGraph measures graph construction.
func BenchmarkDependencyGraph(b *testing.B) {
	s := validFullSchema(b)
	b.ResetTimer()
	for range b.N {
		_ = catalog.BuildGraph(s)
	}
}

// BenchmarkComposePlan measures the compose planning phase (no file I/O).
func BenchmarkComposePlan(b *testing.B) {
	s := validFullSchema(b)
	modules := make([]string, 0, len(s.Modules))
	for _, m := range s.Modules {
		modules = append(modules, m.Name)
	}
	opts := terraform.PlanOptions{Modules: modules}
	b.ResetTimer()
	for range b.N {
		_, _ = terraform.Plan(s, opts)
	}
}

// BenchmarkCLIVersion measures end-to-end CLI invocation cost (version cmd).
func BenchmarkCLIVersion(b *testing.B) {
	var buf bytes.Buffer
	args := []string{"version"}
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		cli.Execute(context.Background(), cli.BuildInfo{Version: "bench"}, args, &buf, &buf)
	}
}

// BenchmarkComposeGenerate measures HCL generation (in-memory, minimal disk).
func BenchmarkComposeGenerate(b *testing.B) {
	s := validFullSchema(b)
	modules := make([]string, 0, len(s.Modules))
	for _, m := range s.Modules {
		modules = append(modules, m.Name)
	}
	plan, err := terraform.Plan(s, terraform.PlanOptions{Modules: modules})
	if err != nil {
		b.Fatalf("plan: %v", err)
	}
	dir := b.TempDir()
	b.ResetTimer()
	for range b.N {
		files, err := terraform.Generate(plan)
		if err != nil {
			b.Fatal(err)
		}
		for _, f := range files {
			_ = os.WriteFile(filepath.Join(dir, filepath.Base(f.Path)), f.Content, 0o644)
		}
	}
}
