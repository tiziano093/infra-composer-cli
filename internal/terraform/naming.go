package terraform

import "strings"

// InstanceName returns the canonical Terraform module call identifier
// used by the generator: `this_<modulename>` per ARCHITECTURE.md naming
// conventions. Provider prefixes (`aws_`, `azurerm_`, …) are NOT
// stripped because doing so would create collisions across providers.
func InstanceName(moduleName string) string {
	return "this_" + moduleName
}

// ExternalVarName builds the top-level variable identifier for an
// external input. Prefixing with the catalog module name guarantees
// uniqueness across modules that share an input name (e.g. two
// `cidr_block` inputs). Internal instance naming (`this_*`) is kept
// out of public-facing identifiers so we can rename instances later
// without breaking consumers.
func ExternalVarName(moduleName, varName string) string {
	return moduleName + "_" + varName
}

// StackOutputName builds the top-level output identifier exported by
// the composed stack.
func StackOutputName(moduleName, outputName string) string {
	return moduleName + "_" + outputName
}

// ProviderLocalName extracts the short provider name from a canonical
// "<namespace>/<name>" address. Returns the input unchanged when no
// slash is present so the generator never emits an empty key.
func ProviderLocalName(address string) string {
	if i := strings.LastIndex(address, "/"); i >= 0 && i < len(address)-1 {
		return address[i+1:]
	}
	return address
}
