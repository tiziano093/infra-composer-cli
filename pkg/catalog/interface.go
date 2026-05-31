package catalog

import (
	"fmt"
	"sort"
)

// InputView describes one input variable as it appears from the
// composer's perspective, after deciding whether it is wired to another
// module's output (and therefore not user-facing) or has to be supplied
// externally.
type InputView struct {
	Module      string
	Name        string
	Type        string
	Description string
	Required    bool
	Sensitive   bool
	Default     any
	// Wired is true when the variable is satisfied by another module
	// in the requested set via Variable.References. Wired inputs are
	// hidden from --required-only views because they are auto-resolved
	// by the compose pipeline.
	Wired  bool
	Source *VariableReference
}

// OutputView describes one output exposed by a module in the requested
// set. The Module field is included so aggregate listings stay
// unambiguous.
type OutputView struct {
	Module      string
	Name        string
	Description string
	Sensitive   bool
}

// ModuleInterface bundles the inputs and outputs of a single module
// after extraction, in the order they appear in the schema.
type ModuleInterface struct {
	Module  string
	Inputs  []InputView
	Outputs []OutputView
}

// Interface is the aggregated view returned by ExtractInterface for a
// requested subset of modules. Modules preserves the input order, while
// AllInputs and AllOutputs are flattened lists sorted by (module, name)
// for deterministic rendering.
type Interface struct {
	Modules    []ModuleInterface
	AllInputs  []InputView
	AllOutputs []OutputView
}

// ExtractOptions configures ExtractInterface.
type ExtractOptions struct {
	// Modules is the ordered subset of module names to inspect.
	Modules []string
	// RequiredOnly drops every input that is not marked required, and
	// also drops wired inputs (they are not user-facing).
	RequiredOnly bool
	// Full includes wired inputs in the per-module / aggregate listing.
	// When false (the default) wired inputs are hidden from the
	// aggregate AllInputs list but kept in the per-module Inputs view
	// so callers can still discover the wiring source.
	Full bool
}

// ExtractInterface inspects the requested modules in s and returns the
// composer-facing interface: which inputs the user must provide, which
// inputs are auto-wired between modules, and which outputs the resulting
// stack will expose.
//
// Modules are looked up by name; an unknown module name yields an
// ErrUnknownModule-wrapped error so callers can surface a precise
// diagnostic. Module names are deduplicated while preserving first
// occurrence order.
func ExtractInterface(s *Schema, opts ExtractOptions) (*Interface, error) {
	if s == nil {
		return nil, fmt.Errorf("interface: nil schema")
	}
	byName := make(map[string]*ModuleEntry, len(s.Modules))
	for i := range s.Modules {
		m := &s.Modules[i]
		byName[m.Name] = m
	}

	selected := make(map[string]struct{}, len(opts.Modules))
	ordered := make([]string, 0, len(opts.Modules))
	for _, name := range opts.Modules {
		if _, dup := selected[name]; dup {
			continue
		}
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownModule, name)
		}
		selected[name] = struct{}{}
		ordered = append(ordered, name)
	}

	out := &Interface{Modules: make([]ModuleInterface, 0, len(ordered))}
	for _, name := range ordered {
		m := byName[name]
		mi := ModuleInterface{Module: name}
		for _, v := range m.Variables {
			view := InputView{
				Module:      name,
				Name:        v.Name,
				Type:        v.Type,
				Description: v.Description,
				Required:    v.Required,
				Sensitive:   v.Sensitive,
				Default:     v.Default,
			}
			if src := pickWiredSource(v.References, selected); src != nil {
				view.Wired = true
				view.Source = src
			}
			if opts.RequiredOnly && (!view.Required || view.Wired) {
				continue
			}
			mi.Inputs = append(mi.Inputs, view)
		}
		for _, o := range m.Outputs {
			mi.Outputs = append(mi.Outputs, OutputView{
				Module: name, Name: o.Name,
				Description: o.Description, Sensitive: o.Sensitive,
			})
		}
		out.Modules = append(out.Modules, mi)
	}

	for _, mi := range out.Modules {
		for _, in := range mi.Inputs {
			if !opts.Full && in.Wired {
				continue
			}
			out.AllInputs = append(out.AllInputs, in)
		}
		out.AllOutputs = append(out.AllOutputs, mi.Outputs...)
	}
	sort.SliceStable(out.AllInputs, func(i, j int) bool {
		a, b := out.AllInputs[i], out.AllInputs[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		return a.Name < b.Name
	})
	sort.SliceStable(out.AllOutputs, func(i, j int) bool {
		a, b := out.AllOutputs[i], out.AllOutputs[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		return a.Name < b.Name
	})
	return out, nil
}

// pickWiredSource returns the first reference whose target module is
// part of the requested set, or nil when none of the references can be
// satisfied internally. This conservative rule means an input remains
// user-facing whenever its dependency is missing from the composition.
func pickWiredSource(refs []VariableReference, selected map[string]struct{}) *VariableReference {
	for i := range refs {
		if _, ok := selected[refs[i].Module]; ok {
			r := refs[i]
			return &r
		}
	}
	return nil
}
