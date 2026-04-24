package catalog

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
)

// BuildOptions parameterise Builder.Build. Provider is the only required
// field; everything else has sensible defaults.
type BuildOptions struct {
	// Provider is the canonical "<namespace>/<name>" address.
	Provider string

	// Include, when non-empty, restricts the build to module names
	// matching at least one of the supplied patterns. Patterns are glob
	// expressions (filepath.Match syntax) compared against the
	// resource/data source name (e.g. "aws_vpc*", "*_subnet*").
	Include []string

	// Exclude removes any module whose name matches at least one
	// pattern, even if it also matches Include. Same syntax as Include.
	Exclude []string

	// Now allows tests to pin GeneratedAt; defaults to time.Now().UTC().
	Now func() time.Time
}

// Builder turns a registry.Client into a validated catalog Schema using
// the discover → list → fetch → normalize → validate pipeline.
type Builder struct {
	client registry.Client
}

// NewBuilder returns a Builder that pulls provider data from client.
func NewBuilder(client registry.Client) *Builder {
	return &Builder{client: client}
}

// Build runs the catalog build pipeline end-to-end.
//
// Steps:
//  1. Discover the provider (resolves the version to pin).
//  2. List every resource and data source.
//  3. Fetch the full schema for each entry.
//  4. Normalize provider DTOs to catalog ModuleEntry values.
//  5. Validate the assembled Schema; return *ValidationError on failure.
//
// Build is deterministic: modules are sorted by name (resources first,
// then data sources, with a secondary alphabetical key) so successive
// runs against the same fixtures produce byte-identical exports.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*Schema, error) {
	if b == nil || b.client == nil {
		return nil, fmt.Errorf("catalog: builder has no registry client")
	}
	if opts.Provider == "" {
		return nil, fmt.Errorf("catalog: BuildOptions.Provider is required")
	}

	prov, err := b.client.DiscoverProvider(ctx, opts.Provider)
	if err != nil {
		return nil, fmt.Errorf("discover provider %s: %w", opts.Provider, err)
	}

	summaries, err := b.client.ListResources(ctx, *prov)
	if err != nil {
		return nil, fmt.Errorf("list resources for %s: %w", prov.Address(), err)
	}

	modules := make([]ModuleEntry, 0, len(summaries))
	seen := make(map[string]struct{}, len(summaries))
	for _, s := range summaries {
		if !s.Kind.IsValid() {
			return nil, fmt.Errorf("registry returned invalid kind %q for module %q", s.Kind, s.Name)
		}
		// Resources and data sources legitimately share names
		// (e.g., azurerm_resource_group exists as both); namespace
		// the dedup key by Kind to avoid false positives.
		key := string(s.Kind) + ":" + s.Name
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("registry returned duplicate %s %q for %s", s.Kind, s.Name, prov.Address())
		}
		seen[key] = struct{}{}

		if !shouldIncludeModule(s.Name, opts.Include, opts.Exclude) {
			continue
		}
		rs, err := b.client.GetResourceSchema(ctx, *prov, s.Name, s.Kind)
		if err != nil {
			return nil, fmt.Errorf("fetch schema for %s/%s %s: %w", prov.Address(), s.Kind, s.Name, err)
		}
		modules = append(modules, normalizeModule(rs))
	}

	sortModules(modules)

	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	s := &Schema{
		SchemaVersion:   SchemaVersion,
		Provider:        prov.Address(),
		ProviderVersion: prov.Version,
		GeneratedAt:     now().UTC(),
		Modules:         modules,
	}

	if err := s.Validate(); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			return s, fmt.Errorf("registry produced invalid catalog: %w", ve)
		}
		return s, err
	}
	return s, nil
}

// normalizeModule maps a registry ResourceSchema onto a catalog
// ModuleEntry. Empty optional fields are dropped so JSON output stays
// minimal.
func normalizeModule(rs *registry.ResourceSchema) ModuleEntry {
	m := ModuleEntry{
		Name:        rs.Name,
		Type:        ModuleType(rs.Kind),
		Group:       rs.Group,
		Source:      rs.Source,
		Description: rs.Description,
	}
	if len(rs.Inputs) > 0 {
		m.Variables = make([]Variable, len(rs.Inputs))
		for i, in := range rs.Inputs {
			v := Variable{
				Name:        in.Name,
				Type:        in.Type,
				Description: in.Description,
				Default:     in.Default,
				Required:    in.Required,
				Sensitive:   in.Sensitive,
			}
			if len(in.Attrs) > 0 {
				v.Attrs = make([]VariableAttr, len(in.Attrs))
				for j, a := range in.Attrs {
					v.Attrs[j] = VariableAttr{Name: a.Name, Type: a.Type, Required: a.Required}
				}
			}
			m.Variables[i] = v
		}
	}
	if len(rs.Outputs) > 0 {
		filtered := make([]Output, 0, len(rs.Outputs))
		for _, o := range rs.Outputs {
			// Dotted names (e.g. "timeouts.read") are nested sub-attributes
			// produced by the registry translator. They are not valid Terraform
			// identifiers and are skipped at generation time anyway; drop them
			// here so the catalog validates cleanly.
			if strings.Contains(o.Name, ".") {
				continue
			}
			filtered = append(filtered, Output{
				Name:        o.Name,
				Description: o.Description,
				Sensitive:   o.Sensitive,
			})
		}
		if len(filtered) > 0 {
			m.Outputs = filtered
		}
	}
	return m
}

// sortModules orders modules so resources come before data sources, with
// each group sorted alphabetically by name. Stable order keeps catalog
// exports diffable across runs.
func sortModules(modules []ModuleEntry) {
	sort.SliceStable(modules, func(i, j int) bool {
		ti, tj := typeRank(modules[i].Type), typeRank(modules[j].Type)
		if ti != tj {
			return ti < tj
		}
		return modules[i].Name < modules[j].Name
	})
}

func typeRank(t ModuleType) int {
	switch t {
	case ModuleTypeResource:
		return 0
	case ModuleTypeData:
		return 1
	default:
		return 2
	}
}

// shouldIncludeModule applies the include/exclude glob filters to a
// resource name. Empty include lists mean "everything"; exclude wins
// when both lists match.
func shouldIncludeModule(name string, include, exclude []string) bool {
	if matchAny(name, exclude) {
		return false
	}
	if len(include) == 0 {
		return true
	}
	return matchAny(name, include)
}

func matchAny(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if ok, err := filepath.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}
