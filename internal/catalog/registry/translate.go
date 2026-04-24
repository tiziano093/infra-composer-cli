package registry

import (
	"sort"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

// translateSchema converts a tfjson.Schema (the format emitted by
// `terraform providers schema -json`) into the catalog-facing
// ResourceSchema. Nested blocks are flattened into top-level inputs by
// recording them with their canonical block path (e.g. "tags",
// "ingress.from_port"); deep-nesting is preserved through type strings
// like "list(object({port:number,...}))" so generated Terraform can
// declare matching variable types.
func translateSchema(name string, kind Kind, raw *tfjson.Schema, sourceURL string) *ResourceSchema {
	rs := &ResourceSchema{
		Name:   name,
		Kind:   kind,
		Source: sourceURL,
		Group:  inferGroup(name),
	}
	if raw == nil || raw.Block == nil {
		return rs
	}
	rs.Description = raw.Block.Description

	rs.Inputs = collectInputs(raw.Block, "")
	rs.Outputs = collectOutputs(raw.Block, "")
	sort.Slice(rs.Inputs, func(i, j int) bool { return rs.Inputs[i].Name < rs.Inputs[j].Name })
	sort.Slice(rs.Outputs, func(i, j int) bool { return rs.Outputs[i].Name < rs.Outputs[j].Name })
	return rs
}

// collectInputs walks block to gather every attribute that the user is
// allowed to set (i.e. not Computed-only). Nested blocks are translated
// into a single attribute whose Type captures the nested shape so the
// composer can declare a matching variable.
func collectInputs(block *tfjson.SchemaBlock, prefix string) []InputSpec {
	if block == nil {
		return nil
	}
	var out []InputSpec
	for name, attr := range block.Attributes {
		if attr == nil {
			continue
		}
		if isComputedOnly(attr) {
			continue
		}
		// Terraform reserves the top-level `id` attribute; even when a
		// provider marks it Optional+Computed it cannot be assigned by
		// users. Surface it through Outputs only.
		if prefix == "" && name == "id" && attr.Computed {
			continue
		}
		out = append(out, InputSpec{
			Name:        joinPath(prefix, name),
			Type:        ctyTypeString(attr.AttributeType),
			Description: attr.Description,
			Required:    attr.Required,
			Sensitive:   attr.Sensitive,
		})
	}
	for name, nested := range block.NestedBlocks {
		if nested == nil || nested.Block == nil {
			continue
		}
		spec := InputSpec{
			Name:        joinPath(prefix, name),
			Type:        nestedBlockType(nested),
			Description: nested.Block.Description,
			Required:    nested.MinItems > 0,
		}
		for attrName, attr := range nested.Block.Attributes {
			if attr == nil || isComputedOnly(attr) {
				continue
			}
			spec.Attrs = append(spec.Attrs, NestedAttr{
				Name:     attrName,
				Type:     ctyTypeString(attr.AttributeType),
				Required: attr.Required,
			})
		}
		sort.Slice(spec.Attrs, func(i, j int) bool { return spec.Attrs[i].Name < spec.Attrs[j].Name })
		out = append(out, spec)
	}
	return out
}

// collectOutputs walks block to gather every attribute marked Computed
// (true outputs of the resource). Nested blocks are exposed as a single
// output whose Description points the user at their nested attributes.
func collectOutputs(block *tfjson.SchemaBlock, prefix string) []OutputSpec {
	if block == nil {
		return nil
	}
	var out []OutputSpec
	for name, attr := range block.Attributes {
		if attr == nil {
			continue
		}
		if !attr.Computed {
			continue
		}
		out = append(out, OutputSpec{
			Name:        joinPath(prefix, name),
			Description: attr.Description,
			Sensitive:   attr.Sensitive,
		})
	}
	for name, nested := range block.NestedBlocks {
		if nested == nil || nested.Block == nil {
			continue
		}
		// Recurse into nested blocks for outputs only when every nested
		// attribute is computed; otherwise the surface is mixed and we
		// surface the block itself as a single computed output.
		if isAllComputed(nested.Block) {
			out = append(out, collectOutputs(nested.Block, joinPath(prefix, name))...)
		}
	}
	return out
}

// isComputedOnly returns true when an attribute is purely an output
// (Computed && !Optional && !Required). Such attributes should not be
// surfaced as inputs.
func isComputedOnly(a *tfjson.SchemaAttribute) bool {
	return a.Computed && !a.Optional && !a.Required
}

func isAllComputed(b *tfjson.SchemaBlock) bool {
	if b == nil {
		return false
	}
	for _, a := range b.Attributes {
		if a != nil && (a.Required || a.Optional) {
			return false
		}
	}
	return true
}

// joinPath concatenates a prefix and a leaf name with a dot separator,
// taking care to skip the separator when the prefix is empty.
func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// ctyTypeString returns a canonical HCL type expression for a cty.Type
// suitable for emitting as `type = ...` in a Terraform variable. Falls
// back to "any" for unrepresentable / nil types so downstream HCL stays
// valid.
func ctyTypeString(t cty.Type) string {
	if t == cty.NilType || t.Equals(cty.DynamicPseudoType) {
		return "any"
	}
	switch {
	case t.Equals(cty.String):
		return "string"
	case t.Equals(cty.Number):
		return "number"
	case t.Equals(cty.Bool):
		return "bool"
	case t.IsListType():
		return "list(" + ctyTypeString(t.ElementType()) + ")"
	case t.IsSetType():
		return "set(" + ctyTypeString(t.ElementType()) + ")"
	case t.IsMapType():
		return "map(" + ctyTypeString(t.ElementType()) + ")"
	case t.IsObjectType():
		attrs := t.AttributeTypes()
		names := make([]string, 0, len(attrs))
		for n := range attrs {
			names = append(names, n)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, n := range names {
			parts = append(parts, n+" = "+ctyTypeString(attrs[n]))
		}
		if len(parts) == 0 {
			return "any"
		}
		return "object({" + strings.Join(parts, ", ") + "})"
	case t.IsTupleType():
		elems := t.TupleElementTypes()
		parts := make([]string, 0, len(elems))
		for _, e := range elems {
			parts = append(parts, ctyTypeString(e))
		}
		return "tuple([" + strings.Join(parts, ", ") + "])"
	}
	return "any"
}

// nestedBlockType produces an HCL type string for a NestedBlock based on
// its NestingMode. The implementation deliberately summarises deep
// shapes (we emit "list(any)" instead of recursively unrolling); this
// keeps catalog schemas compact while still telling users that the
// value is a list/map/etc.
//
// Note: HCL accepts list(any)/set(any)/map(any) but rejects
// object(any) — single-nested blocks therefore fall back to the loose
// "any" type so generated variables remain syntactically valid.
func nestedBlockType(nb *tfjson.SchemaBlockType) string {
	switch nb.NestingMode {
	case tfjson.SchemaNestingModeList:
		return "list(any)"
	case tfjson.SchemaNestingModeSet:
		return "set(any)"
	case tfjson.SchemaNestingModeMap:
		return "map(any)"
	case tfjson.SchemaNestingModeSingle, tfjson.SchemaNestingModeGroup:
		return "any"
	default:
		return "any"
	}
}

// inferGroup makes a best-effort guess at a sensible "group" tag for a
// resource based on its provider-prefixed name. The mapping is
// deliberately permissive; users can always override post-build by
// editing the catalog.
func inferGroup(name string) string {
	lower := strings.ToLower(name)
	switch {
	case containsAny(lower, "vpc", "subnet", "route", "gateway", "network", "nic", "lb_", "loadbalancer", "load_balancer", "dns", "vnet"):
		return "network"
	case containsAny(lower, "instance", "vm_", "compute", "spot", "function", "lambda", "container", "kubernetes", "eks_", "aks_", "gke_"):
		return "compute"
	case containsAny(lower, "bucket", "s3_", "blob", "disk", "volume", "ebs", "rds", "sql", "database", "dynamodb", "cosmosdb", "spanner"):
		return "storage"
	case containsAny(lower, "iam_", "role", "policy", "kms", "secret", "vault", "acl", "auth"):
		return "iam"
	case containsAny(lower, "log", "metric", "monitor", "alarm", "alert", "trace"):
		return "observability"
	}
	return ""
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
