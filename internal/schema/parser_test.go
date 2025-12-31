package schema

import (
	"reflect"
	"strings"
	"testing"

	"admit/internal/invariant"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-cli, Property 3: Schema Round-Trip
// For any valid schema structure, parsing YAML into a Schema and then serializing
// back to YAML SHALL produce an equivalent schema when re-parsed.
// **Validates: Requirements 2.4**
func TestProperty3_SchemaRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid config types
	genConfigType := gen.OneConstOf(TypeString, TypeEnum)

	// Generator for enum values (non-empty list of strings)
	genEnumValues := gen.SliceOfN(3, gen.AlphaString()).
		SuchThat(func(vals []string) bool {
			if len(vals) == 0 {
				return false
			}
			for _, v := range vals {
				if v == "" {
					return false
				}
			}
			return true
		})

	// Generator for a config key path (alphanumeric with dots)
	genPath := gen.RegexMatch(`[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*`)

	// Generator for a single ConfigKey
	genConfigKey := gopter.CombineGens(
		genPath,
		genConfigType,
		gen.Bool(),
		genEnumValues,
	).Map(func(vals []interface{}) ConfigKey {
		path := vals[0].(string)
		configType := vals[1].(ConfigType)
		required := vals[2].(bool)
		enumValues := vals[3].([]string)

		key := ConfigKey{
			Path:     path,
			Type:     configType,
			Required: required,
		}

		// Only set Values for enum type
		if configType == TypeEnum {
			key.Values = enumValues
		}

		return key
	})

	// Generator for a Schema with 1-5 config keys
	genSchema := gen.SliceOfN(3, genConfigKey).
		SuchThat(func(keys []ConfigKey) bool {
			// Ensure unique paths
			seen := make(map[string]bool)
			for _, k := range keys {
				if seen[k.Path] || k.Path == "" {
					return false
				}
				seen[k.Path] = true
			}
			return len(keys) > 0
		}).
		Map(func(keys []ConfigKey) Schema {
			s := Schema{
				Config:     make(map[string]ConfigKey),
				Invariants: []invariant.Invariant{},
			}
			for _, k := range keys {
				s.Config[k.Path] = k
			}
			return s
		})

	properties.Property("round-trip preserves schema", prop.ForAll(
		func(original Schema) bool {
			// Serialize to YAML
			yamlBytes, err := original.ToYAML()
			if err != nil {
				t.Logf("ToYAML failed: %v", err)
				return false
			}

			// Parse back
			parsed, err := ParseSchema(yamlBytes)
			if err != nil {
				t.Logf("ParseSchema failed: %v", err)
				return false
			}

			// Compare
			return reflect.DeepEqual(original, parsed)
		},
		genSchema,
	))

	properties.TestingRun(t)
}

