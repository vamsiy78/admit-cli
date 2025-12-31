package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"admit/internal/invariant"

	"gopkg.in/yaml.v3"
)

// schemaFile represents the YAML file structure
type schemaFile struct {
	Config     map[string]configEntry `yaml:"config"`
	Invariants []invariantEntry       `yaml:"invariants,omitempty"`
}

// configEntry represents a single config entry in YAML
type configEntry struct {
	Type     string   `yaml:"type"`
	Required bool     `yaml:"required"`
	Values   []string `yaml:"values,omitempty"`
}

// invariantEntry represents a single invariant entry in YAML
type invariantEntry struct {
	Name string `yaml:"name"`
	Rule string `yaml:"rule"`
}

// invariantNameRegex validates invariant names: alphanumeric, hyphens, underscores
var invariantNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ParseSchema parses YAML content into a Schema
func ParseSchema(content []byte) (Schema, error) {
	var sf schemaFile
	if err := yaml.Unmarshal(content, &sf); err != nil {
		return Schema{}, fmt.Errorf("invalid YAML: %w", err)
	}

	schema := Schema{
		Config:     make(map[string]ConfigKey),
		Invariants: []invariant.Invariant{},
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

	// Parse invariants if present
	if len(sf.Invariants) > 0 {
		// Collect config keys for validation
		configKeys := make([]string, 0, len(schema.Config))
		for k := range schema.Config {
			configKeys = append(configKeys, k)
		}

		// Track names for uniqueness validation
		seenNames := make(map[string]bool)

		for i, inv := range sf.Invariants {
			// Validate name is present
			if inv.Name == "" {
				return Schema{}, fmt.Errorf("invariant at index %d: missing required field 'name'", i)
			}

			// Validate name format
			if !invariantNameRegex.MatchString(inv.Name) {
				return Schema{}, fmt.Errorf("invariant name '%s' contains invalid characters", inv.Name)
			}

			// Validate name uniqueness
			if seenNames[inv.Name] {
				return Schema{}, fmt.Errorf("duplicate invariant name: '%s'", inv.Name)
			}
			seenNames[inv.Name] = true

			// Validate rule is present
			if inv.Rule == "" {
				return Schema{}, fmt.Errorf("invariant '%s': missing required field 'rule'", inv.Name)
			}

			// Parse rule expression
			expr, err := invariant.ParseRule(inv.Rule, configKeys)
			if err != nil {
				return Schema{}, fmt.Errorf("invariant '%s': invalid rule syntax: %w", inv.Name, err)
			}

			schema.Invariants = append(schema.Invariants, invariant.Invariant{
				Name: inv.Name,
				Rule: inv.Rule,
				Expr: expr,
			})
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

	// Serialize invariants if present
	for _, inv := range s.Invariants {
		sf.Invariants = append(sf.Invariants, invariantEntry{
			Name: inv.Name,
			Rule: inv.Rule,
		})
	}

	return yaml.Marshal(&sf)
}

// LoadSchema reads and parses admit.yaml from the given directory
func LoadSchema(dir string) (Schema, error) {
	path := filepath.Join(dir, "admit.yaml")
	return LoadSchemaFromPath(path)
}

// LoadSchemaFromPath reads and parses a schema from the given file path
func LoadSchemaFromPath(path string) (Schema, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Schema{}, err
		}
		return Schema{}, fmt.Errorf("failed to read schema: %w", err)
	}

	return ParseSchema(content)
}
