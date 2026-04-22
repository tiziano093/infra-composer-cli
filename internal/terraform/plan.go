package terraform

import (
	"errors"
	"fmt"
	"sort"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// PlanOptions parameterise Plan().
type PlanOptions struct {
	// Modules is the user-requested module subset, in declaration order.
	// Duplicates are deduplicated; the resulting plan honours the order
	// of first occurrence as the alphabetical tie-breaker for topo sort.
	Modules []string
	// Resolver maps each catalog ModuleEntry to its concrete Terraform
	// source. nil falls back to PlaceholderResolver so callers can
	// produce a (clearly-flagged) plan even before git wiring exists.
	Resolver SourceResolver
}

// ErrAmbiguousReference is returned when a single variable has more
// than one VariableReference whose target module is part of the
// selected set: the generator cannot decide which output to wire and
// the user must restrict the catalog or the selection.
var ErrAmbiguousReference = errors.New("compose: ambiguous variable reference")

// Plan turns a catalog Schema and a module selection into a
// deterministic ComposePlan. Wiring rules:
//
//   - A variable's value is wired iff exactly one of its References
//     points to a module that is also in the selected set; multiple
//     candidates yield ErrAmbiguousReference.
//   - References to modules NOT in the selection are treated as
//     external user inputs but recorded as a per-module warning so the
//     compose command can surface them clearly.
//   - Modules are returned in dependency-first order via DFS post-order
//     traversal; ties broken alphabetically for stable rendering.
//
// Plan never mutates the input schema.
func Plan(s *catalog.Schema, opts PlanOptions) (*ComposePlan, error) {
	if s == nil {
		return nil, fmt.Errorf("compose: nil schema")
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = PlaceholderResolver{}
	}

	byName := make(map[string]*catalog.ModuleEntry, len(s.Modules))
	for i := range s.Modules {
		byName[s.Modules[i].Name] = &s.Modules[i]
	}

	selected := make(map[string]struct{}, len(opts.Modules))
	ordered := make([]string, 0, len(opts.Modules))
	for _, name := range opts.Modules {
		if _, dup := selected[name]; dup {
			continue
		}
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("%w: %q", catalog.ErrUnknownModule, name)
		}
		selected[name] = struct{}{}
		ordered = append(ordered, name)
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("compose: no modules selected")
	}

	sortedNames, err := topoSort(ordered, byName, selected)
	if err != nil {
		return nil, err
	}

	plan := &ComposePlan{
		Provider:        s.Provider,
		ProviderName:    ProviderLocalName(s.Provider),
		ProviderVersion: s.ProviderVersion,
		Modules:         make([]ResolvedModule, 0, len(sortedNames)),
	}

	for _, name := range sortedNames {
		m := byName[name]
		spec, err := resolver.Resolve(m)
		if err != nil {
			return nil, fmt.Errorf("resolve source for %s: %w", name, err)
		}
		rm := ResolvedModule{
			Module:   m,
			Instance: InstanceName(name),
			Source:   spec,
		}
		for _, v := range m.Variables {
			wired, warn, err := pickWiring(v, name, selected, byName)
			if err != nil {
				return nil, err
			}
			if warn != "" {
				rm.UnwiredWarnings = append(rm.UnwiredWarnings, warn)
				plan.Warnings = append(plan.Warnings, warn)
			}
			if wired != nil {
				rm.WiredInputs = append(rm.WiredInputs, *wired)
				continue
			}
			rm.ExternalInputs = append(rm.ExternalInputs, ExternalInput{
				VarName:     ExternalVarName(name, v.Name),
				LocalName:   v.Name,
				Type:        v.Type,
				Description: v.Description,
				Required:    v.Required,
				Sensitive:   v.Sensitive,
				Default:     v.Default,
			})
		}
		for _, o := range m.Outputs {
			rm.StackOutputs = append(rm.StackOutputs, StackOutput{
				ExportedName: StackOutputName(name, o.Name),
				Module:       name,
				Instance:     rm.Instance,
				Output:       o.Name,
				Description:  o.Description,
				Sensitive:    o.Sensitive,
			})
		}
		plan.Modules = append(plan.Modules, rm)
	}

	if spec, ok := placeholderWarning(plan); ok {
		plan.Warnings = append(plan.Warnings, spec)
	}
	return plan, nil
}

