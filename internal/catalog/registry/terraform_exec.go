package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
)

// TerraformExecClient implements Client by shelling out to the local
// `terraform` CLI to (1) install the requested provider and (2) dump
// its full schema as JSON. The provider binary download itself is
// delegated to terraform; we only ensure the cache directory is set so
// successive Build() calls reuse the binary instead of re-downloading.
//
// Requirements:
//   - `terraform` (>= 1.0) must be on PATH (or pass an explicit binary
//     path via WithTerraformBinary).
//   - Network access on first invocation; subsequent invocations are
//     served entirely from the on-disk cache.
type TerraformExecClient struct {
	binaryPath     string
	pluginCacheDir string
	schemaCacheDir string
	tfRunDir       string

	mu     sync.Mutex
	cached map[string]*tfjson.ProviderSchema
}

// TerraformExecOption configures a TerraformExecClient.
type TerraformExecOption func(*TerraformExecClient)

// WithTerraformBinary points the client at a specific `terraform`
// executable. When unset, the binary is looked up on PATH.
func WithTerraformBinary(path string) TerraformExecOption {
	return func(c *TerraformExecClient) {
		if path != "" {
			c.binaryPath = path
		}
	}
}

// WithSchemaCacheDir enables persistent caching of the per-provider
// schema JSON dump. When the cache hits, the terraform binary is not
// invoked at all.
func WithSchemaCacheDir(dir string) TerraformExecOption {
	return func(c *TerraformExecClient) {
		c.schemaCacheDir = dir
	}
}

// WithPluginCacheDir sets the TF_PLUGIN_CACHE_DIR used during terraform
// init so provider binaries are reused across runs. The directory is
// created lazily on first use.
func WithPluginCacheDir(dir string) TerraformExecOption {
	return func(c *TerraformExecClient) {
		c.pluginCacheDir = dir
	}
}

// WithTerraformRunDir overrides the parent directory used to materialise
// the temporary working dirs (one per provider/version). When unset,
// os.TempDir() is used.
func WithTerraformRunDir(dir string) TerraformExecOption {
	return func(c *TerraformExecClient) {
		c.tfRunDir = dir
	}
}

// NewTerraformExecClient returns a TerraformExecClient configured with
// the supplied options. The configuration is validated lazily so a
// missing terraform binary surfaces only when a method is called.
func NewTerraformExecClient(opts ...TerraformExecOption) *TerraformExecClient {
	c := &TerraformExecClient{cached: map[string]*tfjson.ProviderSchema{}}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// DiscoverProvider implements Client. terraform-exec does not expose a
// "discover provider" call directly, so we delegate to the public
// registry HTTP API for version resolution. When the user passes a
// pinned version inside address (e.g. "hashicorp/random@3.6.0"), it is
// extracted from the address string.
func (c *TerraformExecClient) DiscoverProvider(ctx context.Context, address string) (*ProviderInfo, error) {
	addr, version := splitAddressVersion(address)
	d := NewHTTPDiscovery()
	return d.ResolveProvider(ctx, addr, version)
}

// ListResources implements Client. It triggers a schema dump (cached on
// success) and returns the resource + data source names found in the
// provider schema.
func (c *TerraformExecClient) ListResources(ctx context.Context, p ProviderInfo) ([]ResourceSummary, error) {
	schema, err := c.providerSchema(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceSummary, 0, len(schema.ResourceSchemas)+len(schema.DataSourceSchemas))
	for name := range schema.ResourceSchemas {
		out = append(out, ResourceSummary{Name: name, Kind: KindResource})
	}
	for name := range schema.DataSourceSchemas {
		out = append(out, ResourceSummary{Name: name, Kind: KindData})
	}
	return out, nil
}

// GetResourceSchema implements Client.
func (c *TerraformExecClient) GetResourceSchema(ctx context.Context, p ProviderInfo, name string, kind Kind) (*ResourceSchema, error) {
	schema, err := c.providerSchema(ctx, p)
	if err != nil {
		return nil, err
	}
	var (
		raw *tfjson.Schema
		ok  bool
	)
	switch kind {
	case KindResource:
		raw, ok = schema.ResourceSchemas[name]
	case KindData:
		raw, ok = schema.DataSourceSchemas[name]
	default:
		return nil, fmt.Errorf("registry: unknown kind %q", kind)
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s %s %q", ErrResourceNotFound, p.Namespace, p.Name, kind, name)
	}
	rs := translateSchema(name, kind, raw, providerSourceURL(p))
	return rs, nil
}

// providerSchema returns the schema for p, populating the in-memory
// cache on first hit. Concurrent callers share the work via mu. When a
// schemaCacheDir is configured, the disk cache is consulted before
// invoking terraform.
func (c *TerraformExecClient) providerSchema(ctx context.Context, p ProviderInfo) (*tfjson.ProviderSchema, error) {
	key := p.Address() + "@" + p.Version
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.cached[key]; ok {
		return s, nil
	}
	if s, ok := c.loadSchemaFromDisk(p); ok {
		c.cached[key] = s
		return s, nil
	}
	s, err := c.dumpProviderSchema(ctx, p)
	if err != nil {
		return nil, err
	}
	c.cached[key] = s
	c.saveSchemaToDisk(p, s)
	return s, nil
}

// loadSchemaFromDisk reads a previously-cached provider schema, if any.
// Cache misses (file absent or unreadable) silently return false; we do
// not surface IO errors here because the caller will fall back to a
// fresh terraform invocation.
func (c *TerraformExecClient) loadSchemaFromDisk(p ProviderInfo) (*tfjson.ProviderSchema, bool) {
	if c.schemaCacheDir == "" {
		return nil, false
	}
	path := c.schemaCachePath(p)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var s tfjson.ProviderSchema
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, false
	}
	return &s, true
}

