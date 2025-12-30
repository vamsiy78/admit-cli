package injector

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"admit/internal/artifact"
)

// genConfigArtifact generates a random ConfigArtifact
func genConfigArtifact() gopter.Gen {
	return gen.MapOf(gen.Identifier(), gen.AlphaString()).Map(func(values map[string]string) artifact.ConfigArtifact {
		return artifact.ConfigArtifact{
			ConfigVersion: artifact.ComputeConfigVersion(values),
			Values:        values,
		}
	})
}

// genEnviron generates a random environment slice
func genEnviron() gopter.Gen {
	return gen.SliceOf(
		gopter.CombineGens(
			gen.Identifier(),
			gen.AlphaString(),
		).Map(func(vals []interface{}) string {
			return vals[0].(string) + "=" + vals[1].(string)
		}),
	)
}

// TestInjectedEnvironmentContainsArtifact_Property tests Property 5: Injected Environment Contains Artifact
// Feature: admit-v1-config-artifact, Property 5: Injected Environment Contains Artifact
// For any config artifact and environment variable name, injecting the artifact SHALL result
// in an environment containing that variable with the artifact JSON as its value.
// **Validates: Requirements 3.2, 3.3**
func TestInjectedEnvironmentContainsArtifact_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("injected env contains artifact variable", prop.ForAll(
		func(art artifact.ConfigArtifact, environ []string, varName string) bool {
			if varName == "" {
				return true // Skip empty var names
			}

			result, err := InjectEnv(art, environ, varName)
			if err != nil {
				return false
			}

			// Find the variable in result
			prefix := varName + "="
			found := false
			for _, env := range result {
				if strings.HasPrefix(env, prefix) {
					found = true
					break
				}
			}

			return found
		},
		genConfigArtifact(),
		genEnviron(),
		gen.Identifier(),
	))

	properties.Property("injected value is valid JSON artifact", prop.ForAll(
		func(art artifact.ConfigArtifact, environ []string, varName string) bool {
			if varName == "" {
				return true
			}

			result, err := InjectEnv(art, environ, varName)
			if err != nil {
				return false
			}

			// Find and parse the variable
			prefix := varName + "="
			for _, env := range result {
				if strings.HasPrefix(env, prefix) {
					value := strings.TrimPrefix(env, prefix)
					var parsed artifact.ConfigArtifact
					if err := json.Unmarshal([]byte(value), &parsed); err != nil {
						return false
					}
					// Verify it matches the original
					return parsed.ConfigVersion == art.ConfigVersion
				}
			}

			return false
		},
		genConfigArtifact(),
		genEnviron(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestEnvironmentPreservation_Property tests Property 6: Environment Preservation with Injection
// Feature: admit-v1-config-artifact, Property 6: Environment Preservation with Injection
// For any original environment and injection operation, all original environment variables
// SHALL be preserved in the resulting environment.
// **Validates: Requirements 3.4**
func TestEnvironmentPreservation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("original env vars are preserved", prop.ForAll(
		func(art artifact.ConfigArtifact, environ []string, varName string) bool {
			if varName == "" {
				return true
			}

			result, err := InjectEnv(art, environ, varName)
			if err != nil {
				return false
			}

			// Build a set of result vars (excluding the injected one)
			resultSet := make(map[string]bool)
			prefix := varName + "="
			for _, env := range result {
				if !strings.HasPrefix(env, prefix) {
					resultSet[env] = true
				}
			}

			// Check all original vars (except ones with same name as injected) are present
			for _, env := range environ {
				if !strings.HasPrefix(env, prefix) {
					if !resultSet[env] {
						return false
					}
				}
			}

			return true
		},
		genConfigArtifact(),
		genEnviron(),
		gen.Identifier(),
	))

	properties.Property("result has at most one more var than original", prop.ForAll(
		func(art artifact.ConfigArtifact, environ []string, varName string) bool {
			if varName == "" {
				return true
			}

			result, err := InjectEnv(art, environ, varName)
			if err != nil {
				return false
			}

			// Count how many vars in original have the same name
			prefix := varName + "="
			existingCount := 0
			for _, env := range environ {
				if strings.HasPrefix(env, prefix) {
					existingCount++
				}
			}

			// Result should have original count - existingCount + 1 (the new one)
			expectedLen := len(environ) - existingCount + 1
			return len(result) == expectedLen
		},
		genConfigArtifact(),
		genEnviron(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}
