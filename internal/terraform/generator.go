package terraform

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// Generate renders the per-module .tf files for every GeneratedModule
// in plan. The returned slice contains files in deterministic order:
// modules in the plan order, files within a module in a fixed order
// (version, variables, main, outputs, README). Paths include the
// module folder so callers can write them straight to --output-dir.
func Generate(plan *ComposePlan) ([]GeneratedFile, error) {
	if plan == nil {
		return nil, fmt.Errorf("terraform: nil plan")
	}
	out := make([]GeneratedFile, 0, len(plan.Modules)*5)
	for i := range plan.Modules {
		m := &plan.Modules[i]
		files, err := renderModule(m)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", m.ResourceType, err)
		}
		out = append(out, files...)
	}
	return out, nil
}

// renderModule emits the five files for a single GeneratedModule.
// Paths are scoped under the module folder (`<resource_type>/...`).
func renderModule(m *GeneratedModule) ([]GeneratedFile, error) {
	steps := []struct {
		name string
		fn   func(*GeneratedModule) ([]byte, error)
	}{
		{"version.tf", renderVersion},
		{"variables.tf", renderVariables},
		{"main.tf", renderMain},
		{"outputs.tf", renderOutputs},
		{"README.md", renderReadme},
	}
	out := make([]GeneratedFile, 0, len(steps))
	for _, s := range steps {
		body, err := s.fn(m)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", s.name, err)
		}
		out = append(out, GeneratedFile{
			Module:  m.ResourceType,
			Path:    path.Join(m.ResourceType, s.name),
			Content: body,
		})
	}
	return out, nil
}

// renderVersion emits the terraform { required_version; required_providers }
// block with the real provider source. No `provider "<name>" {}` block
// is emitted: provider configuration is the caller's responsibility,
// not the module's, matching the reference style.
func renderVersion(m *GeneratedModule) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	tf := root.AppendNewBlock("terraform", nil).Body()
	tf.SetAttributeValue("required_version", cty.StringVal(">= 1.0.0"))
	rp := tf.AppendNewBlock("required_providers", nil).Body()

	attrs := []hclwrite.ObjectAttrTokens{
		{Name: hclwrite.TokensForIdentifier("source"), Value: hclwrite.TokensForValue(cty.StringVal(m.ProviderSource))},
	}
	if m.ProviderVersionConstraint != "" {
		attrs = append(attrs, hclwrite.ObjectAttrTokens{
			Name: hclwrite.TokensForIdentifier("version"), Value: hclwrite.TokensForValue(cty.StringVal(m.ProviderVersionConstraint)),
		})
	}
	rp.SetAttributeRaw(m.ProviderLocalName, hclwrite.TokensForObject(attrs))
	return f.Bytes(), nil
}

// renderVariables emits one `variable "<name>" {}` block per
// ModuleVariable, in original (catalog) order.
func renderVariables(m *GeneratedModule) ([]byte, error) {
	if len(m.Variables) == 0 {
		return []byte{}, nil
	}
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	for i, v := range m.Variables {
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
		if v.Description != "" {
			body.SetAttributeValue("description", cty.StringVal(v.Description))
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

// renderMain emits the single `resource|data "<type>" "this" {}` block
// at the heart of the generated module. Each non-nested variable maps
// to a `<name> = var.<name>` assignment. Nested-block variables with
// known child attrs emit a dynamic (or static) block; those without
// (old catalog format) fall back to a TODO comment.
func renderMain(m *GeneratedModule) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	blockType := "resource"
	if m.Kind == catalog.ModuleTypeData {
		blockType = "data"
	}
	block := root.AppendNewBlock(blockType, []string{m.ResourceType, m.LocalName})
	body := block.Body()

	scalars := make([]ModuleVariable, 0, len(m.Variables))
	nested := make([]ModuleVariable, 0)
	for _, v := range m.Variables {
		if v.Nested {
			nested = append(nested, v)
			continue
		}
		scalars = append(scalars, v)
	}
	for _, v := range scalars {
		body.SetAttributeRaw(v.Name, traversalTokens("var", v.Name))
	}
	for _, v := range nested {
		if len(v.Attrs) > 0 {
			emitNestedBlock(body, v)
		} else {
			body.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenComment, Bytes: []byte(fmt.Sprintf("# TODO: configure nested block %q manually using var.%s.\n", v.Name, v.Name))},
			})
		}
	}
	return hclwrite.Format(f.Bytes()), nil
}

// inferForEach returns the HCL for_each expression for a dynamic nested
// block based on the variable's type string. Returns "" for required
// single blocks that should be emitted as static (non-dynamic) blocks.
func inferForEach(mv ModuleVariable) string {
	switch mv.Type {
	case "list(any)", "set(any)":
		return fmt.Sprintf("var.%s != null ? var.%s : []", mv.Name, mv.Name)
	case "map(any)":
		return fmt.Sprintf("var.%s != null ? var.%s : {}", mv.Name, mv.Name)
	default: // "any" = single/group nesting
		if mv.Required {
			return "" // static block
		}
		return fmt.Sprintf("var.%s != null ? [var.%s] : []", mv.Name, mv.Name)
	}
}

