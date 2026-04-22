package terraform

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Generate renders the five core .tf files for the given plan. Files
// are returned in a stable order (providers, variables, locals, main,
// outputs) so callers can hash the result deterministically.
func Generate(plan *ComposePlan) ([]GeneratedFile, error) {
	if plan == nil {
		return nil, fmt.Errorf("terraform: nil plan")
	}
	files := make([]GeneratedFile, 0, 5)
	for _, step := range []struct {
		name string
		fn   func(*ComposePlan) ([]byte, error)
	}{
		{"providers.tf", renderProviders},
		{"variables.tf", renderVariables},
		{"locals.tf", renderLocals},
		{"main.tf", renderMain},
		{"outputs.tf", renderOutputs},
	} {
		body, err := step.fn(plan)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", step.name, err)
		}
		files = append(files, GeneratedFile{Path: step.name, Content: body})
	}
	return files, nil
}

// renderProviders emits a `terraform { required_providers { ... } }`
// block plus a single empty `provider "<name>" {}` placeholder. Provider
// authentication is intentionally left to the user (env vars, profiles,
// etc.) — the comment in the block calls that out.
func renderProviders(plan *ComposePlan) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()

	tfBlock := root.AppendNewBlock("terraform", nil)
	tfBody := tfBlock.Body()
	rp := tfBody.AppendNewBlock("required_providers", nil)
	attrs := []hclwrite.ObjectAttrTokens{{
		Name: hclwrite.TokensForIdentifier(plan.ProviderName),
		Value: hclwrite.TokensForObject([]hclwrite.ObjectAttrTokens{
			{Name: hclwrite.TokensForIdentifier("source"), Value: hclwrite.TokensForValue(cty.StringVal(plan.Provider))},
			{Name: hclwrite.TokensForIdentifier("version"), Value: hclwrite.TokensForValue(cty.StringVal(versionConstraint(plan.ProviderVersion)))},
		}),
	}}
	rp.Body().SetAttributeRaw(plan.ProviderName, hclwrite.TokensForObject(attrs))

	root.AppendNewline()
	provBlock := root.AppendNewBlock("provider", []string{plan.ProviderName})
	provBlock.Body().AppendUnstructuredTokens(hclwrite.Tokens{
		{Type: hclsyntax.TokenComment, Bytes: []byte("  # TODO: configure provider authentication and region as needed.\n")},
	})

	return f.Bytes(), nil
}

// renderVariables emits one variable block per ExternalInput across
// every module in the plan. ExternalInputs already carry the prefixed
// name to avoid collisions.
func renderVariables(plan *ComposePlan) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	first := true
	for _, m := range plan.Modules {
		for _, in := range m.ExternalInputs {
			if !first {
				root.AppendNewline()
			}
			first = false
			block := root.AppendNewBlock("variable", []string{in.VarName})
			body := block.Body()
			if in.Description != "" {
				body.SetAttributeValue("description", cty.StringVal(in.Description))
			}
			if t := in.Type; t != "" {
				body.SetAttributeRaw("type", rawTypeTokens(t))
			}
			if in.Sensitive {
				body.SetAttributeValue("sensitive", cty.BoolVal(true))
			}
			if !in.Required {
				if in.Default == nil {
					body.SetAttributeValue("default", cty.NullVal(cty.DynamicPseudoType))
				} else {
					val, err := anyToCty(in.Default)
					if err != nil {
						return nil, fmt.Errorf("variable %s default: %w", in.VarName, err)
					}
					body.SetAttributeValue("default", val)
				}
			}
		}
	}
	return f.Bytes(), nil
}

// renderLocals emits a small placeholder locals block. Cross-module
// wiring lives directly in the module call attributes; locals.tf is
// reserved for stack-level values the user typically tweaks.
func renderLocals(plan *ComposePlan) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	body := f.Body()
	block := body.AppendNewBlock("locals", nil)
	block.Body().SetAttributeValue("stack_name", cty.StringVal("TODO"))
	return f.Bytes(), nil
}

// renderMain emits one module block per ResolvedModule. Wired inputs
// reference `module.<other>.<output>`; external inputs reference
// `var.<varname>`. Module `version` is only emitted for SourceRegistry.
func renderMain(plan *ComposePlan) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	for i, m := range plan.Modules {
		if i > 0 {
			root.AppendNewline()
		}
		block := root.AppendNewBlock("module", []string{m.Instance})
		body := block.Body()
		if m.Source.Kind == SourcePlaceholder {
			body.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenComment, Bytes: []byte("  # TODO: replace placeholder source with the real module URL.\n")},
			})
		}
		body.SetAttributeValue("source", cty.StringVal(sourceWithRef(m.Source)))
		if m.Source.Kind == SourceRegistry && m.Source.Ref != "" {
			body.SetAttributeValue("version", cty.StringVal(m.Source.Ref))
		}
		// Render inputs in catalog declaration order (already preserved
		// by ResolvedModule construction).
		for _, w := range m.WiredInputs {
			body.SetAttributeRaw(w.VarName, traversalTokens("module", w.FromInstance, w.FromOutput))
		}
		for _, e := range m.ExternalInputs {
			body.SetAttributeRaw(e.LocalName, traversalTokens("var", e.VarName))
		}
	}
	return f.Bytes(), nil
}