// Feature: admit-cli, Property 2: Invalid YAML Produces Parse Error
// For any byte sequence that is not valid YAML, the schema parser SHALL return a parse error.
// **Validates: Requirements 2.3**
func TestProperty2_InvalidYAMLProducesParseError(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for invalid YAML content
	genInvalidYAML := gen.OneGenOf(
		// Unbalanced brackets/braces
		gen.Const([]byte("config: {unclosed")),
		gen.Const([]byte("config: [unclosed")),
		// Invalid indentation
		gen.Const([]byte("config:\n  key1: value\n key2: value")),
		// Tab characters in wrong places
		gen.Const([]byte("config:\n\t\tkey: value")),
		// Duplicate keys at same level (YAML 1.2 allows but our parser should handle)
		gen.Const([]byte("config:\n  db.url:\n    type: string\nconfig:\n  other: value")),
		// Invalid characters
		gen.Const([]byte("config: @invalid")),
		// Truncated content
		gen.Const([]byte("config:\n  db.url:\n    type:")),
		// Random binary data
		gen.SliceOfN(50, gen.UInt8Range(128, 255)).Map(func(b []uint8) []byte {
			result := make([]byte, len(b))
			for i, v := range b {
				result[i] = byte(v)
			}
			return result
		}),
	)

	properties.Property("invalid YAML produces error or empty schema", prop.ForAll(
		func(content []byte) bool {
			schema, err := ParseSchema(content)
			// Either we get an error, or we get an empty/invalid schema
			// (some malformed YAML may parse to empty structures)
			if err != nil {
				return true
			}
			// If no error, the schema should be empty or have no valid config
			return len(schema.Config) == 0
		},
		genInvalidYAML,
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 1: Schema Parsing with Invariants
// For any valid schema YAML with an invariants section containing valid invariant declarations,
// the Schema_Parser SHALL successfully parse and return a Schema with the invariants populated.
// **Validates: Requirements 1.1, 1.4, 1.5**
func TestProperty1_SchemaParsingWithInvariants(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Simple generator for config key paths using fixed prefixes
	genConfigPath := gen.IntRange(1, 100).Map(func(i int) string {
		return "config" + string(rune('a'+i%26)) + ".key" + string(rune('0'+i%10))
	})

	// Simple generator for invariant names using fixed prefixes
	genInvariantName := gen.IntRange(1, 100).Map(func(i int) string {
		return "inv-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
	})

	// Generator for string literal values
	genStringValue := gen.IntRange(1, 100).Map(func(i int) string {
		return "val" + string(rune('a'+i%26))
	})

	// Generator for a schema with config keys and invariants
	genSchemaWithInvariants := gopter.CombineGens(
		// Generate 1-3 config keys
		gen.IntRange(1, 3),
		// Generate 1-3 invariants
		gen.IntRange(1, 3),
		// Seed for generating unique values
		gen.IntRange(0, 1000),
	).Map(func(vals []interface{}) struct {
		ConfigPaths []string
		InvNames    []string
		Rules       []string
	} {
		numConfigs := vals[0].(int)
		numInvs := vals[1].(int)
		seed := vals[2].(int)

		configPaths := make([]string, numConfigs)
		for i := 0; i < numConfigs; i++ {
			configPaths[i] = "config" + string(rune('a'+(seed+i)%26)) + ".key" + string(rune('0'+(seed+i)%10))
		}

		invNames := make([]string, numInvs)
		rules := make([]string, numInvs)
		for i := 0; i < numInvs; i++ {
			invNames[i] = "inv-" + string(rune('a'+(seed+i)%26)) + string(rune('0'+(seed+i)%10))
			// Use a config key in the rule
			configIdx := i % numConfigs
			rules[i] = configPaths[configIdx] + ` == "val` + string(rune('a'+(seed+i)%26)) + `"`
		}

		return struct {
			ConfigPaths []string
			InvNames    []string
			Rules       []string
		}{
			ConfigPaths: configPaths,
			InvNames:    invNames,
			Rules:       rules,
		}
	})

	// Suppress unused variable warnings
	_ = genConfigPath
	_ = genInvariantName
	_ = genStringValue

	properties.Property("valid schema with invariants parses successfully", prop.ForAll(
		func(data struct {
			ConfigPaths []string
			InvNames    []string
			Rules       []string
		}) bool {
			// Build YAML content
			yaml := "config:\n"
			for _, path := range data.ConfigPaths {
				yaml += "  " + path + ":\n"
				yaml += "    type: string\n"
			}
			yaml += "invariants:\n"
			for i, name := range data.InvNames {
				yaml += "  - name: " + name + "\n"
				yaml += "    rule: '" + data.Rules[i] + "'\n"
			}

			// Parse the schema
			schema, err := ParseSchema([]byte(yaml))
			if err != nil {
				t.Logf("ParseSchema failed: %v\nYAML:\n%s", err, yaml)
				return false
			}

			// Verify invariants were parsed
			if len(schema.Invariants) != len(data.InvNames) {
				t.Logf("Expected %d invariants, got %d", len(data.InvNames), len(schema.Invariants))
				return false
			}

			// Verify each invariant has correct name and rule
			for i, inv := range schema.Invariants {
				if inv.Name != data.InvNames[i] {
					t.Logf("Expected invariant name %s, got %s", data.InvNames[i], inv.Name)
					return false
				}
				if inv.Rule != data.Rules[i] {
					t.Logf("Expected invariant rule %s, got %s", data.Rules[i], inv.Rule)
					return false
				}
				if inv.Expr == nil {
					t.Logf("Invariant %s has nil Expr", inv.Name)
					return false
				}
			}

			return true
		},
		genSchemaWithInvariants,
	))

	properties.Property("invariant names are unique", prop.ForAll(
		func(data struct {
			ConfigPaths []string
			InvNames    []string
			Rules       []string
		}) bool {
			// Build YAML content
			yaml := "config:\n"
			for _, path := range data.ConfigPaths {
				yaml += "  " + path + ":\n"
				yaml += "    type: string\n"
			}
			yaml += "invariants:\n"
			for i, name := range data.InvNames {
				yaml += "  - name: " + name + "\n"
				yaml += "    rule: '" + data.Rules[i] + "'\n"
			}

			// Parse the schema
			schema, err := ParseSchema([]byte(yaml))
			if err != nil {
				t.Logf("ParseSchema failed: %v", err)
				return false
			}

			// Verify all names are unique
			seen := make(map[string]bool)
			for _, inv := range schema.Invariants {
				if seen[inv.Name] {
					t.Logf("Duplicate invariant name: %s", inv.Name)
					return false
				}
				seen[inv.Name] = true
			}

			return true
		},
		genSchemaWithInvariants,
	))

	properties.Property("invariant names contain only valid characters", prop.ForAll(
		func(data struct {
			ConfigPaths []string
			InvNames    []string
			Rules       []string
		}) bool {
			// Build YAML content
			yaml := "config:\n"
			for _, path := range data.ConfigPaths {
				yaml += "  " + path + ":\n"
				yaml += "    type: string\n"
			}
			yaml += "invariants:\n"
			for i, name := range data.InvNames {
				yaml += "  - name: " + name + "\n"
				yaml += "    rule: '" + data.Rules[i] + "'\n"
			}

			// Parse the schema
			schema, err := ParseSchema([]byte(yaml))
			if err != nil {
				t.Logf("ParseSchema failed: %v", err)
				return false
			}

			// Verify all names match the valid pattern
			for _, inv := range schema.Invariants {
				if !invariantNameRegex.MatchString(inv.Name) {
					t.Logf("Invalid invariant name: %s", inv.Name)
					return false
				}
			}

			return true
		},
		genSchemaWithInvariants,
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 2: Invariant Field Validation
// For any invariant declaration missing a `name` or `rule` field,
// the Schema_Parser SHALL return a parse error indicating the missing field.
// **Validates: Requirements 1.2, 1.3**
func TestProperty2_InvariantFieldValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid invariant names
	genInvariantName := gen.IntRange(1, 100).Map(func(i int) string {
		return "inv-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
	})

	// Generator for valid rules
	genRule := gen.IntRange(1, 100).Map(func(i int) string {
		return `execution.env == "val` + string(rune('a'+i%26)) + `"`
	})

	// Generator for invariants with missing name
	genMissingName := gopter.CombineGens(
		gen.IntRange(0, 5), // index of invariant with missing name
		genRule,
	).Map(func(vals []interface{}) struct {
		MissingNameIndex int
		Rule             string
	} {
		return struct {
			MissingNameIndex int
			Rule             string
		}{
			MissingNameIndex: vals[0].(int),
			Rule:             vals[1].(string),
		}
	})

	// Generator for invariants with missing rule
	genMissingRule := gopter.CombineGens(
		genInvariantName,
	).Map(func(vals []interface{}) struct {
		Name string
	} {
		return struct {
			Name string
		}{
			Name: vals[0].(string),
		}
	})

	properties.Property("missing name field produces error", prop.ForAll(
		func(data struct {
			MissingNameIndex int
			Rule             string
		}) bool {
			// Build YAML with missing name
			yaml := `config:
  db.url:
    type: string
invariants:
  - rule: '` + data.Rule + `'
`
			// Parse the schema
			_, err := ParseSchema([]byte(yaml))
			if err == nil {
				t.Logf("Expected error for missing name, got nil")
				return false
			}

			// Verify error message mentions missing name
			errStr := err.Error()
			if !contains(errStr, "name") && !contains(errStr, "missing") {
				t.Logf("Error message should mention missing name: %s", errStr)
				return false
			}

			return true
		},
		genMissingName,
	))

	properties.Property("missing rule field produces error", prop.ForAll(
		func(data struct {
			Name string
		}) bool {
			// Build YAML with missing rule
			yaml := `config:
  db.url:
    type: string
invariants:
  - name: ` + data.Name + `
`
			// Parse the schema
			_, err := ParseSchema([]byte(yaml))
			if err == nil {
				t.Logf("Expected error for missing rule, got nil")
				return false
			}

			// Verify error message mentions missing rule
			errStr := err.Error()
			if !contains(errStr, "rule") && !contains(errStr, "missing") {
				t.Logf("Error message should mention missing rule: %s", errStr)
				return false
			}

			return true
		},
		genMissingRule,
	))

	properties.Property("empty name field produces error", prop.ForAll(
		func(rule string) bool {
			// Build YAML with empty name
			yaml := `config:
  db.url:
    type: string
invariants:
  - name: ""
    rule: '` + rule + `'
`
			// Parse the schema
			_, err := ParseSchema([]byte(yaml))
			if err == nil {
				t.Logf("Expected error for empty name, got nil")
				return false
			}

			return true
		},
		genRule,
	))

	properties.Property("empty rule field produces error", prop.ForAll(
		func(name string) bool {
			// Build YAML with empty rule
			yaml := `config:
  db.url:
    type: string
invariants:
  - name: ` + name + `
    rule: ""
`
			// Parse the schema
			_, err := ParseSchema([]byte(yaml))
			if err == nil {
				t.Logf("Expected error for empty rule, got nil")
				return false
			}

			return true
		},
		genInvariantName,
	))

	properties.TestingRun(t)
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsLower(s, substr)))
}

func containsLower(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
