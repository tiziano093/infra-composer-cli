// Package registry abstracts the source of truth for provider metadata
// consumed by the catalog builder pipeline. Phase 2 ships a single
// in-process implementation (FakeClient) that reads JSON fixtures from
// disk; a real Terraform Registry HTTP client is planned for a later
// phase behind the same Client interface.
package registry

import (
	"context"
	"errors"
)

// Kind is the Terraform construct kind exposed by a provider entry.
// Mirrors catalog.ModuleType but kept as a plain string here to avoid
// pulling the catalog schema types into the registry layer.
type Kind string

const (
	KindResource Kind = "resource"
	KindData     Kind = "data"
)

// IsValid reports whether k is one of the recognised Kind values.
func (k Kind) IsValid() bool {
	switch k {
	case KindResource, KindData:
		return true
	default:
		return false
	}
}

// ProviderInfo identifies a Terraform provider release.
type ProviderInfo struct {
	Namespace string
	Name      string
	Version   string
}

// Address returns the canonical "<namespace>/<name>" provider address.
func (p ProviderInfo) Address() string {
	return p.Namespace + "/" + p.Name
}

// ResourceSummary is the lightweight listing entry returned by
// ListResources. It carries just enough information to fetch the full
// schema in a follow-up call.
type ResourceSummary struct {
	Name string
	Kind Kind
}

// ResourceSchema is the full provider-side description of a single
// resource or data source. Field shapes are deliberately neutral so the
// builder can map them onto catalog.ModuleEntry without leaking HTTP
// details.
type ResourceSchema struct {
	Name        string
	Kind        Kind
	Group       string
	Source      string
	Description string
	Inputs      []InputSpec
	Outputs     []OutputSpec
}

// InputSpec mirrors a Terraform provider input attribute.
type InputSpec struct {
	Name        string
	Type        string
	Description string
	Default     any
	Required    bool
	Sensitive   bool
}

// OutputSpec mirrors a Terraform provider output attribute.
type OutputSpec struct {
	Name        string
	Description string
	Sensitive   bool
}

// Client is the registry-side surface area used by the catalog builder.
// Implementations may be backed by HTTP, a static catalog dump, or test
// fixtures; they MUST be safe for sequential use within a single Build()
// invocation but are not required to be safe for concurrent calls.
type Client interface {
	// DiscoverProvider resolves a provider address ("<namespace>/<name>")
	// to its canonical metadata, including the version that should be
	// pinned in the resulting catalog. The address is the user-facing
	// identifier; implementations choose how to resolve "latest".
	DiscoverProvider(ctx context.Context, address string) (*ProviderInfo, error)

	// ListResources enumerates every resource and data source exposed by
	// the given provider release. Order is implementation-defined; the
	// builder sorts results before normalisation so the output is stable.
	ListResources(ctx context.Context, p ProviderInfo) ([]ResourceSummary, error)

	// GetResourceSchema returns the full schema for the named resource.
	// Implementations MUST return ErrResourceNotFound when the lookup
	// fails so the builder can surface a precise error.
	GetResourceSchema(ctx context.Context, p ProviderInfo, name string, kind Kind) (*ResourceSchema, error)
}

// Sentinel errors returned by Client implementations. Wrap with %w so
// callers can inspect them via errors.Is.
var (
	ErrProviderNotFound = errors.New("registry: provider not found")
	ErrResourceNotFound = errors.New("registry: resource not found")
)