// pickWiring decides what to do with one variable in a selected
// module. Returns (wired, warning, err) — at most one of wired/warning
// is non-zero. warning is purely informational; err aborts planning.
func pickWiring(v catalog.Variable, owner string, selected map[string]struct{}, byName map[string]*catalog.ModuleEntry) (*WiredInput, string, error) {
	if len(v.References) == 0 {
		return nil, "", nil
	}
	var matched []catalog.VariableReference
	var unselected []catalog.VariableReference
	for _, ref := range v.References {
		if ref.Module == owner {
			continue
		}
		if _, ok := selected[ref.Module]; ok {
			matched = append(matched, ref)
		} else {
			unselected = append(unselected, ref)
		}
	}
	switch len(matched) {
	case 0:
		if len(unselected) == 0 {
			return nil, "", nil
		}
		warn := fmt.Sprintf(
			"%s.%s references %s which is not in the selection; materialised as external variable %q",
			owner, v.Name, refList(unselected), ExternalVarName(owner, v.Name),
		)
		return nil, warn, nil
	case 1:
		m := byName[matched[0].Module]
		_ = m
		return &WiredInput{
			VarName:      v.Name,
			FromModule:   matched[0].Module,
			FromInstance: InstanceName(matched[0].Module),
			FromOutput:   matched[0].Output,
		}, "", nil
	default:
		return nil, "", fmt.Errorf("%w: %s.%s has %d candidate sources in the selection (%s); restrict the selection or pin the reference",
			ErrAmbiguousReference, owner, v.Name, len(matched), refList(matched))
	}
}

func refList(refs []catalog.VariableReference) string {
	out := ""
	for i, r := range refs {
		if i > 0 {
			out += ", "
		}
		out += r.Module + "." + r.Output
	}
	return out
}

// topoSort returns the selected module names ordered so each module
// appears after every dependency (consumer-after-producer) it has in
// the same set. Ties are broken alphabetically for stability.
func topoSort(selectedOrder []string, byName map[string]*catalog.ModuleEntry, selected map[string]struct{}) ([]string, error) {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	colour := make(map[string]int, len(selectedOrder))
	out := make([]string, 0, len(selectedOrder))

	var visit func(n string, stack []string) error
	visit = func(n string, stack []string) error {
		switch colour[n] {
		case grey:
			cyc := append([]string(nil), stack...)
			cyc = append(cyc, n)
			return fmt.Errorf("compose: dependency cycle in selection: %s", joinPath(cyc))
		case black:
			return nil
		}
		colour[n] = grey
		// Collect referenced modules in the selection, sorted alphabetically
		// so the traversal order does not leak schema authoring order.
		deps := make([]string, 0)
		seenDep := make(map[string]struct{})
		for _, v := range byName[n].Variables {
			for _, ref := range v.References {
				if ref.Module == n {
					continue
				}
				if _, ok := selected[ref.Module]; !ok {
					continue
				}
				if _, dup := seenDep[ref.Module]; dup {
					continue
				}
				seenDep[ref.Module] = struct{}{}
				deps = append(deps, ref.Module)
			}
		}
		sort.Strings(deps)
		for _, d := range deps {
			if err := visit(d, append(stack, n)); err != nil {
				return err
			}
		}
		colour[n] = black
		out = append(out, n)
		return nil
	}

	// Walk roots in alphabetical order so the postorder result is
	// stable regardless of how the user listed --modules.
	roots := append([]string(nil), selectedOrder...)
	sort.Strings(roots)
	for _, r := range roots {
		if err := visit(r, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func joinPath(p []string) string {
	out := ""
	for i, n := range p {
		if i > 0 {
			out += " → "
		}
		out += n
	}
	return out
}

func placeholderWarning(plan *ComposePlan) (string, bool) {
	count := 0
	for _, m := range plan.Modules {
		if m.Source.Kind == SourcePlaceholder {
			count++
		}
	}
	if count == 0 {
		return "", false
	}
	return fmt.Sprintf("%d module(s) have placeholder sources; edit main.tf before running terraform init", count), true
}
