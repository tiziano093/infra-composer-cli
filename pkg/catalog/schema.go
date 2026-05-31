// Package catalog defines the catalog schema used by infra-composer to
// describe the modules available for a given Terraform provider, plus
// helpers to parse, validate and (later) build/search such catalogs.
//
// The exported types mirror the JSON wire format of catalog files
// (schema.json) and are re-exported from pkg/catalog for downstream
// library consumers.
package catalog

import "time"

// SchemaVersion is the only catalog schema version currently supported
// by this CLI. Catalogs declaring a different version are rejected.
const SchemaVersion = "1.0"

// Schema is the top-level catalog document.
type Schema struct {
	// SchemaVersion is the catalog format version. Must equal SchemaVersion.
	SchemaVersion string `json:"schema_version"`

	// Provider is the Terraform provider this catalog covers, in the
	// canonical "<namespace>/<name>" form (e.g. "hashicorp/aws").
	Provider string `json:"provider"`

	// ProviderVersion is the semver of the provider release the catalog
	// was generated from (e.g. "5.42.0").
	ProviderVersion string `json:"provider_version"`

	// GeneratedAt is the UTC timestamp when the catalog was produced.
	// Optional: zero value means "unknown".
	GeneratedAt time.Time `json:"generated_at,omitempty"`

	// Modules is the unordered set of modules in the catalog.
	// Module names must be unique across the slice.
	Modules []ModuleEntry `json:"modules"`
}

// ModuleType discriminates the kind of Terraform construct an entry
// represents. Only "resource" and "data" are valid.
type ModuleType string

const (
	ModuleTypeResource ModuleType = "resource"
	ModuleTypeData     ModuleType = "data"
)

// IsValid reports whether t is one of the recognised ModuleType values.
func (t ModuleType) IsValid() bool {
	switch t {
	case ModuleTypeResource, ModuleTypeData:
		return true
	default:
		return false
	}
}

// ModuleEntry describes a single module exposed by a provider.
type ModuleEntry struct {
	// Name is the Terraform identifier (e.g. "aws_vpc"). Required, unique
	// per Schema.
	Name string `json:"name"`

	// Type is "resource" or "data". Required.
	Type ModuleType `json:"type"`

	// Group is an optional logical grouping used by search filters
	// (e.g. "network", "compute").
	Group string `json:"group,omitempty"`

	// Source is an optional remote location (typically a Git URL or
	// Terraform Registry address) where the module is hosted.
	Source string `json:"source,omitempty"`

	// Description is a human-readable summary.
	Description string `json:"description,omitempty"`

	// Variables are the input variables exposed by the module.
	// Variable names must be unique within a single ModuleEntry.
	Variables []Variable `json:"variables,omitempty"`

	// Outputs are the output values exposed by the module.
	// Output names must be unique within a single ModuleEntry.
	Outputs []Output `json:"outputs,omitempty"`
}

// VariableAttr describes one child attribute of a nested block variable.
// Populated during catalog build for blocks whose inner schema is known,
// and consumed by the generator to emit typed content {} bodies.
type VariableAttr struct {
	// Name is the Terraform attribute identifier inside the block.
	Name string `json:"name"`
	// Type is the HCL type literal (e.g. "string", "number").
	Type string `json:"type"`
	// Required mirrors the provider attribute Required flag.
	Required bool `json:"required"`
}

// Variable describes a Terraform input variable.
type Variable struct {
	// Name is the variable identifier. Required, unique per module.
	Name string `json:"name"`

	// Type is the HCL type literal (e.g. "string", "number", "bool",
	// "list(string)", "map(string)"). Required.
	Type string `json:"type"`

	// Description is a human-readable summary.
	Description string `json:"description,omitempty"`

	// Default is the default value, encoded as the corresponding JSON
	// type. Nil means "no default".
	Default any `json:"default,omitempty"`

	// Required signals that no default is provided and the consumer
	// must supply a value. When true, Default must be nil.
	Required bool `json:"required"`

	// Sensitive marks the variable as containing secret material.
	Sensitive bool `json:"sensitive,omitempty"`

	// References declares cross-module dependencies: the variable's value
	// is expected to be wired (typically via a `locals` block in the
	// composed stack) from the listed module outputs in the same catalog.
	// Empty means "no inter-module dependency".
	References []VariableReference `json:"references,omitempty"`
	// Attrs holds the child attributes of a nested block variable.
	// Nil for flat scalar variables. omitempty ensures old catalogs that
	// lack this field remain valid on round-trip.
	Attrs []VariableAttr `json:"attrs,omitempty"`
}

// VariableReference points to a single output of another module in the
// same catalog. It is the schema-level mechanism through which
// infra-composer infers the dependency graph used by `dependencies` and
// `compose`.
type VariableReference struct {
	// Module is the target module name (must exist in the same Schema
	// and must differ from the referencing module).
	Module string `json:"module"`

	// Output is the name of the output on the target module to wire in.
	Output string `json:"output"`
}

// Output describes a Terraform output value.
type Output struct {
	// Name is the output identifier. Required, unique per module.
	Name string `json:"name"`

	// Description is a human-readable summary.
	Description string `json:"description,omitempty"`

	// Sensitive marks the output as containing secret material.
	Sensitive bool `json:"sensitive,omitempty"`
}
