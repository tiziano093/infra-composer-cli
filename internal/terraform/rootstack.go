package terraform

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// RootStackFiles is the fixed set of files emitted at the root of the
// compose output directory when plan.EmitRootStack is true.
var RootStackFiles = []string{
	"versions.tf",
	"providers.tf",
	"variables.tf",
	"locals.tf",
	"main.tf",
	"outputs.tf",
}

// RenderRootStack renders the top-level stack files that instantiate
// every GeneratedModule in plan via a `module` block. User-facing
// inputs (variables not satisfied by another module in the plan) are
// surfaced as root variables; cross-module references are wired
// directly from `module.<dep>.<output>`. Every module output is
// re-exported at stack level for external consumption.
//
// Paths in the returned files are at the root of the compose output
// directory (no folder prefix). Content is deterministic for a given
// plan so callers can rely on stable diffs.
func RenderRootStack(plan *ComposePlan) ([]GeneratedFile, error) {
	if plan == nil {
		return nil, fmt.Errorf("terraform: nil plan")
	}
	if !plan.EmitRootStack {
		return nil, nil
	}

	wiring := buildWiring(plan)
	rootVars := collectRootVariables(plan, wiring)
	order := orderedModules(plan)

	versionsBody, err := renderRootVersions(plan)
	if err != nil {
		return nil, fmt.Errorf("render versions.tf: %w", err)
	}
	providersBody, err := renderRootProviders(plan)
	if err != nil {
		return nil, fmt.Errorf("render providers.tf: %w", err)
	}
	variablesBody, err := renderRootVariables(rootVars)
	if err != nil {
		return nil, fmt.Errorf("render variables.tf: %w", err)
	}
	mainBody, err := renderRootMain(plan, order, wiring, rootVars)
	if err != nil {
		return nil, fmt.Errorf("render main.tf: %w", err)
	}
	outputsBody, err := renderRootOutputs(plan, order)
	if err != nil {
		return nil, fmt.Errorf("render outputs.tf: %w", err)
	}
	localsBody := renderRootLocals()

	files := []GeneratedFile{
		{Path: "versions.tf", Content: versionsBody},
		{Path: "providers.tf", Content: providersBody},
		{Path: "variables.tf", Content: variablesBody},
		{Path: "locals.tf", Content: localsBody},
		{Path: "main.tf", Content: mainBody},
		{Path: "outputs.tf", Content: outputsBody},
	}
	return files, nil
}

// wiringEntry records, per (module, variable), how the root-stack main.tf
// should assign the value. When Ref is non-nil the assignment reads
// `module.<Ref.Module>.<Ref.Output>`; otherwise it reads `var.<RootName>`.
type wiringEntry struct {
	Ref *catalog.VariableReference
	// RootName is the root-stack variable name that satisfies this
	// module input when Ref is nil. May differ from the module-local
	// variable name when collisions across modules forced a rename.
	RootName string
}

// wiring is indexed by [module.ResourceType][variable.Name].
type wiring map[string]map[string]wiringEntry

// rootVariable is a single aggregated root-stack input variable.
type rootVariable struct {
	Name        string
	Type        string
	Description string
	Default     any
	Required    bool
	Sensitive   bool
	// Source captures the originating (module, variable) pair; populated
	// to keep the description traceable when a collision forced a rename.
	SourceModule string
	SourceVar    string
	Nested       bool
}

// buildWiring walks plan.Modules and determines, for every non-wired
// input, the final root variable name to use, plus records the wiring
// target for every cross-module reference whose target is in the plan.
func buildWiring(plan *ComposePlan) wiring {
	inPlan := make(map[string]struct{}, len(plan.Modules))
	for _, m := range plan.Modules {
		inPlan[m.ResourceType] = struct{}{}
	}
	w := make(wiring, len(plan.Modules))

	// Detect name collisions on non-wired variables across modules so we
	// can disambiguate by prefixing the module name. Wired variables are
	// skipped because they never reach the root variables.tf.
	occurrences := make(map[string]int)
	for _, m := range plan.Modules {
		for _, v := range m.Variables {
			if ref := pickInPlanRef(v.References, inPlan); ref != nil {
				continue
			}
			occurrences[v.Name]++
		}
	}

	for _, m := range plan.Modules {
		w[m.ResourceType] = make(map[string]wiringEntry, len(m.Variables))
		for _, v := range m.Variables {
			if ref := pickInPlanRef(v.References, inPlan); ref != nil {
				refCopy := *ref
				w[m.ResourceType][v.Name] = wiringEntry{Ref: &refCopy}
				continue
			}
			rootName := v.Name
			if occurrences[v.Name] > 1 {
				rootName = m.ResourceType + "_" + v.Name
			}
			w[m.ResourceType][v.Name] = wiringEntry{RootName: rootName}
		}
	}
	return w
}

