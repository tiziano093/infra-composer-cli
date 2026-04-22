// Package catalog re-exports the public catalog data types so external
// consumers of infra-composer can depend on a stable import path
// without reaching into internal/.
//
// The implementation lives in internal/catalog; this package only
// provides type aliases. Adding behaviour here is intentionally not
// supported.
package catalog

import internalcatalog "github.com/tiziano093/infra-composer-cli/internal/catalog"

// SchemaVersion is the catalog format version supported by this build.
const SchemaVersion = internalcatalog.SchemaVersion

// Re-exported data types (see internal/catalog for documentation).
type (
	Schema        = internalcatalog.Schema
	ModuleEntry   = internalcatalog.ModuleEntry
	ModuleType    = internalcatalog.ModuleType
	Variable      = internalcatalog.Variable
	Output        = internalcatalog.Output
	SearchOptions = internalcatalog.SearchOptions
	SearchResult  = internalcatalog.SearchResult
	BuildOptions  = internalcatalog.BuildOptions
	Builder       = internalcatalog.Builder
	ExportOptions = internalcatalog.ExportOptions
)

// Re-exported ModuleType constants.
const (
	ModuleTypeResource = internalcatalog.ModuleTypeResource
	ModuleTypeData     = internalcatalog.ModuleTypeData
)

// Re-exported file/path constants.
const SchemaFileName = internalcatalog.SchemaFileName

// Search filters and ranks modules in s according to opts.
// See internal/catalog.Search for documentation.
var Search = internalcatalog.Search

// NewBuilder constructs a Builder backed by the given registry client.
// See internal/catalog.NewBuilder for documentation.
var NewBuilder = internalcatalog.NewBuilder

// Export serialises a Schema to disk. See internal/catalog.Export.
var Export = internalcatalog.Export
