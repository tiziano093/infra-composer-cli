// Package config loads infra-composer configuration from defaults, an
// optional YAML file, INFRA_COMPOSER_* environment variables, and CLI
// flags. Higher layers override lower ones.
package config

// Default values for every Config field. Returned by reference so callers
// can mutate without affecting other invocations.
func Defaults() *Config {
	return &Config{
		Logging: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Catalog: CatalogConfig{
			SchemaPath: "",
			ModulesDir: "",
		},
		Terraform: TerraformConfig{
			OutputDir:  "./stack",
			CIProvider: "github",
		},
		Git: GitConfig{
			SourceTemplate: "",
		},
		OutputFormat: "table",
	}
}