// pickInPlanRef returns the first reference whose target module is
// part of the current plan, mirroring catalog.pickWiredSource behaviour.
func pickInPlanRef(refs []catalog.VariableReference, inPlan map[string]struct{}) *catalog.VariableReference {
	for i := range refs {
		if _, ok := inPlan[refs[i].Module]; ok {
			return &refs[i]
		}
	}
	return nil
}

// collectRootVariables aggregates every non-wired module variable into
// the deduplicated set that will populate the root variables.tf. The
// result is sorted by RootName for deterministic rendering.
func collectRootVariables(plan *ComposePlan, w wiring) []rootVariable {
	seen := make(map[string]struct{})
	out := make([]rootVariable, 0)
	for _, m := range plan.Modules {
		entries := w[m.ResourceType]
		for _, v := range m.Variables {
			e, ok := entries[v.Name]
			if !ok || e.Ref != nil {
				continue
			}
			if _, dup := seen[e.RootName]; dup {
				continue
			}
			seen[e.RootName] = struct{}{}
			out = append(out, rootVariable{
				Name:         e.RootName,
				Type:         v.Type,
				Description:  v.Description,
				Default:      v.Default,
				Required:     v.Required,
				Sensitive:    v.Sensitive,
				SourceModule: m.ResourceType,
				SourceVar:    v.Name,
				Nested:       v.Nested,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// orderedModules returns plan.Modules indexed by name in plan.TopoOrder
// when the order is set, falling back to the plan's original order.
// Entries listed in TopoOrder but missing from plan.Modules are skipped.
func orderedModules(plan *ComposePlan) []GeneratedModule {
	byName := make(map[string]*GeneratedModule, len(plan.Modules))
	for i := range plan.Modules {
		byName[plan.Modules[i].ResourceType] = &plan.Modules[i]
	}
	if len(plan.TopoOrder) == 0 {
		out := make([]GeneratedModule, len(plan.Modules))
		copy(out, plan.Modules)
		return out
	}
	out := make([]GeneratedModule, 0, len(plan.Modules))
	seen := make(map[string]struct{}, len(plan.Modules))
	for _, name := range plan.TopoOrder {
		m, ok := byName[name]
		if !ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, *m)
	}
	for _, m := range plan.Modules {
		if _, ok := seen[m.ResourceType]; ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

// renderRootVersions emits the stack-level terraform block. The provider
// constraint mirrors the one used by each per-module version.tf so init
// resolves a single provider release for the whole stack.
func renderRootVersions(plan *ComposePlan) ([]byte, error) {
	if len(plan.Modules) == 0 {
		return []byte{}, nil
	}
	ref := plan.Modules[0]
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	tf := root.AppendNewBlock("terraform", nil).Body()
	tf.SetAttributeValue("required_version", cty.StringVal(">= 1.0.0"))
	rp := tf.AppendNewBlock("required_providers", nil).Body()
	attrs := []hclwrite.ObjectAttrTokens{
		{Name: hclwrite.TokensForIdentifier("source"), Value: hclwrite.TokensForValue(cty.StringVal(ref.ProviderSource))},
	}
	if ref.ProviderVersionConstraint != "" {
		attrs = append(attrs, hclwrite.ObjectAttrTokens{
			Name:  hclwrite.TokensForIdentifier("version"),
			Value: hclwrite.TokensForValue(cty.StringVal(ref.ProviderVersionConstraint)),
		})
	}
	rp.SetAttributeRaw(ref.ProviderLocalName, hclwrite.TokensForObject(attrs))
	return f.Bytes(), nil
}

// renderRootProviders emits an empty `provider "<local>" {}` block so
// consumers fill in region/profile/credentials without editing modules.
func renderRootProviders(plan *ComposePlan) ([]byte, error) {
	if len(plan.Modules) == 0 {
		return []byte{}, nil
	}
	local := plan.Modules[0].ProviderLocalName
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	root.AppendUnstructuredTokens(hclwrite.Tokens{{
		Type:  hclsyntax.TokenComment,
		Bytes: []byte("# Provider configuration. Fill in region, profile, credentials, default_tags, etc.\n"),
	}})
	root.AppendNewBlock("provider", []string{local})
	return f.Bytes(), nil
}

// renderRootVariables emits the aggregated user-facing input variables.
func renderRootVariables(vars []rootVariable) ([]byte, error) {
	if len(vars) == 0 {
		return []byte{}, nil
	}
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	for i, v := range vars {
		if i > 0 {
			root.AppendNewline()
		}
		block := root.AppendNewBlock("variable", []string{v.Name})
		body := block.Body()
		typeExpr := v.Type
		if v.Nested || typeExpr == "" {
			typeExpr = "any"
		}
		body.SetAttributeRaw("type", rawTypeTokens(typeExpr))
		desc := v.Description
		if v.SourceVar != "" && v.SourceModule != "" && v.Name != v.SourceVar {
			tag := fmt.Sprintf("[%s.%s] ", v.SourceModule, v.SourceVar)
			desc = tag + desc
		}
		if desc != "" {
			body.SetAttributeValue("description", cty.StringVal(desc))
		}
		if v.Sensitive {
			body.SetAttributeValue("sensitive", cty.BoolVal(true))
		}
		if !v.Required {
			if v.Default == nil {
				body.SetAttributeValue("default", cty.NullVal(cty.DynamicPseudoType))
			} else {
				val, err := anyToCty(v.Default)
				if err != nil {
					return nil, fmt.Errorf("variable %s default: %w", v.Name, err)
				}
				body.SetAttributeValue("default", val)
			}
		}
	}
	return f.Bytes(), nil
}

// renderRootLocals emits a skeleton locals.tf so users have a canonical
// place to define stack-level helper values without editing modules.
func renderRootLocals() []byte {
	var buf bytes.Buffer
	buf.WriteString("# Stack-level locals. Add helper values reused across module blocks here,\n")
	buf.WriteString("# for example:\n")
	buf.WriteString("#\n")
	buf.WriteString("# locals {\n")
	buf.WriteString("#   common_tags = {\n")
	buf.WriteString("#     project = var.project\n")
	buf.WriteString("#     owner   = var.owner\n")
	buf.WriteString("#   }\n")
	buf.WriteString("# }\n")
	return buf.Bytes()
}

// renderRootMain emits one `module` block per plan entry in topological
// order. Non-wired inputs read from `var.<RootName>`; wired inputs read
// from `module.<ref.Module>.<ref.Output>`.
func renderRootMain(plan *ComposePlan, order []GeneratedModule, w wiring, rootVars []rootVariable) ([]byte, error) {
	if len(order) == 0 {
		return []byte{}, nil
	}
	_ = rootVars // kept for future validation hooks
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	for i, m := range order {
		if i > 0 {
			root.AppendNewline()
		}
		block := root.AppendNewBlock("module", []string{m.ResourceType})
		body := block.Body()
		body.SetAttributeValue("source", cty.StringVal("./"+m.ResourceType))
		vars := append([]ModuleVariable(nil), m.Variables...)
		sort.SliceStable(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
		entries := w[m.ResourceType]
		for _, v := range vars {
			e, ok := entries[v.Name]
			if !ok {
				continue
			}
			if e.Ref != nil {
				body.SetAttributeRaw(v.Name, traversalTokens("module", e.Ref.Module, e.Ref.Output))
				continue
			}
			body.SetAttributeRaw(v.Name, traversalTokens("var", e.RootName))
		}
	}
	return f.Bytes(), nil
}

// renderRootOutputs re-exports every module output at stack level as
// `<module>_<output>` so downstream consumers (other stacks, tests) can
// read them without digging into module state.
func renderRootOutputs(plan *ComposePlan, order []GeneratedModule) ([]byte, error) {
	if len(order) == 0 {
		return []byte{}, nil
	}
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	first := true
	for _, m := range order {
		outs := append([]ModuleOutput(nil), m.Outputs...)
		sort.SliceStable(outs, func(i, j int) bool { return outs[i].Name < outs[j].Name })
		for _, o := range outs {
			if !first {
				root.AppendNewline()
			}
			first = false
			name := m.ResourceType + "_" + o.Name
			name = strings.ReplaceAll(name, ".", "_")
			block := root.AppendNewBlock("output", []string{name})
			body := block.Body()
			body.SetAttributeRaw("value", traversalTokens("module", m.ResourceType, o.Name))
			if o.Description != "" {
				body.SetAttributeValue("description", cty.StringVal(o.Description))
			}
			if o.Sensitive {
				body.SetAttributeValue("sensitive", cty.BoolVal(true))
			}
		}
	}
	_ = plan
	return f.Bytes(), nil
}
