package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"admit/internal/contract"
	"admit/internal/invariant"

	"gopkg.in/yaml.v3"
)

// schemaFile represents the YAML file structure
type schemaFile struct {
	Config       map[string]configEntry      `yaml:"config"`
	Invariants   []invariantEntry            `yaml:"invariants,omitempty"`
	Environments map[string]environmentEntry `yaml:"environments,omitempty"`
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

// environmentEntry represents a single environment contract in YAML
type environmentEntry struct {
	Allow map[string]ruleEntry `yaml:"allow,omitempty"`
	Deny  map[string]ruleEntry `yaml:"deny,omitempty"`
}

// ruleEntry represents a rule value that can be a single string or array of strings
type ruleEntry struct {
	values []string
}

// UnmarshalYAML implements custom unmarshaling for ruleEntry to handle both
// single values and arrays
func (r *ruleEntry) UnmarshalYAML(value *yaml.Node) error {
	// Try to unmarshal as a single string first
	var single string
	if err := value.Decode(&single); err == nil {
		r.values = []string{single}
		return nil
	}

	// Try to unmarshal as an array of strings
	var array []string
	if err := value.Decode(&array); err == nil {
		r.values = array
		return nil
	}

	return fmt.Errorf("rule value must be a string or array of strings")
}

// MarshalYAML implements custom marshaling for ruleEntry
// Single values are serialized as strings, multiple values as arrays
func (r ruleEntry) MarshalYAML() (interface{}, error) {
	if len(r.values) == 1 {
		return r.values[0], nil
	}
	return r.values, nil
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
		Config:       make(map[string]ConfigKey),
		Invariants:   []invariant.Invariant{},
		Environments: make(map[string]contract.Contract),
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

	// Parse environments if present
	if len(sf.Environments) > 0 {
		for envName, envEntry := range sf.Environments {
			c, err := parseEnvironmentContract(envName, envEntry)
			if err != nil {
				return Schema{}, fmt.Errorf("environment '%s': %w", envName, err)
			}
			schema.Environments[envName] = c
		}
	}

	return schema, nil
}

// parseEnvironmentContract converts an environmentEntry to a contract.Contract
func parseEnvironmentContract(name string, entry environmentEntry) (contract.Contract, error) {
	c := contract.Contract{
		Name:  name,
		Allow: make(map[string]contract.Rule),
		Deny:  make(map[string]contract.Rule),
	}

	// Parse allow rules
	for key, rule := range entry.Allow {
		if len(rule.values) == 0 {
			return contract.Contract{}, fmt.Errorf("allow rule for '%s' has no values", key)
		}
		c.Allow[key] = contract.Rule{
			Values: rule.values,
			IsGlob: false, // Allow rules don't support glob patterns
		}
	}

	// Parse deny rules
	for key, rule := range entry.Deny {
		if len(rule.values) == 0 {
			return contract.Contract{}, fmt.Errorf("deny rule for '%s' has no values", key)
		}
		// Check if any value contains a glob pattern
		isGlob := false
		for _, v := range rule.values {
			if strings.Contains(v, "*") {
				isGlob = true
				break
			}
		}
		c.Deny[key] = contract.Rule{
			Values: rule.values,
			IsGlob: isGlob,
		}
	}

	return c, nil
}

// ToYAML serializes a Schema back to YAML bytes
func (s Schema) ToYAML() ([]byte, error) {
	sf := schemaFile{
		Config:       make(map[string]configEntry),
		Environments: make(map[string]environmentEntry),
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

	// Serialize environments if present
	for envName, c := range s.Environments {
		envEntry := environmentEntry{
			Allow: make(map[string]ruleEntry),
			Deny:  make(map[string]ruleEntry),
		}
		for key, rule := range c.Allow {
			envEntry.Allow[key] = ruleEntry{values: rule.Values}
		}
		for key, rule := range c.Deny {
			envEntry.Deny[key] = ruleEntry{values: rule.Values}
		}
		sf.Environments[envName] = envEntry
	}

	// Remove empty environments map to avoid serializing empty section
	if len(sf.Environments) == 0 {
		sf.Environments = nil
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
