package terraform

import "strings"

// LocalBlockName is the identifier used as the second label of the
// single resource/data block in every generated module
// (e.g. `resource "azurerm_resource_group" "this" {}`). Hard-coded to
// keep the v1 output predictable; multi-instance support lives behind
// future for_each work.
const LocalBlockName = "this"

// ProviderLocalName extracts the short provider name from a canonical
// "<namespace>/<name>" address. Returns the input unchanged when no
// slash is present so the generator never emits an empty key.
func ProviderLocalName(address string) string {
	if i := strings.LastIndex(address, "/"); i >= 0 && i < len(address)-1 {
		return address[i+1:]
	}
	return address
}

// VersionConstraint converts a literal provider version into a
// pessimistic `~> major.minor` pin. Falls back to the raw input when
// parsing fails so unconventional tags still render.
func VersionConstraint(version string) string {
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

// IsNestedAttrName reports whether a catalog variable name targets a
// nested block sub-attribute (i.e. carries dotted segments produced by
// the registry translator). Cut B renders the parent block as a single
// `any`-typed variable and skips dotted children entirely.
func IsNestedAttrName(name string) bool {
	return strings.Contains(name, ".")
}

// IsNestedBlockType reports whether a catalog variable type string is
// the synthetic shape emitted by the registry translator for nested
// blocks (any/list(any)/set(any)/map(any)). These cannot be modelled
// structurally in v1 and are flagged for TODO rendering.
func IsNestedBlockType(typeExpr string) bool {
	switch typeExpr {
	case "any", "list(any)", "set(any)", "map(any)":
		return true
	}
	return false
}
