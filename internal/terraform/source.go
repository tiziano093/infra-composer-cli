// Package terraform composes a Terraform stack from a catalog Schema.
//
// The package is split in three concerns:
//
//   - Plan: select catalog modules, resolve their sources, compute the
//     wiring graph (which inputs flow from another module's output and
//     which must be supplied externally), and produce a deterministic
//     ComposePlan that downstream code can reason about without touching
//     the catalog package.
//
//   - Generate: turn a ComposePlan into a list of GeneratedFile values
//     by rendering the five core .tf files via hashicorp/hcl/v2/hclwrite
//     so HCL literals are always syntactically valid regardless of the
//     shape of catalog defaults.
//
//   - Source: a small SourceResolver interface so Block D can plug in
//     git-based detection without changing anything in this package.
//
// The package is intentionally side-effect free: writing GeneratedFile
// values to disk is the responsibility of the calling command (today
// internal/commands/compose.go).
package terraform

import (
	"github.com/tiziano093/infra-composer-cli/internal/catalog"
)

// SourceKind classifies how a module's source URL must be rendered in
// the generated `module` block. Different kinds need different pinning
// idioms (Terraform module `version` only applies to the registry).
type SourceKind string

const (
	// SourceRegistry is a Terraform Registry module address; supports
	// the `version` attribute on the module block.
	SourceRegistry SourceKind = "registry"
	// SourceGit is a remote git source. Pinning is encoded inside the
	// `source` URL via `?ref=<tag>`; no `version` attribute is rendered.
	SourceGit SourceKind = "git"
	// SourceLocal is a relative path to an in-repo module.
	SourceLocal SourceKind = "local"
	// SourcePlaceholder marks the source as unresolved. Generation
	// still proceeds but emits a clearly-flagged TODO so users know to
	// fix it before terraform init.
	SourcePlaceholder SourceKind = "placeholder"
)

// SourceSpec describes the fully-resolved Terraform source for one
// composed module instance. Address is what gets rendered into the
// module block's `source` attribute; Ref carries the version/tag for
// kinds that need it (registry → SemVer, git → tag/branch/SHA).
type SourceSpec struct {
	Kind    SourceKind
	Address string
	Ref     string
}

// SourceResolver decides where a module instance's code lives.
// Implementations may consult the catalog ModuleEntry, the surrounding
// git repository, or a static map.
type SourceResolver interface {
	Resolve(m *catalog.ModuleEntry) (SourceSpec, error)
}

// PlaceholderResolver is the trivial resolver used by Block C while git
// integration is not yet wired up: it surfaces ModuleEntry.Source
// verbatim when present (treated as a git URL) and falls back to a
// clearly-marked placeholder otherwise.
type PlaceholderResolver struct{}

// Resolve implements SourceResolver.
func (PlaceholderResolver) Resolve(m *catalog.ModuleEntry) (SourceSpec, error) {
	if m == nil {
		return SourceSpec{Kind: SourcePlaceholder, Address: "TODO: set module source"}, nil
	}
	if m.Source != "" {
		return SourceSpec{Kind: SourceGit, Address: m.Source}, nil
	}
	return SourceSpec{Kind: SourcePlaceholder, Address: "TODO: set module source"}, nil
}
