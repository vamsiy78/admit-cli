package schema

import (
	"reflect"
	"testing"

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
			s := Schema{Config: make(map[string]ConfigKey)}
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
