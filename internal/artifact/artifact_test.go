package artifact

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"admit/internal/resolver"
)

// sha256HashPattern matches a valid sha256: prefixed hex string
var sha256HashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// genConfigKey generates a valid config key (non-empty, dot-separated segments)
func genConfigKey() gopter.Gen {
	return gen.Identifier().SuchThat(func(s string) bool {
		return len(s) > 0
	})
}

// genResolvedValue generates a random ResolvedValue with a valid key
func genResolvedValue() gopter.Gen {
	return gopter.CombineGens(
		genConfigKey(),
		gen.AlphaString(),
		gen.Bool(),
	).Map(func(vals []interface{}) resolver.ResolvedValue {
		key := vals[0].(string)
		value := vals[1].(string)
		present := vals[2].(bool)
		return resolver.ResolvedValue{
			Key:     key,
			EnvVar:  strings.ToUpper(strings.ReplaceAll(key, ".", "_")),
			Value:   value,
			Present: present,
		}
	})
}

// genResolvedValuesWithUniqueKeys generates a slice of ResolvedValues with unique keys
func genResolvedValuesWithUniqueKeys() gopter.Gen {
	return gen.SliceOf(genResolvedValue()).Map(func(resolved []resolver.ResolvedValue) []resolver.ResolvedValue {
		// Deduplicate by key, keeping last occurrence
		seen := make(map[string]bool)
		result := make([]resolver.ResolvedValue, 0)
		for i := len(resolved) - 1; i >= 0; i-- {
			if !seen[resolved[i].Key] {
				seen[resolved[i].Key] = true
				result = append([]resolver.ResolvedValue{resolved[i]}, result...)
			}
		}
		return result
	})
}

// TestArtifactStructureValidity_Property tests Property 1: Artifact Structure Validity
// Feature: admit-v1-config-artifact, Property 1: Artifact Structure Validity
// For any set of resolved configuration values, the generated artifact SHALL contain
// a valid configVersion (sha256-prefixed hex string) and a values map containing
// all resolved key-value pairs.
// **Validates: Requirements 1.1, 1.2, 1.3**
func TestArtifactStructureValidity_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("artifact has valid configVersion format", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)
			return sha256HashPattern.MatchString(artifact.ConfigVersion)
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("artifact values contains all present resolved values", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)

			// Count present values
			presentCount := 0
			for _, rv := range resolved {
				if rv.Present {
					presentCount++
					// Check value is in artifact
					if val, ok := artifact.Values[rv.Key]; !ok || val != rv.Value {
						return false
					}
				}
			}

			// Artifact should have exactly the present values
			return len(artifact.Values) == presentCount
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("artifact values excludes non-present values", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)

			for _, rv := range resolved {
				if !rv.Present {
					// Non-present values should not be in artifact
					if _, ok := artifact.Values[rv.Key]; ok {
						return false
					}
				}
			}
			return true
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("artifact can be serialized to valid JSON", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)
			jsonBytes, err := artifact.ToJSON()
			if err != nil {
				return false
			}

			// Verify it's valid JSON by unmarshaling
			var parsed ConfigArtifact
			return json.Unmarshal(jsonBytes, &parsed) == nil
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.TestingRun(t)
}


// TestCanonicalJSONDeterminism_Property tests Property 2: Canonical JSON Determinism
// Feature: admit-v1-config-artifact, Property 2: Canonical JSON Determinism
// For any config artifact, serializing to canonical JSON SHALL produce output with
// sorted keys and no whitespace, and the same artifact SHALL always produce identical byte output.
// **Validates: Requirements 1.4**
func TestCanonicalJSONDeterminism_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("canonical JSON has no whitespace", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)
			canonical, err := artifact.ToCanonicalJSON()
			if err != nil {
				return false
			}

			// Check for no spaces, tabs, or newlines (except within string values)
			str := string(canonical)
			// Simple check: no space after : or , outside of strings
			// A proper check would parse, but this is a reasonable approximation
			return !strings.Contains(str, ": ") && !strings.Contains(str, ", ") &&
				!strings.Contains(str, "\n") && !strings.Contains(str, "\t")
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("canonical JSON has sorted keys", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)
			canonical, err := artifact.ToCanonicalJSON()
			if err != nil {
				return false
			}

			// Parse and verify keys are in order
			var parsed map[string]interface{}
			if err := json.Unmarshal(canonical, &parsed); err != nil {
				return false
			}

			// Check that "configVersion" comes before "values" (alphabetically)
			str := string(canonical)
			configIdx := strings.Index(str, `"configVersion"`)
			valuesIdx := strings.Index(str, `"values"`)
			return configIdx < valuesIdx
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("same artifact produces identical canonical JSON", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)

			canonical1, err1 := artifact.ToCanonicalJSON()
			canonical2, err2 := artifact.ToCanonicalJSON()

			if err1 != nil || err2 != nil {
				return false
			}

			return string(canonical1) == string(canonical2)
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("canonical JSON is valid JSON", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact := GenerateArtifact(resolved)
			canonical, err := artifact.ToCanonicalJSON()
			if err != nil {
				return false
			}

			var parsed ConfigArtifact
			return json.Unmarshal(canonical, &parsed) == nil
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.TestingRun(t)
}


// TestConfigHashIdempotence_Property tests Property 3: Config Hash Idempotence
// Feature: admit-v1-config-artifact, Property 3: Config Hash Idempotence
// For any set of config values, generating the artifact multiple times SHALL produce
// the same configVersion hash.
// **Validates: Requirements 1.5**
func TestConfigHashIdempotence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("same resolved values produce same configVersion", prop.ForAll(
		func(resolved []resolver.ResolvedValue) bool {
			artifact1 := GenerateArtifact(resolved)
			artifact2 := GenerateArtifact(resolved)

			return artifact1.ConfigVersion == artifact2.ConfigVersion
		},
		genResolvedValuesWithUniqueKeys(),
	))

	properties.Property("ComputeConfigVersion is idempotent", prop.ForAll(
		func(values map[string]string) bool {
			hash1 := ComputeConfigVersion(values)
			hash2 := ComputeConfigVersion(values)

			return hash1 == hash2
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
	))

	properties.TestingRun(t)
}


// TestConfigHashUniqueness_Property tests Property 4: Config Hash Uniqueness
// Feature: admit-v1-config-artifact, Property 4: Config Hash Uniqueness
// For any two sets of config values that differ in at least one value, the generated
// configVersion hashes SHALL be different.
// **Validates: Requirements 1.6**
func TestConfigHashUniqueness_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("different values produce different hashes", prop.ForAll(
		func(values map[string]string, key string, value1 string, value2 string) bool {
			// Skip if values are the same
			if value1 == value2 {
				return true
			}

			// Create two maps that differ by one value
			values1 := make(map[string]string)
			values2 := make(map[string]string)
			for k, v := range values {
				values1[k] = v
				values2[k] = v
			}
			values1[key] = value1
			values2[key] = value2

			hash1 := ComputeConfigVersion(values1)
			hash2 := ComputeConfigVersion(values2)

			return hash1 != hash2
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("adding a key changes the hash", prop.ForAll(
		func(values map[string]string, newKey string, newValue string) bool {
			// Skip if key already exists
			if _, exists := values[newKey]; exists {
				return true
			}

			hash1 := ComputeConfigVersion(values)

			values2 := make(map[string]string)
			for k, v := range values {
				values2[k] = v
			}
			values2[newKey] = newValue

			hash2 := ComputeConfigVersion(values2)

			return hash1 != hash2
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
		gen.Identifier(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
