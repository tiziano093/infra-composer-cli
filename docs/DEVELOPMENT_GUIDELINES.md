# Infra-Composer CLI — Development Guidelines

**Version:** 1.0  
**Status:** Active  
**Audience:** Developers, Architects  
**Last Updated:** 2026-04-22

---

## Code Organization Principles

### 1. Single Responsibility
Each package should have one clear purpose:
- `internal/catalog/` → Catalog logic
- `internal/terraform/` → Terraform generation
- `internal/config/` → Configuration loading
- `internal/output/` → Output formatting

### 2. Package Hierarchy
```
cmd/           ← Entry point (no business logic)
  ↓
internal/cli/  ← CLI framework (command dispatch)
  ↓
internal/commands/  ← Command handlers (orchestration)
  ↓
internal/{catalog,terraform,config,etc.}/  ← Domain logic
```

### 3. Interface-Based Design
- Define interfaces at the point of use (consumer-driven)
- Keep interfaces small (1-3 methods when possible)
- Use interfaces for external dependencies (filesystem, HTTP, logging)

### 4. Error Handling
- Define custom error types per domain package
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Commands convert domain errors to CLI errors

---

## Naming Conventions

### Packages
- Short, lowercase, single word when possible
- Plural for collections: `commands`, `templates`
- Avoid underscores: ✅ `catalog` ✗ `cat_alog`

### Types & Functions
- PascalCase for exported (public)
- camelCase for unexported (private)
- Descriptive names: ✅ `SchemaValidator` ✗ `Validator`

### Variables
- Short names acceptable in small scopes: `i`, `err`
- Descriptive for larger scopes: `catalogSchema`, `outputFormatter`
- Boolean names: `isValid`, `hasErrors`, `shouldContinue`

### Constants
- UPPER_SNAKE_CASE for package-level constants
- Grouped logically
- Well-commented for non-obvious values

### Files
- `*_test.go` for unit tests
- `*_integration_test.go` for integration tests
- No `_utils.go` files (split into domain-specific files)

### Example
```go
// Good: Clear package name, exported type, private functions
package catalog

type Schema struct { ... }

func (s *Schema) Validate() error { ... }

func (s *Schema) validate() error { ... }  // private helper
```

---

## Code Style

### Formatting
- Run `gofmt -w .` and `goimports -w .` before commit
- Configure IDE to format on save
- 80-character soft limit for readability (hard limit: 120)

### Comments
- Comment exported types, functions, constants
- Explain **why**, not **what** (code shows what)
- Add complexity explanations for non-obvious logic

```go
// Good: Explains rationale
// We memoize dependency resolution results to avoid O(n²) recomputation
// when multiple modules depend on the same dependency tree.
func (r *Resolver) resolveDependencies(module string) []string {
    // ...
}

// Avoid: Obvious comment
// Parse the schema
schema, err := parseSchema(file)
```

### Imports
- Grouped: stdlib, external, internal
- Use goimports for automatic organization

```go
import (
    "fmt"
    "io/ioutil"
    
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
    
    "github.com/tiziano093/infra-composer-cli/internal/config"
)
```

### Error Handling
```go
// Good: Add context, don't lose information
if err := readFile(path); err != nil {
    return fmt.Errorf("read config file: %w", err)
}

// Avoid: Silent failures
if err := readFile(path); err != nil {
    return nil  // ✗ Lost error information
}

// Avoid: Generic wrapping
if err := readFile(path); err != nil {
    return errors.New("failed")  // ✗ Lost original error
}
```

### Pointers vs Values
- Use pointers for:
  - Struct methods (consistency)
  - Large structs (>128 bytes)
  - When you need to modify receiver
- Use values for:
  - Small structs (<64 bytes)
  - Immutable data
  - API parameters when passing won't be modified

---

## Testing Guidelines

### Unit Tests

**Location:** `test/unit/` or alongside source code  
**Naming:** `package_test.go` (separate package) or `_test.go` suffix

**Structure:**
```go
func TestSchemaValidate(t *testing.T) {
    // Arrange: Set up test data
    schema := &Schema{
        Provider: "aws",
        Modules:  []ModuleEntry{...},
    }
    
    // Act: Call the function
    err := schema.Validate()
    
    // Assert: Verify results
    assert.NoError(t, err)
    assert.Equal(t, "aws", schema.Provider)
}
```

**Best Practices:**
- One assertion per test (or related assertions)
- Use table-driven tests for multiple cases
- Mock external dependencies (HTTP, filesystem)
- Avoid testing implementation details

