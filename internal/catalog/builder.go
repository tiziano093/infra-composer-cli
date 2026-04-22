package catalog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
)

// BuildOptions parameterise Builder.Build. Provider is the only required
// field; everything else has sensible defaults.
type BuildOptions struct {
	// Provider is the canonical "<namespace>/<name>" address.
	Provider string

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
		if _, dup := seen[s.Name]; dup {
			return nil, fmt.Errorf("registry returned duplicate module %q for %s", s.Name, prov.Address())
		}
		seen[s.Name] = struct{}{}

		if !s.Kind.IsValid() {
			return nil, fmt.Errorf("registry returned invalid kind %q for module %q", s.Kind, s.Name)
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
			m.Variables[i] = Variable{
				Name:        in.Name,
				Type:        in.Type,
				Description: in.Description,
				Default:     in.Default,
				Required:    in.Required,
				Sensitive:   in.Sensitive,
			}
		}
	}
	if len(rs.Outputs) > 0 {
		m.Outputs = make([]Output, len(rs.Outputs))
		for i, o := range rs.Outputs {
			m.Outputs[i] = Output{
				Name:        o.Name,
				Description: o.Description,
				Sensitive:   o.Sensitive,
			}
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
