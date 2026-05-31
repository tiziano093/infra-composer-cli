package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleType_IsValid(t *testing.T) {
	t.Parallel()
	assert.True(t, ModuleTypeResource.IsValid())
	assert.True(t, ModuleTypeData.IsValid())
	assert.False(t, ModuleType("").IsValid())
	assert.False(t, ModuleType("module").IsValid())
}

func TestSchemaVersion_Constant(t *testing.T) {
	t.Parallel()
	require.Equal(t, "1.0", SchemaVersion)
}
