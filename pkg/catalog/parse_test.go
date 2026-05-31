package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixturePath resolves a test fixture under <repo>/test/fixtures/schemas.
// It walks upward from the package dir so the helper works regardless of
// where `go test` is invoked from.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "test", "fixtures", "schemas", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("fixture %s not found from %s", name, dir)
	return ""
}

func TestParse_ValidMinimal(t *testing.T) {
	t.Parallel()
	f, err := os.Open(fixturePath(t, "valid_minimal.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	s, err := Parse(f)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, SchemaVersion, s.SchemaVersion)
	assert.Equal(t, "hashicorp/aws", s.Provider)
	assert.Equal(t, "5.42.0", s.ProviderVersion)
	assert.Empty(t, s.Modules)
}

func TestParse_ValidFull_RoundTrip(t *testing.T) {
	t.Parallel()
	f, err := os.Open(fixturePath(t, "valid_full.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	s, err := Parse(f)
	require.NoError(t, err)
	require.Len(t, s.Modules, 3)

	vpc := s.Modules[0]
	assert.Equal(t, "aws_vpc", vpc.Name)
	assert.Equal(t, ModuleTypeResource, vpc.Type)
	assert.Equal(t, "network", vpc.Group)
	require.Len(t, vpc.Variables, 2)
	assert.True(t, vpc.Variables[0].Required)
	assert.Equal(t, true, vpc.Variables[1].Default)
	require.Len(t, vpc.Outputs, 2)

	identity := s.Modules[1]
	assert.Equal(t, ModuleTypeData, identity.Type)

	subnet := s.Modules[2]
	assert.Equal(t, "aws_subnet", subnet.Name)
	require.Len(t, subnet.Variables, 2)
	require.Len(t, subnet.Variables[0].References, 1)
	assert.Equal(t, "aws_vpc", subnet.Variables[0].References[0].Module)
	assert.Equal(t, "id", subnet.Variables[0].References[0].Output)
}

func TestParse_NilReader(t *testing.T) {
	t.Parallel()
	_, err := Parse(nil)
	require.Error(t, err)
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
}

func TestParse_Malformed(t *testing.T) {
	t.Parallel()
	f, err := os.Open(fixturePath(t, "invalid_malformed.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	_, err = Parse(f)
	require.Error(t, err)
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
}

func TestParse_RejectsUnknownFields(t *testing.T) {
	t.Parallel()
	body := `{"schema_version":"1.0","provider":"x/y","provider_version":"1.0.0","modules":[],"surprise":true}`
	_, err := Parse(strings.NewReader(body))
	require.Error(t, err)
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	assert.Contains(t, pe.Error(), "surprise")
}

func TestParse_RejectsTrailingData(t *testing.T) {
	t.Parallel()
	body := `{"schema_version":"1.0","provider":"x/y","provider_version":"1.0.0","modules":[]}{}`
	_, err := Parse(strings.NewReader(body))
	require.Error(t, err)
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestLoad_AnnotatesPathOnParseError(t *testing.T) {
	t.Parallel()
	path := fixturePath(t, "invalid_malformed.json")
	_, err := Load(path)
	require.Error(t, err)
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	assert.Equal(t, path, pe.Path)
}

func TestLoad_ValidFull(t *testing.T) {
	t.Parallel()
	s, err := Load(fixturePath(t, "valid_full.json"))
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "hashicorp/aws", s.Provider)
}

func TestLoad_ValidationErrorOnMissingProvider(t *testing.T) {
	t.Parallel()
	_, err := Load(fixturePath(t, "invalid_missing_provider.json"))
	require.Error(t, err)
	ve, ok := AsValidationError(err)
	require.True(t, ok)
	require.NotEmpty(t, ve.Issues)
}

func TestLoad_ValidationErrorOnDupModules(t *testing.T) {
	t.Parallel()
	_, err := Load(fixturePath(t, "invalid_dup_modules.json"))
	require.Error(t, err)
	ve, ok := AsValidationError(err)
	require.True(t, ok)
	found := false
	for _, iss := range ve.Issues {
		if strings.Contains(iss.Message, "duplicate module name") {
			found = true
		}
	}
	assert.True(t, found, "expected duplicate module name issue, got %v", ve.Issues)
}
