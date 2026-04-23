// Package terraform turns a catalog Schema into a set of standalone,
// reusable Terraform module folders — one per selected resource or data
// source. Each generated folder contains a single resource (or data)
// block, an ergonomic variables.tf, an outputs.tf exposing every
// computed attribute, a version.tf pinning the real provider, and a
// README.md auto-documenting inputs and outputs.
//
// The package is intentionally side-effect free: writing GeneratedFile
// values to disk is the responsibility of the calling command.
package terraform

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// ModuleVariable is one input variable of a generated module folder.
// Each instance maps 1:1 to a variable block in the produced
// variables.tf and to an attribute assignment inside the resource block
// in main.tf (`<Name> = var.<Name>`).
type ModuleVariable struct {
	// Name is the Terraform identifier; matches the provider attribute.
	Name string
	// Type is an HCL type literal (e.g. "string", "list(string)",
	// "map(any)"). Empty falls back to "any" at render time.
	Type string
	// Description carries the provider attribute description verbatim.
	Description string
	// Default is the default value, if any. Required variables must
	// have Default == nil.
	Default any
	// Required marks the variable as having no default; matching
	// provider Required semantics.
	Required bool
	// Sensitive marks the variable as containing secret material.
	Sensitive bool
	// Nested signals a nested-block attribute that the generator
	// cannot fully model in v1 (Cut B). The variable is still rendered
	// (with type "any" and default null) and the resource block emits
	// a TODO so the user can complete the nested block by hand.
	Nested bool
	// References carries the catalog-declared cross-module wiring for
	// this variable. When root-stack rendering is enabled, each
	// reference whose target module is also part of the plan is turned
	// into a `var = module.<ref.Module>.<ref.Output>` assignment in the
	// root main.tf.
	References []catalog.VariableReference
}

// ModuleOutput is one output exposed by the generated module. Each
// instance maps to one `output { value = <kind>.<type>.this.<name> }`
// block in outputs.tf.
type ModuleOutput struct {
	Name        string
	Description string
	Sensitive   bool
}

// GeneratedModule is a fully-resolved, self-describing blueprint for
// a single reusable Terraform module folder. The generator never
// reaches back into the catalog when rendering one.
type GeneratedModule struct {
	// ResourceType is the Terraform resource/data type and also the
	// folder name (e.g. "azurerm_resource_group").
	ResourceType string
	// Kind discriminates resource vs data source.
	Kind catalog.ModuleType
	// LocalName is the identifier of the single block inside main.tf
	// (e.g. "this" → `resource "azurerm_resource_group" "this" {}`).
	LocalName string
	// Description is the provider-supplied summary surfaced in README.
	Description string
	// Group is an optional logical tag carried over from the catalog.
	Group string

	// ProviderLocalName is the short provider key used in
	// required_providers (e.g. "azurerm").
	ProviderLocalName string
	// ProviderSource is the canonical "<namespace>/<name>" address
	// rendered in required_providers.source.
	ProviderSource string
	// ProviderVersionConstraint is the rendered constraint string
	// (e.g. "~> 4.69") rendered in required_providers.version.
	ProviderVersionConstraint string

	Variables []ModuleVariable
	Outputs   []ModuleOutput

	// Warnings is a per-module diagnostic list (e.g. which variables
	// were collapsed by Cut B). Surfaced verbatim by the compose
	// command and recorded in the generated README.
	Warnings []string
}

// ComposePlan groups every generated module produced by Plan() so the
// caller can render and write them in one pass. Each module is
// independent: the generator emits one folder per element with no
// cross-module references.
type ComposePlan struct {
	// Provider is the canonical "<namespace>/<name>" carried over
	// from the source catalog. Identical for every module in Modules.
	Provider string
	// ProviderVersion is the literal provider version the catalog
	// was generated from (used to derive the per-module constraint).
	ProviderVersion string
	// Modules lists the generated modules in the order requested by
	// the user (with duplicates removed).
	Modules []GeneratedModule
	// Warnings collects plan-level diagnostics that are not specific
	// to a single module.
	Warnings []string
}

// GeneratedFile is one rendered artefact returned by Generate. Path is
// always relative to the compose output directory and includes the
// module folder, e.g. "azurerm_resource_group/main.tf".
type GeneratedFile struct {
	// Module is the ResourceType the file belongs to. Empty for files
	// shared across modules (none are emitted today; reserved).
	Module  string
	Path    string
	Content []byte
}

// SHA256Hex returns the hex-encoded SHA-256 of the file content. Used
// by dry-run output so consumers can diff plans without dumping
// bodies.
func (f GeneratedFile) SHA256Hex() string {
	sum := sha256.Sum256(f.Content)
	return hex.EncodeToString(sum[:])
}