// renderOutputs emits one stack-level output per source-module output.
func renderOutputs(plan *ComposePlan) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	first := true
	for _, m := range plan.Modules {
		for _, o := range m.StackOutputs {
			if !first {
				root.AppendNewline()
			}
			first = false
			block := root.AppendNewBlock("output", []string{o.ExportedName})
			body := block.Body()
			if o.Description != "" {
				body.SetAttributeValue("description", cty.StringVal(o.Description))
			}
			body.SetAttributeRaw("value", traversalTokens("module", o.Instance, o.Output))
			if o.Sensitive {
				body.SetAttributeValue("sensitive", cty.BoolVal(true))
			}
		}
	}
	return f.Bytes(), nil
}

// rawTypeTokens emits a Terraform variable type expression
// (`string`, `list(string)`, `object({...})`, …) as a raw token so
// hclwrite does not quote it.
func rawTypeTokens(typeExpr string) hclwrite.Tokens {
	return hclwrite.Tokens{{
		Type:         hclsyntax.TokenIdent,
		Bytes:        []byte(typeExpr),
		SpacesBefore: 1,
	}}
}

// traversalTokens builds an attribute traversal like `module.foo.bar`
// from individual identifier segments.
func traversalTokens(parts ...string) hclwrite.Tokens {
	traversal := make(hcl.Traversal, 0, len(parts))
	for i, p := range parts {
		if i == 0 {
			traversal = append(traversal, hcl.TraverseRoot{Name: p})
			continue
		}
		traversal = append(traversal, hcl.TraverseAttr{Name: p})
	}
	return hclwrite.TokensForTraversal(traversal)
}

// versionConstraint converts a literal provider version into a `~>` pin
// at the major.minor level. Falls back to the raw input when parsing
// fails, so weird tags still render even if unconventional.
func versionConstraint(version string) string {
	if version == "" {
		return ""
	}
	v := version
	if v[0] == 'v' {
		v = v[1:]
	}
	dot1 := -1
	dot2 := -1
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			if dot1 < 0 {
				dot1 = i
			} else if dot2 < 0 {
				dot2 = i
				break
			}
		}
	}
	if dot1 < 0 || dot2 < 0 {
		return v
	}
	return "~> " + v[:dot2]
}

// sourceWithRef returns the rendered `source = "…"` string. Git refs
// are appended via `?ref=`; registry refs are emitted in a separate
// `version` attribute by the caller. Placeholder sources are returned
// verbatim so the user spots them immediately.
func sourceWithRef(s SourceSpec) string {
	switch s.Kind {
	case SourceGit:
		if s.Ref == "" {
			return s.Address
		}
		if containsRune(s.Address, '?') {
			return s.Address + "&ref=" + s.Ref
		}
		return s.Address + "?ref=" + s.Ref
	default:
		return s.Address
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// anyToCty converts a JSON-decoded Go value (the shape json.Unmarshal
// into `any` produces) to a cty.Value suitable for hclwrite.
// Supported: nil, bool, float64, string, []any, map[string]any.
// Unknown types yield an error so we never silently drop data.
func anyToCty(v any) (cty.Value, error) {
	switch x := v.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType), nil
	case bool:
		return cty.BoolVal(x), nil
	case string:
		return cty.StringVal(x), nil
	case float64:
		return cty.NumberFloatVal(x), nil
	case int:
		return cty.NumberIntVal(int64(x)), nil
	case int64:
		return cty.NumberIntVal(x), nil
	case []any:
		if len(x) == 0 {
			return cty.EmptyTupleVal, nil
		}
		out := make([]cty.Value, len(x))
		for i, el := range x {
			cv, err := anyToCty(el)
			if err != nil {
				return cty.NilVal, err
			}
			out[i] = cv
		}
		return cty.TupleVal(out), nil
	case map[string]any:
		if len(x) == 0 {
			return cty.EmptyObjectVal, nil
		}
		out := make(map[string]cty.Value, len(x))
		for k, el := range x {
			cv, err := anyToCty(el)
			if err != nil {
				return cty.NilVal, err
			}
			out[k] = cv
		}
		return cty.ObjectVal(out), nil
	default:
		return cty.NilVal, fmt.Errorf("unsupported default type %T", v)
	}
}
