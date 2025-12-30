package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// schemaFile represents the YAML file structure
type schemaFile struct {
	Config map[string]configEntry `yaml:"config"`
}

// configEntry represents a single config entry in YAML
type configEntry struct {
	Type     string   `yaml:"type"`
	Required bool     `yaml:"required"`
	Values   []string `yaml:"values,omitempty"`
}

// ParseSchema parses YAML content into a Schema
func ParseSchema(content []byte) (Schema, error) {
	var sf schemaFile
	if err := yaml.Unmarshal(content, &sf); err != nil {
		return Schema{}, fmt.Errorf("invalid YAML: %w", err)
	}

	schema := Schema{
		Config: make(map[string]ConfigKey),
	}

	for path, entry := range sf.Config {
		configType := ConfigType(entry.Type)

		// Validate type
		if configType != TypeString && configType != TypeEnum {
			return Schema{}, fmt.Errorf("unknown type '%s' for config '%s'", entry.Type, path)
		}

		// Validate enum has values
		if configType == TypeEnum && len(entry.Values) == 0 {
			return Schema{}, fmt.Errorf("enum type requires 'values' for config '%s'", path)
		}

		schema.Config[path] = ConfigKey{
			Path:     path,
			Type:     configType,
			Required: entry.Required,
			Values:   entry.Values,
		}
	}

	return schema, nil
}

// ToYAML serializes a Schema back to YAML bytes
func (s Schema) ToYAML() ([]byte, error) {
	sf := schemaFile{
		Config: make(map[string]configEntry),
	}

	for path, key := range s.Config {
		sf.Config[path] = configEntry{
			Type:     string(key.Type),
			Required: key.Required,
			Values:   key.Values,
		}
	}

	return yaml.Marshal(&sf)
}

// LoadSchema reads and parses admit.yaml from the given directory
func LoadSchema(dir string) (Schema, error) {
	path := filepath.Join(dir, "admit.yaml")

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Schema{}, fmt.Errorf("no admit.yaml found in %s", dir)
		}
		return Schema{}, fmt.Errorf("failed to read admit.yaml: %w", err)
	}

	return ParseSchema(content)
}