// saveSchemaToDisk persists s to the schema cache; failures are logged
// implicitly via the error return but do not propagate (a write miss is
// an optimisation loss, not a correctness issue).
func (c *TerraformExecClient) saveSchemaToDisk(p ProviderInfo, s *tfjson.ProviderSchema) {
	if c.schemaCacheDir == "" {
		return
	}
	path := c.schemaCachePath(p)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// schemaCachePath returns the on-disk filename used for the supplied
// provider release.
func (c *TerraformExecClient) schemaCachePath(p ProviderInfo) string {
	return filepath.Join(c.schemaCacheDir, p.Namespace, p.Name, p.Version+".json")
}

// dumpProviderSchema runs terraform init + providers schema -json in a
// fresh working dir tied to p, then extracts the provider sub-schema.
func (c *TerraformExecClient) dumpProviderSchema(ctx context.Context, p ProviderInfo) (*tfjson.ProviderSchema, error) {
	workDir, cleanup, err := c.materialiseWorkDir(p)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tf, err := tfexec.NewTerraform(workDir, c.terraformBinary())
	if err != nil {
		return nil, fmt.Errorf("registry: locate terraform binary: %w", err)
	}
	if c.pluginCacheDir != "" {
		if err := os.MkdirAll(c.pluginCacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("registry: create plugin cache dir %s: %w", c.pluginCacheDir, err)
		}
		// tfexec.SetEnv REPLACES the child env wholesale; merge the
		// host environment so terraform can still find PATH-resolved
		// helpers like `getent` (used by the registry installer for
		// DNS resolution on Linux), HOME, TMPDIR, proxy vars, etc.
		env := mergedEnv(map[string]string{"TF_PLUGIN_CACHE_DIR": c.pluginCacheDir})
		if err := tf.SetEnv(env); err != nil {
			return nil, fmt.Errorf("registry: set terraform env: %w", err)
		}
	}

	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return nil, fmt.Errorf("terraform init for %s: %w", p.Address(), err)
	}

	schemas, err := tf.ProvidersSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform providers schema for %s: %w", p.Address(), err)
	}
	want := registryProviderKey(p)
	prov, ok := schemas.Schemas[want]
	if !ok {
		// Some terraform versions key the map by short address only;
		// also try "<ns>/<name>".
		if alt, altOK := schemas.Schemas[p.Address()]; altOK {
			return alt, nil
		}
		return nil, fmt.Errorf("registry: terraform did not return a schema for %q (got keys: %v)", want, mapKeys(schemas.Schemas))
	}
	return prov, nil
}

// materialiseWorkDir writes the minimal main.tf needed to install the
// requested provider into a unique subdirectory and returns it together
// with a cleanup function.
func (c *TerraformExecClient) materialiseWorkDir(p ProviderInfo) (string, func(), error) {
	parent := c.tfRunDir
	if parent == "" {
		parent = os.TempDir()
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, fmt.Errorf("registry: create run dir parent %s: %w", parent, err)
	}
	dir, err := os.MkdirTemp(parent, fmt.Sprintf("infra-composer-tf-%s-%s-", p.Name, p.Version))
	if err != nil {
		return "", nil, fmt.Errorf("registry: create run dir: %w", err)
	}
	main := filepath.Join(dir, "main.tf")
	contents := fmt.Sprintf(`terraform {
  required_providers {
    %s = {
      source  = "%s/%s"
      version = "%s"
    }
  }
}
`, p.Name, p.Namespace, p.Name, p.Version)
	if err := os.WriteFile(main, []byte(contents), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("registry: write main.tf: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// terraformBinary returns the configured binary path or, when unset,
// resolves "terraform" via PATH.
func (c *TerraformExecClient) terraformBinary() string {
	if c.binaryPath != "" {
		return c.binaryPath
	}
	if path, err := exec.LookPath("terraform"); err == nil {
		return path
	}
	return "terraform"
}

// registryProviderKey returns the canonical "registry.terraform.io/ns/name"
// key terraform uses to index the providers-schema output.
func registryProviderKey(p ProviderInfo) string {
	return "registry.terraform.io/" + p.Namespace + "/" + p.Name
}

// providerSourceURL synthesises the "source" URL recorded on each
// generated ModuleEntry so downstream tools can link back to the
// upstream provider repository.
func providerSourceURL(p ProviderInfo) string {
	return "https://registry.terraform.io/providers/" + p.Namespace + "/" + p.Name + "/" + p.Version
}

// splitAddressVersion accepts both "<ns>/<name>" and
// "<ns>/<name>@<version>" addresses and returns the address + optional
// version separately.
func splitAddressVersion(address string) (string, string) {
	for i := 0; i < len(address); i++ {
		if address[i] == '@' {
			return address[:i], address[i+1:]
		}
	}
	return address, ""
}

func mapKeys(m map[string]*tfjson.ProviderSchema) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// dumpJSONForDebug is a tiny helper used by errors and tests to surface
// the raw schema when something unexpected happens. Unused in normal
// flows; kept here so future debugging code doesn't need to add a
// separate dependency.
func dumpJSONForDebug(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<encode error: %v>", err)
	}
	return string(b)
}

// mergedEnv overlays overrides on top of os.Environ() and returns the
// flat map expected by tfexec.SetEnv. Necessary because SetEnv replaces
// the child environment wholesale; without merging we'd lose PATH,
// HOME, proxy vars, etc.
func mergedEnv(overrides map[string]string) map[string]string {
	env := make(map[string]string, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		if i := indexEqual(kv); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func indexEqual(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return i
		}
	}
	return -1
}

var _ = dumpJSONForDebug // silence "unused" until callers wire it in.
