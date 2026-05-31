package catalog

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSchema() *Schema {
	return &Schema{
		SchemaVersion:   SchemaVersion,
		Provider:        "hashicorp/aws",
		ProviderVersion: "5.42.0",
		GeneratedAt:     time.Date(2026, 4, 22, 10, 30, 0, 0, time.UTC),
		Modules: []ModuleEntry{
			{
				Name: "aws_vpc",
				Type: ModuleTypeResource,
				Variables: []Variable{
					{Name: "cidr_block", Type: "string", Required: true},
				},
				Outputs: []Output{{Name: "id"}},
			},
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validSchema().Validate())
}

func TestValidate_NilSchema(t *testing.T) {
	t.Parallel()
	var s *Schema
	err := s.Validate()
	require.Error(t, err)
	ve, ok := AsValidationError(err)
	require.True(t, ok)
	require.Len(t, ve.Issues, 1)
}

// fieldsOf returns the set of issue fields for substring assertions in
// table tests.
func fieldsOf(err error) []string {
	ve, ok := AsValidationError(err)
	if !ok {
		return nil
	}
	out := make([]string, len(ve.Issues))
	for i, iss := range ve.Issues {
		out[i] = iss.String()
	}
	return out
}

func TestValidate_TopLevelFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		mutate      func(*Schema)
		wantField   string
		wantMessage string
	}{
		{"missing schema_version", func(s *Schema) { s.SchemaVersion = "" }, "schema_version", "is required"},
		{"unsupported schema_version", func(s *Schema) { s.SchemaVersion = "2.0" }, "schema_version", "unsupported"},
		{"missing provider", func(s *Schema) { s.Provider = "" }, "provider", "is required"},
		{"bad provider", func(s *Schema) { s.Provider = "Hashicorp/AWS" }, "provider", "must match"},
		{"missing provider_version", func(s *Schema) { s.ProviderVersion = "" }, "provider_version", "is required"},
		{"bad provider_version", func(s *Schema) { s.ProviderVersion = "latest" }, "provider_version", "semver"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := validSchema()
			tc.mutate(s)
			err := s.Validate()
			require.Error(t, err)
			joined := strings.Join(fieldsOf(err), " | ")
			assert.Contains(t, joined, tc.wantField)
			assert.Contains(t, joined, tc.wantMessage)
		})
	}
}

func TestValidate_ModuleErrors(t *testing.T) {
	t.Parallel()
	s := &Schema{
		SchemaVersion:   SchemaVersion,
		Provider:        "hashicorp/aws",
		ProviderVersion: "5.42.0",
		Modules: []ModuleEntry{
			{Name: "", Type: ModuleTypeResource},
			{Name: "1bad", Type: ModuleTypeResource},
			{Name: "ok", Type: ModuleType("module")},
			{Name: "ok", Type: ModuleTypeResource},
			{Name: "ok", Type: ModuleTypeResource},
		},
	}
	err := s.Validate()
	require.Error(t, err)
	joined := strings.Join(fieldsOf(err), " | ")
	assert.Contains(t, joined, "modules[0].name")
	assert.Contains(t, joined, "modules[1].name")
	assert.Contains(t, joined, "modules[2].type")
	assert.Contains(t, joined, "duplicate module name")
}

func TestValidate_VariableErrors(t *testing.T) {
	t.Parallel()
	s := validSchema()
	s.Modules[0].Variables = []Variable{
		{Name: "", Type: "string"},
		{Name: "good", Type: ""},
		{Name: "good", Type: "string"},
		{Name: "with_default", Type: "string", Required: true, Default: "x"},
	}
	err := s.Validate()
	require.Error(t, err)
	joined := strings.Join(fieldsOf(err), " | ")
	assert.Contains(t, joined, "variables[0].name")
	assert.Contains(t, joined, "variables[1].type")
	assert.Contains(t, joined, "duplicate variable name")
	assert.Contains(t, joined, "variables[3].default")
}

func TestValidate_OutputErrors(t *testing.T) {
	t.Parallel()
	s := validSchema()
	s.Modules[0].Outputs = []Output{
		{Name: ""},
		{Name: "1bad"},
		{Name: "good"},
		{Name: "good"},
	}
	err := s.Validate()
	require.Error(t, err)
	joined := strings.Join(fieldsOf(err), " | ")
	assert.Contains(t, joined, "outputs[0].name")
	assert.Contains(t, joined, "outputs[1].name")
	assert.Contains(t, joined, "duplicate output name")
}

func TestValidationError_ErrorString(t *testing.T) {
	t.Parallel()
	ve := &ValidationError{Issues: []ValidationIssue{
		{Field: "provider", Message: "is required"},
		{Message: "schema is nil"},
	}}
	assert.Equal(t,
		"catalog validation failed: provider: is required; schema is nil",
		ve.Error())
}

func TestValidate_References_Valid(t *testing.T) {
	t.Parallel()
	s := validSchema()
	s.Modules = append(s.Modules, ModuleEntry{
		Name: "aws_subnet", Type: ModuleTypeResource,
		Variables: []Variable{{
			Name: "vpc_id", Type: "string", Required: true,
			References: []VariableReference{{Module: "aws_vpc", Output: "id"}},
		}},
		Outputs: []Output{{Name: "id"}},
	})
	require.NoError(t, s.Validate())
}

func TestValidate_References_Errors(t *testing.T) {
	t.Parallel()
	s := validSchema()
	s.Modules = append(s.Modules, ModuleEntry{
		Name: "aws_subnet", Type: ModuleTypeResource,
		Variables: []Variable{
			{Name: "v_unknown_mod", Type: "string", References: []VariableReference{{Module: "missing", Output: "id"}}},
			{Name: "v_unknown_out", Type: "string", References: []VariableReference{{Module: "aws_vpc", Output: "nope"}}},
			{Name: "v_self", Type: "string", References: []VariableReference{{Module: "aws_subnet", Output: "id"}}},
			{Name: "v_blank", Type: "string", References: []VariableReference{{Module: "", Output: ""}}},
		},
		Outputs: []Output{{Name: "id"}},
	})
	err := s.Validate()
	require.Error(t, err)
	joined := strings.Join(fieldsOf(err), " | ")
	assert.Contains(t, joined, `unknown module "missing"`)
	assert.Contains(t, joined, `module "aws_vpc" has no output "nope"`)
	assert.Contains(t, joined, `self-reference to "aws_subnet"`)
	assert.Contains(t, joined, "references[0].module: is required")
	assert.Contains(t, joined, "references[0].output: is required")
}
