package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

func TestTerraformExecClient_DiskCacheRoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	c := NewTerraformExecClient(WithSchemaCacheDir(cacheDir))
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}

	want := &tfjson.ProviderSchema{
		ResourceSchemas: map[string]*tfjson.Schema{
			"random_string": {Block: &tfjson.SchemaBlock{
				Attributes: map[string]*tfjson.SchemaAttribute{
					"length": {AttributeType: cty.Number, Required: true},
				},
			}},
		},
	}
	c.saveSchemaToDisk(p, want)

	if _, err := os.Stat(filepath.Join(cacheDir, "hashicorp", "random", "3.6.0.json")); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	got, ok := c.loadSchemaFromDisk(p)
	if !ok {
		t.Fatal("loadSchemaFromDisk returned ok=false")
	}
	r := got.ResourceSchemas["random_string"]
	if r == nil || r.Block.Attributes["length"] == nil || !r.Block.Attributes["length"].Required {
		t.Fatalf("schema round-trip lost data: %+v", got)
	}
}

func TestTerraformExecClient_DiskCacheDisabledWhenNoDir(t *testing.T) {
	c := NewTerraformExecClient()
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}
	if _, ok := c.loadSchemaFromDisk(p); ok {
		t.Fatal("expected ok=false when cache dir unset")
	}
	// saveSchemaToDisk must be a no-op (no panic, no file created).
	c.saveSchemaToDisk(p, &tfjson.ProviderSchema{})
}

func TestTerraformExecClient_DiscoverProvider_BadAddress(t *testing.T) {
	c := NewTerraformExecClient()
	_, err := c.DiscoverProvider(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestTerraformExecClient_GetResourceSchema_FromCachedDisk(t *testing.T) {
	cacheDir := t.TempDir()
	c := NewTerraformExecClient(WithSchemaCacheDir(cacheDir))
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}

	c.saveSchemaToDisk(p, &tfjson.ProviderSchema{
		ResourceSchemas: map[string]*tfjson.Schema{
			"random_string": {Block: &tfjson.SchemaBlock{
				Description: "A random string.",
				Attributes: map[string]*tfjson.SchemaAttribute{
					"length": {AttributeType: cty.Number, Required: true},
					"result": {AttributeType: cty.String, Computed: true},
				},
			}},
		},
		DataSourceSchemas: map[string]*tfjson.Schema{
			"random_id": {Block: &tfjson.SchemaBlock{}},
		},
	})

	got, err := c.GetResourceSchema(context.Background(), p, "random_string", KindResource)
	if err != nil {
		t.Fatalf("GetResourceSchema: %v", err)
	}
	if got.Description != "A random string." || len(got.Inputs) != 1 || got.Inputs[0].Name != "length" {
		t.Fatalf("unexpected schema: %+v", got)
	}
	if len(got.Outputs) != 1 || got.Outputs[0].Name != "result" {
		t.Fatalf("unexpected outputs: %+v", got.Outputs)
	}

	list, err := c.ListResources(context.Background(), p)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListResources count: %d", len(list))
	}
}

func TestSplitAddressVersion(t *testing.T) {
	cases := []struct {
		in       string
		wantAddr string
		wantVer  string
	}{
		{"hashicorp/random", "hashicorp/random", ""},
		{"hashicorp/random@3.6.0", "hashicorp/random", "3.6.0"},
		{"hashicorp/random@latest", "hashicorp/random", "latest"},
		{"", "", ""},
	}
	for _, c := range cases {
		gotAddr, gotVer := splitAddressVersion(c.in)
		if gotAddr != c.wantAddr || gotVer != c.wantVer {
			t.Errorf("splitAddressVersion(%q)=(%q,%q) want (%q,%q)",
				c.in, gotAddr, gotVer, c.wantAddr, c.wantVer)
		}
	}
}