**Table-Driven Tests:**
```go
func TestSearchModules(t *testing.T) {
    tests := []struct {
        name     string
        query    string
        expected int
        wantErr  bool
    }{
        {"empty query", "", 0, true},
        {"single keyword", "vpc", 5, false},
        {"multiple keywords", "vpc peering", 2, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            results, err := Search(tt.query)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Len(t, results, tt.expected)
            }
        })
    }
}
```

### Integration Tests

**Location:** `test/integration/`  
**Naming:** `*_integration_test.go`

**Strategy:**
- Use fixtures (mock schemas, example requests)
- Test full workflow end-to-end
- Mock external APIs (Terraform Registry)
- Don't hit real external APIs

**Example:**
```go
func TestComposeWorkflow(t *testing.T) {
    // Load mock schema from fixture
    schema := loadSchema(t, "test/fixtures/schemas/aws-schema.json")
    
    // Execute compose command
    result, err := Compose(context.Background(), ComposeRequest{
        Schema:   schema,
        Modules:  []string{"aws_vpc", "aws_subnet"},
        OutputDir: t.TempDir(),
    })
    
    // Verify files generated
    assert.NoError(t, err)
    assert.FileExists(t, filepath.Join(result.OutputDir, "providers.tf"))
    assert.FileExists(t, filepath.Join(result.OutputDir, "main.tf"))
}
```

### Test Coverage

- **Target:** 80%+ coverage for business logic
- **Acceptable gaps:** Error handling for rare edge cases
- **Measure:** `go test ./... -cover`
- **Report:** GitHub Actions uploads to codecov.io

---

## Common Patterns

### Error Types
```go
package catalog

// Define custom error types
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// Usage in functions
func (s *Schema) Validate() error {
    if s.Provider == "" {
        return &ValidationError{
            Field:   "provider",
            Message: "required",
        }
    }
    return nil
}
```

### Dependency Injection
```go
type CatalogService struct {
    logger Logger
    client *http.Client
    cache  Cache
}

// Constructor
func NewCatalogService(logger Logger, client *http.Client, cache Cache) *CatalogService {
    return &CatalogService{
        logger: logger,
        client: client,
        cache:  cache,
    }
}

// Methods
func (cs *CatalogService) Build(ctx context.Context, provider string) error {
    cs.logger.Infof("Building catalog for provider: %s", provider)
    // ...
}
```

### Configuration Pattern
```go
type Config struct {
    // Unexported fields with public accessors
    logLevel string
    outputDir string
}

// Getters
func (c *Config) LogLevel() string {
    return c.logLevel
}

// Setters with validation
func (c *Config) SetOutputDir(path string) error {
    if path == "" {
        return errors.New("output directory required")
    }
    c.outputDir = path
    return nil
}
```

### Builder Pattern (for complex objects)
```go
type ComposeRequestBuilder struct {
    request ComposeRequest
}

func NewComposeRequest() *ComposeRequestBuilder {
    return &ComposeRequestBuilder{}
}

func (b *ComposeRequestBuilder) WithModules(modules []string) *ComposeRequestBuilder {
    b.request.Modules = modules
    return b
}

func (b *ComposeRequestBuilder) WithOutputDir(dir string) *ComposeRequestBuilder {
    b.request.OutputDir = dir
    return b
}

func (b *ComposeRequestBuilder) Build() (ComposeRequest, error) {
    if err := b.request.Validate(); err != nil {
        return ComposeRequest{}, err
    }
    return b.request, nil
}

// Usage
req, err := NewComposeRequest().
    WithModules([]string{"vpc", "subnet"}).
    WithOutputDir("./stack").
    Build()
```

---

## Performance Considerations

### Startup Performance
- Avoid loading large files at initialization
- Lazy-load config/schema until needed
- Use goroutines for independent operations (catalog discovery, file I/O)

### Memory Usage
- Preallocate slices when size is known: `make([]string, 0, len(items))`
- Avoid unnecessary string concatenations: use `strings.Builder`
- Stream large files instead of loading into memory

### Concurrency
- Use goroutines for I/O-bound operations (HTTP, filesystem)
- Use sync.WaitGroup for goroutine coordination
- Avoid shared state; pass data through channels when possible

**Example:**
```go
// Good: Parallel crawling with bounded concurrency
func (b *Builder) crawlProviders(ctx context.Context, providers []string) error {
    sem := make(chan struct{}, 5)  // Limit to 5 concurrent requests
    var g errgroup.Group
    
    for _, provider := range providers {
        provider := provider  // Capture for goroutine
        g.Go(func() error {
            sem <- struct{}{}      // Acquire
            defer func() { <-sem }() // Release
            return b.crawlProvider(ctx, provider)
        })
    }
    
    return g.Wait()
}
```

