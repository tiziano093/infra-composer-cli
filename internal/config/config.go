package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the consolidated runtime configuration for infra-composer.
// All fields have sensible defaults from Defaults().
type Config struct {
	Logging      LogConfig       `mapstructure:"logging"`
	Catalog      CatalogConfig   `mapstructure:"catalog"`
	Terraform    TerraformConfig `mapstructure:"terraform"`
	Git          GitConfig       `mapstructure:"git"`
	OutputFormat string          `mapstructure:"output_format"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type CatalogConfig struct {
	SchemaPath string `mapstructure:"schema"`
	ModulesDir string `mapstructure:"modules_dir"`
}

type TerraformConfig struct {
	OutputDir  string `mapstructure:"output_dir"`
	CIProvider string `mapstructure:"ci_provider"`
}

type GitConfig struct {
	SourceTemplate string `mapstructure:"source_template"`
}

// EnvPrefix is the prefix for all environment variable overrides.
const EnvPrefix = "INFRA_COMPOSER"

// LoadOptions parameterises Load.
type LoadOptions struct {
	// ConfigFile, if non-empty, is the explicit path to a YAML config file.
	// If empty, the default ~/.infra-composer/config.yaml is consulted when
	// it exists; missing default file is not an error.
	ConfigFile string
}

// Load builds a Config by merging defaults, the YAML file (if any) and
// environment variables. CLI flag overrides are applied by callers after
// Load returns, since flag values are owned by the cobra command tree.
func Load(opts LoadOptions) (*Config, error) {
	v := viper.New()
	cfg := Defaults()

	// Seed Viper with defaults so env/file overrides merge cleanly.
	v.SetDefault("logging.level", cfg.Logging.Level)
	v.SetDefault("logging.format", cfg.Logging.Format)
	v.SetDefault("catalog.schema", cfg.Catalog.SchemaPath)
	v.SetDefault("catalog.modules_dir", cfg.Catalog.ModulesDir)
	v.SetDefault("terraform.output_dir", cfg.Terraform.OutputDir)
	v.SetDefault("terraform.ci_provider", cfg.Terraform.CIProvider)
	v.SetDefault("git.source_template", cfg.Git.SourceTemplate)
	v.SetDefault("output_format", cfg.OutputFormat)

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// Bind common flat env vars documented in ARCHITECTURE.md so users can
	// set them without nested key syntax.
	_ = v.BindEnv("logging.level", EnvPrefix+"_LOG_LEVEL")
	_ = v.BindEnv("catalog.schema", EnvPrefix+"_SCHEMA")
	_ = v.BindEnv("catalog.modules_dir", EnvPrefix+"_MODULES_DIR")
	_ = v.BindEnv("terraform.output_dir", EnvPrefix+"_OUTPUT_DIR")
	_ = v.BindEnv("terraform.ci_provider", EnvPrefix+"_CI_PROVIDER")
	_ = v.BindEnv("output_format", EnvPrefix+"_FORMAT")

	path, err := resolveConfigPath(opts.ConfigFile)
	if err != nil {
		return nil, err
	}
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			// Explicit path requested but unreadable -> hard error.
			if opts.ConfigFile != "" {
				return nil, fmt.Errorf("read config %s: %w", path, err)
			}
			// Default path: only ignore "not found"-class issues.
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) && !os.IsNotExist(err) {
				return nil, fmt.Errorf("read default config %s: %w", path, err)
			}
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// resolveConfigPath returns the absolute path of the config file to read,
// or empty string if no file should be considered.
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// No home dir is fine — we just skip the default file.
		return "", nil
	}
	candidate := filepath.Join(home, ".infra-composer", "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", nil
}