// emitNestedBlock appends a dynamic or static nested block to body.
// Dynamic blocks use `block.value.attr` references; static (required
// single) blocks use `var.block.attr` references. hclwrite.Format
// is applied to the renderMain output so indentation is normalised.
func emitNestedBlock(body *hclwrite.Body, mv ModuleVariable) {
	forEach := inferForEach(mv)
	var sb strings.Builder
	if forEach == "" {
		// Required single-instance block: static, no dynamic wrapper.
		fmt.Fprintf(&sb, "%s {\n", mv.Name)
		for _, a := range mv.Attrs {
			fmt.Fprintf(&sb, "  %s = var.%s.%s\n", a.Name, mv.Name, a.Name)
		}
		sb.WriteString("}\n")
	} else {
		fmt.Fprintf(&sb, "dynamic %q {\n  for_each = %s\n  content {\n", mv.Name, forEach)
		for _, a := range mv.Attrs {
			fmt.Fprintf(&sb, "    %s = %s.value.%s\n", a.Name, mv.Name, a.Name)
		}
		sb.WriteString("  }\n}\n")
	}
	body.AppendUnstructuredTokens(hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(sb.String())},
	})
}

// renderOutputs emits one `output "<name>" {}` block per ModuleOutput,
// referring to the single `<kind>.<type>.this.<name>` traversal.
func renderOutputs(m *GeneratedModule) ([]byte, error) {
	if len(m.Outputs) == 0 {
		return []byte{}, nil
	}
	f := hclwrite.NewEmptyFile()
	root := f.Body()
	for i, o := range m.Outputs {
		if i > 0 {
			root.AppendNewline()
		}
		block := root.AppendNewBlock("output", []string{o.Name})
		body := block.Body()
		body.SetAttributeRaw("value", outputTraversal(m, o.Name))
		if o.Description != "" {
			body.SetAttributeValue("description", cty.StringVal(o.Description))
		}
		if o.Sensitive {
			body.SetAttributeValue("sensitive", cty.BoolVal(true))
		}
	}
	return f.Bytes(), nil
}

// outputTraversal builds the `<kind>.<type>.this.<name>` reference
// honouring resource vs data block syntax (`data.` prefix for data
// sources, bare type for resources).
func outputTraversal(m *GeneratedModule, attr string) hclwrite.Tokens {
	if m.Kind == catalog.ModuleTypeData {
		return traversalTokens("data", m.ResourceType, m.LocalName, attr)
	}
	return traversalTokens(m.ResourceType, m.LocalName, attr)
}

// renderReadme emits a markdown README documenting the module's
// provider, inputs, outputs, and v1 limitations. Style mimics the
// terraform-docs default tables so the output is familiar.
func renderReadme(m *GeneratedModule) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# %s\n\n", m.ResourceType)
	if m.Description != "" {
		fmt.Fprintf(&buf, "%s\n\n", m.Description)
	}
	kindLabel := "resource"
	if m.Kind == catalog.ModuleTypeData {
		kindLabel = "data source"
	}
	fmt.Fprintf(&buf, "Generated %s module wrapping `%s`.\n\n", kindLabel, m.ResourceType)

	buf.WriteString("## Requirements\n\n")
	buf.WriteString("| Name | Source | Version |\n|------|--------|---------|\n")
	fmt.Fprintf(&buf, "| %s | `%s` | `%s` |\n\n", m.ProviderLocalName, m.ProviderSource, fallback(m.ProviderVersionConstraint, "n/a"))

	if len(m.Variables) > 0 {
		buf.WriteString("## Inputs\n\n")
		buf.WriteString("| Name | Type | Required | Default | Description |\n")
		buf.WriteString("|------|------|----------|---------|-------------|\n")
		vars := append([]ModuleVariable(nil), m.Variables...)
		sort.SliceStable(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
		for _, v := range vars {
			required := "no"
			if v.Required {
				required = "**yes**"
			}
			defaultStr := "n/a"
			if !v.Required {
				defaultStr = formatDefault(v.Default)
			}
			typeStr := v.Type
			if v.Nested || typeStr == "" {
				typeStr = "any"
			}
			fmt.Fprintf(&buf, "| `%s` | `%s` | %s | %s | %s |\n",
				v.Name, typeStr, required, defaultStr, escapeMarkdownCell(v.Description))
		}
		buf.WriteString("\n")
	}

	if len(m.Outputs) > 0 {
		buf.WriteString("## Outputs\n\n")
		buf.WriteString("| Name | Description |\n|------|-------------|\n")
		outs := append([]ModuleOutput(nil), m.Outputs...)
		sort.SliceStable(outs, func(i, j int) bool { return outs[i].Name < outs[j].Name })
		for _, o := range outs {
			fmt.Fprintf(&buf, "| `%s` | %s |\n", o.Name, escapeMarkdownCell(o.Description))
		}
		buf.WriteString("\n")
	}

	buf.WriteString("## v1 limitations\n\n")
	buf.WriteString("- Single static block (no `for_each`/`count`).\n")
	buf.WriteString("- Nested blocks from catalogs built before v2.1 are surfaced as `any`-typed variables with a TODO comment; rebuild the catalog to get typed `dynamic` blocks.\n")

	if len(m.Warnings) > 0 {
		buf.WriteString("\n## Generation warnings\n\n")
		for _, w := range m.Warnings {
			fmt.Fprintf(&buf, "- %s\n", w)
		}
	}
	return buf.Bytes(), nil
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func escapeMarkdownCell(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func formatDefault(v any) string {
	if v == nil {
		return "`null`"
	}
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("`%q`", x)
	case bool, float64, int, int64:
		return fmt.Sprintf("`%v`", x)
	default:
		return "see schema"
	}
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

// traversalTokens builds an attribute traversal like `var.foo` from
// individual identifier segments.
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

// anyToCty converts a JSON-decoded Go value (the shape json.Unmarshal
// into `any` produces) to a cty.Value suitable for hclwrite.
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
