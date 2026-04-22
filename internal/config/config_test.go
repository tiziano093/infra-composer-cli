package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	t.Parallel()
	c := Defaults()
	assert.Equal(t, "info", c.Logging.Level)
	assert.Equal(t, "text", c.Logging.Format)
	assert.Equal(t, "./stack", c.Terraform.OutputDir)
	assert.Equal(t, "github", c.Terraform.CIProvider)
	assert.Equal(t, "table", c.OutputFormat)
}

func TestLoad_NoFile_ReturnsDefaults(t *testing.T) {
	// Isolate HOME so default config path cannot resolve.
	t.Setenv("HOME", t.TempDir())
	clearEnv(t)

	cfg, err := Load(LoadOptions{})
	require.NoError(t, err)
	assert.Equal(t, Defaults(), cfg)
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearEnv(t)
	t.Setenv("INFRA_COMPOSER_LOG_LEVEL", "debug")
	t.Setenv("INFRA_COMPOSER_OUTPUT_DIR", "/tmp/stack")
	t.Setenv("INFRA_COMPOSER_CI_PROVIDER", "azure")
	t.Setenv("INFRA_COMPOSER_FORMAT", "json")

	cfg, err := Load(LoadOptions{})
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "/tmp/stack", cfg.Terraform.OutputDir)
	assert.Equal(t, "azure", cfg.Terraform.CIProvider)
	assert.Equal(t, "json", cfg.OutputFormat)
}

func TestLoad_FileOverridesDefaults_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	clearEnv(t)
	path := filepath.Join(dir, "infra.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
logging:
  level: warn
  format: json
terraform:
  output_dir: /tmp/from-file
output_format: yaml
`), 0o644))

	cfg, err := Load(LoadOptions{ConfigFile: path})
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "/tmp/from-file", cfg.Terraform.OutputDir)
	assert.Equal(t, "yaml", cfg.OutputFormat)

	t.Setenv("INFRA_COMPOSER_LOG_LEVEL", "error")
	cfg, err = Load(LoadOptions{ConfigFile: path})
	require.NoError(t, err)
	assert.Equal(t, "error", cfg.Logging.Level, "env should override file")
}

func TestLoad_ExplicitMissingFile_IsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearEnv(t)
	_, err := Load(LoadOptions{ConfigFile: filepath.Join(t.TempDir(), "missing.yaml")})
	assert.Error(t, err)
}

// clearEnv unsets every INFRA_COMPOSER_* var seen in the current process so
// tests are deterministic regardless of the developer's shell state.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				if len(kv) >= len(EnvPrefix)+1 && kv[:len(EnvPrefix)+1] == EnvPrefix+"_" {
					t.Setenv(kv[:i], "")
					_ = os.Unsetenv(kv[:i])
				}
				break
			}
		}
	}
}
