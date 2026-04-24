package terraform

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// PlanOptions parameterise Plan(). Modules is the ordered list of
// catalog entries the user wants to materialise; each entry may be
// bare (`aws_vpc`) or kind-qualified (`data.aws_vpc`,
// `resource.aws_vpc`) to disambiguate when both exist.
type PlanOptions struct {
	Modules []string
}

// ErrAmbiguousSelection is returned when a bare module name resolves
// to more than one catalog entry (e.g. both a resource and a data
// source share the name) and the user did not pass a kind prefix.
var ErrAmbiguousSelection = errors.New("compose: ambiguous module selection")

// entryKey identifies a catalog entry by its (kind, name) pair so a
// single catalog can carry resource and data variants of the same name
// without clashing.
type entryKey struct {
	kind catalog.ModuleType
	name string
}

// Plan turns a catalog Schema and a module selection into a
// ComposePlan: one GeneratedModule per requested entry, with full
// variables/outputs and provider metadata baked in.
//
// Selection rules:
//   - "<kind>.<name>" picks an exact entry (kind ∈ {resource, data}).
//   - "<name>" defaults to the resource entry; falls back to data when
//     no resource exists; errors with ErrAmbiguousSelection only when
//     callers ask for an unsupported kind in the future.
//   - Duplicates in opts.Modules are de-duplicated by (kind, name).
//
// Plan never mutates the input schema and never reaches outside the
// catalog (no source resolution, no remote calls).
func Plan(s *catalog.Schema, opts PlanOptions) (*ComposePlan, error) {
	if s == nil {
		return nil, fmt.Errorf("compose: nil schema")
	}
	if len(opts.Modules) == 0 {
		return nil, fmt.Errorf("compose: no modules selected")
	}

	byKey := make(map[entryKey]*catalog.ModuleEntry, len(s.Modules))
	byName := make(map[string][]*catalog.ModuleEntry, len(s.Modules))
	for i := range s.Modules {
		m := &s.Modules[i]
		byKey[entryKey{m.Type, m.Name}] = m
		byName[m.Name] = append(byName[m.Name], m)
	}

	plan := &ComposePlan{
		Provider:        s.Provider,
		ProviderVersion: s.ProviderVersion,
	}
	providerLocal := ProviderLocalName(s.Provider)
	versionPin := VersionConstraint(s.ProviderVersion)

	seen := make(map[entryKey]struct{}, len(opts.Modules))
	for _, raw := range opts.Modules {
		entry, k, err := resolveEntry(raw, byKey, byName)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		plan.Modules = append(plan.Modules, buildModule(entry, providerLocal, s.Provider, versionPin))
	}
	return plan, nil
}

// resolveEntry interprets one selection token and returns the matched
// catalog entry plus its composite key.
func resolveEntry(raw string, byKey map[entryKey]*catalog.ModuleEntry, byName map[string][]*catalog.ModuleEntry) (*catalog.ModuleEntry, entryKey, error) {
	if dot := strings.IndexByte(raw, '.'); dot > 0 {
		kindPart := raw[:dot]
		namePart := raw[dot+1:]
		var kind catalog.ModuleType
		switch kindPart {
		case "resource":
			kind = catalog.ModuleTypeResource
		case "data":
			kind = catalog.ModuleTypeData
		default:
			return nil, entryKey{}, fmt.Errorf("%w: unknown kind prefix %q (use \"resource.\" or \"data.\")", catalog.ErrUnknownModule, kindPart)
		}
		k := entryKey{kind, namePart}
		entry, ok := byKey[k]
		if !ok {
			return nil, entryKey{}, fmt.Errorf("%w: %s", catalog.ErrUnknownModule, raw)
		}
		return entry, k, nil
	}
	candidates, ok := byName[raw]
	if !ok || len(candidates) == 0 {
		return nil, entryKey{}, fmt.Errorf("%w: %q", catalog.ErrUnknownModule, raw)
	}
	if len(candidates) == 1 {
		c := candidates[0]
		return c, entryKey{c.Type, c.Name}, nil
	}
	for _, c := range candidates {
		if c.Type == catalog.ModuleTypeResource {
			return c, entryKey{c.Type, c.Name}, nil
		}
	}
	// More than one candidate but no resource preferred — surface the
	// ambiguity instead of guessing.
	return nil, entryKey{}, fmt.Errorf("%w: %q matches multiple kinds; use \"resource.%s\" or \"data.%s\"",
		ErrAmbiguousSelection, raw, raw, raw)
}

// buildModule converts one catalog entry into a fully self-describing
// GeneratedModule, applying Cut B handling for nested-block variables.
func buildModule(m *catalog.ModuleEntry, providerLocal, providerSource, versionPin string) GeneratedModule {
	gm := GeneratedModule{
		ResourceType:              m.Name,
		Kind:                      m.Type,
		LocalName:                 LocalBlockName,
		Description:               m.Description,
		Group:                     m.Group,
		ProviderLocalName:         providerLocal,
		ProviderSource:            providerSource,
		ProviderVersionConstraint: versionPin,
	}

	// Pre-build lookup from parent block name to child attrs. Populated
	// only for new-format catalogs that carry catalog.Variable.Attrs.
	attrsByParent := make(map[string][]catalog.VariableAttr, len(m.Variables))
	for _, v := range m.Variables {
		if len(v.Attrs) > 0 {
			attrsByParent[v.Name] = v.Attrs
		}
	}

	nestedCollapsed := 0
	skippedChildren := 0
	for _, v := range m.Variables {
		if IsNestedAttrName(v.Name) {
			// Cut B: drop dotted child attributes. Their parent
			// block is still rendered as a single any-typed
			// variable below.
			skippedChildren++
			continue
		}
		mv := ModuleVariable{
			Name:        v.Name,
			Type:        v.Type,
			Description: v.Description,
			Default:     v.Default,
			Required:    v.Required,
			Sensitive:   v.Sensitive,
			References:  append([]catalog.VariableReference(nil), v.References...),
		}
		if IsNestedBlockType(v.Type) {
			mv.Nested = true
			nestedCollapsed++
			if catalogAttrs, ok := attrsByParent[v.Name]; ok {
				mv.Attrs = make([]NestedAttr, len(catalogAttrs))
				for i, a := range catalogAttrs {
					mv.Attrs[i] = NestedAttr{Name: a.Name, Type: a.Type, Required: a.Required}
				}
			}
		}
		gm.Variables = append(gm.Variables, mv)
	}
	for _, o := range m.Outputs {
		// Skip dotted output names too — same reasoning as inputs.
		if IsNestedAttrName(o.Name) {
			continue
		}
		gm.Outputs = append(gm.Outputs, ModuleOutput{
			Name:        o.Name,
			Description: o.Description,
			Sensitive:   o.Sensitive,
		})
	}
	if nestedCollapsed > 0 {
		gm.Warnings = append(gm.Warnings,
			fmt.Sprintf("%d nested block(s) rendered as untyped (any) variables; complete the matching block(s) in main.tf manually",
				nestedCollapsed))
	}
	if skippedChildren > 0 {
		gm.Warnings = append(gm.Warnings,
			fmt.Sprintf("%d nested sub-attribute(s) hidden (Cut B); use the parent block's variable to set them",
				skippedChildren))
	}
	return gm
}
