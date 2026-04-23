// Package integration hosts opt-in end-to-end tests that exercise the
// real Terraform Registry. They are skipped unless the developer
// explicitly opts in via INFRA_COMPOSER_E2E=1, because they:
//   - require network access to registry.terraform.io
//   - require a `terraform` binary (>=1.0) on PATH
//   - download a real provider plugin (cached on disk under TF_PLUGIN_CACHE_DIR)
//
// The smoke target is "hashicorp/random" because it ships only a handful
// of resources and no cloud credentials are involved.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
)

func requireE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("INFRA_COMPOSER_E2E") != "1" {
		t.Skip("skipping; set INFRA_COMPOSER_E2E=1 to run the registry round-trip")
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("skipping; terraform binary not in PATH")
	}
}

func TestE2E_HashicorpRandom_BuildsCatalog(t *testing.T) {
	requireE2E(t)

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	outputDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := registry.NewTerraformExecClient(
		registry.WithPluginCacheDir(filepath.Join(cacheRoot, "plugins")),
		registry.WithSchemaCacheDir(filepath.Join(cacheRoot, "schemas")),
	)
	builder := catalog.NewBuilder(client)
	s, err := builder.Build(ctx, catalog.BuildOptions{
		Provider: "hashicorp/random",
		Now:      func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(s.Modules) == 0 {
		t.Fatal("expected at least one module from hashicorp/random")
	}

	// Sanity: random_id is a known resource and must round-trip into
	// the schema unchanged.
	hasRandomID := false
	for _, m := range s.Modules {
		if m.Name == "random_id" {
			hasRandomID = true
			break
		}
	}
	if !hasRandomID {
		t.Fatal("expected random_id in catalog modules")
	}

	dest, err := catalog.Export(s, catalog.ExportOptions{Dir: outputDir})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var roundTrip catalog.Schema
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(roundTrip.Modules) != len(s.Modules) {
		t.Fatalf("module count mismatch: got %d want %d",
			len(roundTrip.Modules), len(s.Modules))
	}
}

func TestE2E_HashicorpRandom_FilteredCatalog(t *testing.T) {
	requireE2E(t)

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := registry.NewTerraformExecClient(
		registry.WithPluginCacheDir(filepath.Join(cacheRoot, "plugins")),
		registry.WithSchemaCacheDir(filepath.Join(cacheRoot, "schemas")),
	)
	builder := catalog.NewBuilder(client)
	s, err := builder.Build(ctx, catalog.BuildOptions{
		Provider: "hashicorp/random",
		Include:  []string{"random_id"},
		Now:      func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(s.Modules) != 1 || s.Modules[0].Name != "random_id" {
		t.Fatalf("expected exactly random_id, got %+v", s.Modules)
	}
}
