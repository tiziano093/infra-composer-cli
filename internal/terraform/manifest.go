package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// ManifestFileName is the on-disk name of the compose manifest emitted
// at the root of --output-dir alongside per-module folders.
const ManifestFileName = "modules.json"

// ManifestSchemaVersion pins the compose manifest format version. The
// value is independent from catalog.SchemaVersion so the two formats
// can evolve separately.
const ManifestSchemaVersion = "1.0"

// ComposeManifest is the machine-readable contract describing what the
// compose command produced. It is written to ManifestFileName at the
// root of the output directory after the per-module folders.
//
// Consumers (humans scanning the file, CI scripts, IDE tooling) get a
// single entry point summarising every module: its on-disk folder, the
// provider it wraps and the full variables/outputs/references set
// carried over verbatim from the source catalog.
type ComposeManifest struct {
	// SchemaVersion identifies the manifest format; always
	// ManifestSchemaVersion for documents produced by this CLI.
	SchemaVersion string `json:"schema_version"`

	// GeneratedAt is the UTC timestamp when the compose command ran.
	GeneratedAt time.Time `json:"generated_at"`

	// Provider is the canonical "<namespace>/<name>" address carried
	// over from the source catalog.
	Provider string `json:"provider"`

	// ProviderVersion is the provider release the underlying catalog
	// was generated from.
	ProviderVersion string `json:"provider_version"`

	// SourceCatalog is the path to the catalog schema.json used by the
	// compose run, recorded so downstream tools can trace back to the
	// original contract. Empty when unknown.
	SourceCatalog string `json:"source_catalog,omitempty"`

	// Modules lists every composed module in the order produced by
	// Plan(). Each entry embeds the catalog.ModuleEntry verbatim so
	// consumers reuse the existing catalog type.
	Modules []ComposedModule `json:"modules"`
}

// ComposedModule couples a generated module folder with the catalog
// entry it was materialised from.
type ComposedModule struct {
	// Path is the on-disk location of the module folder, relative to
	// the compose output directory (e.g. "./azurerm_resource_group").
	Path string `json:"path"`

	// Entry is the catalog entry the module was generated from. It
	// carries the full variable/output/reference metadata so consumers
	// can drive downstream tooling from a single file.
	Entry catalog.ModuleEntry `json:"entry"`
}

// BuildManifest produces the ComposeManifest for plan. sourceCatalog is
// recorded verbatim (typically the --schema path used by the compose
// command); pass "" when unknown. The caller is responsible for
// rendering and writing the manifest via RenderManifestFile.
func BuildManifest(plan *ComposePlan, sourceCatalog string) *ComposeManifest {
	if plan == nil {
		return nil
	}
	modules := make([]ComposedModule, 0, len(plan.Modules))
	for _, m := range plan.Modules {
		modules = append(modules, ComposedModule{
			Path:  "./" + m.ResourceType,
			Entry: composedEntryFromModule(m),
		})
	}
	return &ComposeManifest{
		SchemaVersion:   ManifestSchemaVersion,
		GeneratedAt:     time.Now().UTC(),
		Provider:        plan.Provider,
		ProviderVersion: plan.ProviderVersion,
		SourceCatalog:   sourceCatalog,
		Modules:         modules,
	}
}

// composedEntryFromModule rebuilds a catalog.ModuleEntry from the
// resolved GeneratedModule so the manifest reflects the post-plan view
// (e.g. with Cut B children already folded into their parent block
// variable) rather than the raw catalog entry.
func composedEntryFromModule(m GeneratedModule) catalog.ModuleEntry {
	vars := make([]catalog.Variable, 0, len(m.Variables))
	for _, v := range m.Variables {
		vars = append(vars, catalog.Variable{
			Name:        v.Name,
			Type:        variableTypeForManifest(v),
			Description: v.Description,
			Default:     v.Default,
			Required:    v.Required,
			Sensitive:   v.Sensitive,
			References:  append([]catalog.VariableReference(nil), v.References...),
		})
	}
	outs := make([]catalog.Output, 0, len(m.Outputs))
	for _, o := range m.Outputs {
		outs = append(outs, catalog.Output{
			Name:        o.Name,
			Description: o.Description,
			Sensitive:   o.Sensitive,
		})
	}
	return catalog.ModuleEntry{
		Name:        m.ResourceType,
		Type:        m.Kind,
		Group:       m.Group,
		Description: m.Description,
		Variables:   vars,
		Outputs:     outs,
	}
}

// variableTypeForManifest mirrors the type normalisation applied by the
// variables.tf renderer so the manifest matches the on-disk module.
func variableTypeForManifest(v ModuleVariable) string {
	if v.Nested || v.Type == "" {
		return "any"
	}
	return v.Type
}

// RenderManifestFile serialises the manifest as pretty-printed JSON and
// wraps it in a GeneratedFile at ManifestFileName so the compose
// command can write it through the same atomic path as the per-module
// files.
func RenderManifestFile(m *ComposeManifest) (GeneratedFile, error) {
	if m == nil {
		return GeneratedFile{}, fmt.Errorf("terraform: nil manifest")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return GeneratedFile{}, fmt.Errorf("encode manifest: %w", err)
	}
	return GeneratedFile{
		Path:    ManifestFileName,
		Content: buf.Bytes(),
	}, nil
}