---

## Documentation

### Code Comments
- Exported types: Describe purpose and usage
- Non-obvious algorithms: Explain approach
- Workarounds: Explain why not obvious solution

```go
// SchemaBuilder constructs a catalog schema from discovered Terraform providers.
// It implements a 4-step pipeline: discover → crawl → normalize → generate.
type SchemaBuilder struct { ... }

// Build returns a fully populated Schema for the given provider.
// It queries the Terraform Registry API to discover all resources and data sources,
// then normalizes them into the internal schema format.
func (sb *SchemaBuilder) Build(ctx context.Context, provider string) (*Schema, error) {
    // ...
}
```

### Example Code
- Provide runnable examples in `examples/` for major features
- Keep examples concise and focused on single feature
- Update examples when API changes

### README Structure
For each package:
- One-line description
- Example usage
- Key types/functions
- Links to related packages

---

## Refactoring Guidelines

### When to Refactor
- Code duplication (>3 similar blocks)
- Complex nested logic (>3 levels)
- Single function >50 lines
- Low test coverage (<70%)

### Refactoring Checklist
- ✓ Tests pass before refactoring
- ✓ Refactor in small commits
- ✓ One logical change per commit
- ✓ Tests pass after each change
- ✓ No behavior changes (only structure)

### Safe Refactoring Techniques
1. Extract method: Move logic to new function
2. Extract type: Move related data to new struct
3. Rename: Use IDE rename-all feature
4. Consolidate: Merge similar functions
5. Move: Transfer to more appropriate package

---

## Dependency Management

### Adding Dependencies
- Use `go get <package>@<version>`
- Prefer stable, mature packages
- Check license compatibility
- Keep dependency count reasonable

### Updating Dependencies
- Regular `go get -u ./...` for patch updates
- Review CHANGELOG for minor/major updates
- Run full test suite after updates
- Test on target platforms before releasing

### Vendoring
- Not required; go.mod is sufficient
- Use modules for reproducible builds

---

## IDE Configuration

### VS Code (recommended)
```json
{
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  },
  "go.lintOnSave": "package",
  "go.lintTool": "golangci-lint",
  "go.coverageDecorator": {
    "enabled": true,
    "type": "gutter"
  }
}
```

### Essential Go Extensions
- Go (golang.go)
- Go Tools (golang.go-tools)
- Golangci-lint (golangci.golangci-lint)

---

## Code Review Checklist

Before submitting PR:
- [ ] Code follows style guidelines (run `make fmt`)
- [ ] Tests added/updated with >80% coverage
- [ ] Comments explain non-obvious logic
- [ ] Error handling covers edge cases
- [ ] No hardcoded values (use constants)
- [ ] Performance acceptable (no unnecessary allocations)
- [ ] Documentation updated

---

## Anti-Patterns to Avoid

| ✗ Anti-Pattern | ✓ Better Approach |
|---|---|
| Global variables | Pass as dependencies |
| Large functions (>50 lines) | Extract helpers |
| Generic package names (`utils`, `helpers`) | Domain-specific packages |
| Silent error failures | Log and return error |
| Comments on obvious code | Focus on "why" not "what" |
| Tight coupling to external APIs | Define interfaces |
| Mixing concerns in one file | Separate by responsibility |

---

## Common Mistakes & Fixes

### Mistake: Interface bloat
```go
// ✗ Too many methods in one interface
type Repository interface {
    Create(ctx context.Context, item Item) error
    Read(ctx context.Context, id string) (Item, error)
    Update(ctx context.Context, item Item) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context) ([]Item, error)
    // ... 10 more methods
}

// ✓ Split into focused interfaces
type Reader interface {
    Read(ctx context.Context, id string) (Item, error)
}

type Writer interface {
    Create(ctx context.Context, item Item) error
    Update(ctx context.Context, item Item) error
    Delete(ctx context.Context, id string) error
}
```

### Mistake: Error type assertion in multiple places
```go
// ✗ Repeated type assertions
if err != nil {
    if ve, ok := err.(*ValidationError); ok {
        // handle validation error
    }
}

// ✓ Use errors.As (Go 1.13+)
var ve *ValidationError
if errors.As(err, &ve) {
    // handle validation error
}
```

---

## Useful Resources

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Conventions](https://golang.org/cmd/go/)
- Project docs: ARCHITECTURE.md, TESTING_STRATEGY.md

---

**Last Updated:** 2026-04-22  
**Next Review:** After Phase 1 code review  
