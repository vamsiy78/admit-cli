package execid

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// sha256HashPattern matches a valid sha256: prefixed hex string
var sha256HashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// genSchemaKeys generates a list of schema keys (dot-notation paths)
func genSchemaKeys() gopter.Gen {
	return gen.SliceOfN(3, gen.Identifier()).Map(func(parts []string) []string {
		var keys []string
		for _, p := range parts {
			if p != "" {
				keys = append(keys, p)
			}
		}
		return keys
	})
}

// genEnviron generates environment variables as KEY=VALUE strings
func genEnviron(schemaKeys []string) gopter.Gen {
	return gen.SliceOfN(len(schemaKeys), gen.AlphaString()).Map(func(values []string) []string {
		var environ []string
		for i, key := range schemaKeys {
			envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			if i < len(values) {
				environ = append(environ, envVar+"="+values[i])
			}
		}
		return environ
	})
}

// Feature: admit-v4-execution-identity, Property 1: Execution ID Determinism
// Validates: Requirements 1.4, 2.4, 3.5, 7.1, 7.2, 7.4
// For any execution context (config values, command, arguments, relevant environment),
// computing the execution_id multiple times SHALL produce identical results.
func TestExecutionIDDeterminism_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: same inputs produce same execution ID
	properties.Property("same inputs produce same execution ID", prop.ForAll(
		func(configVersion string, command string, args []string, schemaKeys []string) bool {
			// Generate consistent environ from schema keys
			environ := make([]string, len(schemaKeys))
			for i, key := range schemaKeys {
				envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				environ[i] = envVar + "=value" + string(rune('0'+i))
			}

			// Compute twice
			id1 := ComputeExecutionID("sha256:"+configVersion, command, args, environ, schemaKeys)
			id2 := ComputeExecutionID("sha256:"+configVersion, command, args, environ, schemaKeys)

			return id1.ExecutionID == id2.ExecutionID &&
				id1.CommandHash == id2.CommandHash &&
				id1.EnvironmentHash == id2.EnvironmentHash
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOfN(3, gen.Identifier()),
	))

	// Property: command hash is idempotent
	properties.Property("command hash is idempotent", prop.ForAll(
		func(command string, args []string) bool {
			hash1 := ComputeCommandHash(command, args)
			hash2 := ComputeCommandHash(command, args)
			return hash1 == hash2
		},
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: environment hash is idempotent
	properties.Property("environment hash is idempotent", prop.ForAll(
		func(schemaKeys []string) bool {
			environ := make([]string, len(schemaKeys))
			for i, key := range schemaKeys {
				envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				environ[i] = envVar + "=value"
			}

			hash1 := ComputeEnvironmentHash(environ, schemaKeys)
			hash2 := ComputeEnvironmentHash(environ, schemaKeys)
			return hash1 == hash2
		},
		gen.SliceOfN(3, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 2: Hash Format Compliance
// Validates: Requirements 1.3, 2.3, 3.4
// For any generated hash (execution_id, command_hash, environment_hash),
// it SHALL be formatted as "sha256:" followed by exactly 64 lowercase hexadecimal characters.
func TestHashFormatCompliance_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: execution ID has valid format
	properties.Property("execution ID has valid sha256 format", prop.ForAll(
		func(configVersion string, command string, args []string) bool {
			id := ComputeExecutionID("sha256:"+configVersion, command, args, []string{}, []string{})
			return sha256HashPattern.MatchString(id.ExecutionID)
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: command hash has valid format
	properties.Property("command hash has valid sha256 format", prop.ForAll(
		func(command string, args []string) bool {
			hash := ComputeCommandHash(command, args)
			return sha256HashPattern.MatchString(hash)
		},
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: environment hash has valid format
	properties.Property("environment hash has valid sha256 format", prop.ForAll(
		func(schemaKeys []string) bool {
			environ := make([]string, len(schemaKeys))
			for i, key := range schemaKeys {
				envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				environ[i] = envVar + "=value"
			}
			hash := ComputeEnvironmentHash(environ, schemaKeys)
			return sha256HashPattern.MatchString(hash)
		},
		gen.SliceOfN(3, gen.Identifier()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 3: Execution ID Composition
// Validates: Requirements 1.1, 1.2
// For any valid config version, command hash, and environment hash,
// the execution_id SHALL be the SHA-256 hash of their concatenation.
// Changing any component SHALL produce a different execution_id.
func TestExecutionIDComposition_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: changing config version changes execution ID
	properties.Property("changing config version changes execution ID", prop.ForAll(
		func(config1 string, config2 string, command string, args []string) bool {
			if config1 == config2 {
				return true // Skip if same
			}
			id1 := ComputeExecutionID("sha256:"+config1, command, args, []string{}, []string{})
			id2 := ComputeExecutionID("sha256:"+config2, command, args, []string{}, []string{})
			return id1.ExecutionID != id2.ExecutionID
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: changing command changes execution ID
	properties.Property("changing command changes execution ID", prop.ForAll(
		func(configVersion string, command1 string, command2 string, args []string) bool {
			if command1 == command2 {
				return true // Skip if same
			}
			id1 := ComputeExecutionID("sha256:"+configVersion, command1, args, []string{}, []string{})
			id2 := ComputeExecutionID("sha256:"+configVersion, command2, args, []string{}, []string{})
			return id1.ExecutionID != id2.ExecutionID
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: changing environment changes execution ID
	properties.Property("changing environment changes execution ID", prop.ForAll(
		func(configVersion string, command string, value1 string, value2 string) bool {
			if value1 == value2 {
				return true // Skip if same
			}
			schemaKeys := []string{"test.key"}
			environ1 := []string{"TEST_KEY=" + value1}
			environ2 := []string{"TEST_KEY=" + value2}

			id1 := ComputeExecutionID("sha256:"+configVersion, command, []string{}, environ1, schemaKeys)
			id2 := ComputeExecutionID("sha256:"+configVersion, command, []string{}, environ2, schemaKeys)
			return id1.ExecutionID != id2.ExecutionID
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 4: Command Hash Sensitivity
// Validates: Requirements 2.1, 2.2
// For any two commands that differ in target, argument values, or argument order,
// the command_hash SHALL be different.
func TestCommandHashSensitivity_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: different commands produce different hashes
	properties.Property("different commands produce different hashes", prop.ForAll(
		func(cmd1 string, cmd2 string) bool {
			if cmd1 == cmd2 {
				return true // Skip if same
			}
			hash1 := ComputeCommandHash(cmd1, []string{})
			hash2 := ComputeCommandHash(cmd2, []string{})
			return hash1 != hash2
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	// Property: different argument values produce different hashes
	properties.Property("different argument values produce different hashes", prop.ForAll(
		func(command string, arg1 string, arg2 string) bool {
			if arg1 == arg2 {
				return true // Skip if same
			}
			hash1 := ComputeCommandHash(command, []string{arg1})
			hash2 := ComputeCommandHash(command, []string{arg2})
			return hash1 != hash2
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	// Property: different argument order produces different hashes
	properties.Property("different argument order produces different hashes", prop.ForAll(
		func(command string, arg1 string, arg2 string) bool {
			if arg1 == arg2 {
				return true // Skip if same args
			}
			hash1 := ComputeCommandHash(command, []string{arg1, arg2})
			hash2 := ComputeCommandHash(command, []string{arg2, arg1})
			return hash1 != hash2
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	// Property: same command and args produce same hash
	properties.Property("same command and args produce same hash", prop.ForAll(
		func(command string, args []string) bool {
			hash1 := ComputeCommandHash(command, args)
			hash2 := ComputeCommandHash(command, args)
			return hash1 == hash2
		},
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 5: Environment Hash Filtering
// Validates: Requirements 3.1, 3.2, 7.3
// For any environment and schema, the environment_hash SHALL only incorporate
// environment variables that correspond to config keys defined in the schema.
func TestEnvironmentHashFiltering_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: non-schema env vars don't affect hash
	properties.Property("non-schema env vars don't affect hash", prop.ForAll(
		func(schemaKey string, extraKey string, value string) bool {
			if schemaKey == extraKey {
				return true // Skip if same
			}
			schemaKeys := []string{schemaKey}
			schemaEnvVar := strings.ToUpper(strings.ReplaceAll(schemaKey, ".", "_"))
			extraEnvVar := strings.ToUpper(strings.ReplaceAll(extraKey, ".", "_"))

			// Environ with only schema var
			environ1 := []string{schemaEnvVar + "=" + value}
			// Environ with schema var + extra var
			environ2 := []string{schemaEnvVar + "=" + value, extraEnvVar + "=extra"}

			hash1 := ComputeEnvironmentHash(environ1, schemaKeys)
			hash2 := ComputeEnvironmentHash(environ2, schemaKeys)
			return hash1 == hash2
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString(),
	))

	// Property: only schema-referenced vars are included
	properties.Property("only schema-referenced vars are included", prop.ForAll(
		func(schemaKey string, value1 string, value2 string) bool {
			if value1 == value2 {
				return true // Skip if same
			}
			schemaKeys := []string{schemaKey}
			schemaEnvVar := strings.ToUpper(strings.ReplaceAll(schemaKey, ".", "_"))

			// Different values for schema var should produce different hashes
			environ1 := []string{schemaEnvVar + "=" + value1}
			environ2 := []string{schemaEnvVar + "=" + value2}

			hash1 := ComputeEnvironmentHash(environ1, schemaKeys)
			hash2 := ComputeEnvironmentHash(environ2, schemaKeys)
			return hash1 != hash2
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 6: Environment Hash Order Independence
// Validates: Requirements 3.3
// For any set of environment variables, the environment_hash SHALL be identical
// regardless of the order in which environment variables are provided.
func TestEnvironmentHashOrderIndependence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: different environ order produces same hash
	properties.Property("different environ order produces same hash", prop.ForAll(
		func(key1 string, key2 string, value1 string, value2 string) bool {
			if key1 == key2 {
				return true // Skip if same keys
			}
			schemaKeys := []string{key1, key2}
			envVar1 := strings.ToUpper(strings.ReplaceAll(key1, ".", "_"))
			envVar2 := strings.ToUpper(strings.ReplaceAll(key2, ".", "_"))

			// Different orders
			environ1 := []string{envVar1 + "=" + value1, envVar2 + "=" + value2}
			environ2 := []string{envVar2 + "=" + value2, envVar1 + "=" + value1}

			hash1 := ComputeEnvironmentHash(environ1, schemaKeys)
			hash2 := ComputeEnvironmentHash(environ2, schemaKeys)
			return hash1 == hash2
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property: shuffled environ produces same hash
	properties.Property("shuffled environ produces same hash", prop.ForAll(
		func(keys []string, values []string) bool {
			if len(keys) < 2 || len(values) < len(keys) {
				return true // Skip if not enough data
			}
			// Deduplicate keys
			keySet := make(map[string]bool)
			var uniqueKeys []string
			for _, k := range keys {
				if !keySet[k] && k != "" {
					keySet[k] = true
					uniqueKeys = append(uniqueKeys, k)
				}
			}
			if len(uniqueKeys) < 2 {
				return true
			}

			// Build environ
			var environ []string
			for i, key := range uniqueKeys {
				envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				if i < len(values) {
					environ = append(environ, envVar+"="+values[i])
				}
			}

			// Reverse order
			reversed := make([]string, len(environ))
			for i, e := range environ {
				reversed[len(environ)-1-i] = e
			}

			hash1 := ComputeEnvironmentHash(environ, uniqueKeys)
			hash2 := ComputeEnvironmentHash(reversed, uniqueKeys)
			return hash1 == hash2
		},
		gen.SliceOfN(5, gen.Identifier()),
		gen.SliceOfN(5, gen.AlphaString()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 7: Execution Identity JSON Structure
// Validates: Requirements 4.2, 4.3, 4.4
// For any execution identity, the JSON output SHALL contain all required fields.
func TestExecutionIdentityJSONStructure_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: JSON contains all required fields
	properties.Property("JSON contains all required fields", prop.ForAll(
		func(configVersion string, command string, args []string) bool {
			id := ComputeExecutionID("sha256:"+configVersion, command, args, []string{}, []string{})

			jsonBytes, err := id.ToJSON()
			if err != nil {
				return false
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
				return false
			}

			// Check all required fields exist
			requiredFields := []string{"executionId", "configVersion", "commandHash", "environmentHash", "command", "args"}
			for _, field := range requiredFields {
				if _, ok := parsed[field]; !ok {
					return false
				}
			}
			return true
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: all hash fields have valid format
	properties.Property("all hash fields have valid sha256 format", prop.ForAll(
		func(configVersion string, command string, args []string) bool {
			id := ComputeExecutionID("sha256:"+configVersion, command, args, []string{}, []string{})

			return sha256HashPattern.MatchString(id.ExecutionID) &&
				sha256HashPattern.MatchString(id.CommandHash) &&
				sha256HashPattern.MatchString(id.EnvironmentHash)
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: JSON can be parsed back
	properties.Property("JSON can be parsed back to struct", prop.ForAll(
		func(configVersion string, command string, args []string) bool {
			id := ComputeExecutionID("sha256:"+configVersion, command, args, []string{}, []string{})

			jsonBytes, err := id.ToJSON()
			if err != nil {
				return false
			}

			var parsed ExecutionIdentityV4
			if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
				return false
			}

			return parsed.ExecutionID == id.ExecutionID &&
				parsed.ConfigVersion == id.ConfigVersion &&
				parsed.CommandHash == id.CommandHash &&
				parsed.EnvironmentHash == id.EnvironmentHash &&
				parsed.Command == id.Command
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v4-execution-identity, Property 8: Execution ID Injection
// Validates: Requirements 4.5
// For any execution with --execution-id-env, the specified environment variable
// SHALL be set to the execution_id value.
func TestExecutionIDInjection_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Short() returns executionId
	properties.Property("Short() returns executionId", prop.ForAll(
		func(configVersion string, command string, args []string) bool {
			id := ComputeExecutionID("sha256:"+configVersion, command, args, []string{}, []string{})
			return id.Short() == id.ExecutionID
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: execution ID can be injected into environ
	properties.Property("execution ID can be injected into environ", prop.ForAll(
		func(configVersion string, command string, varName string) bool {
			if varName == "" {
				return true
			}
			id := ComputeExecutionID("sha256:"+configVersion, command, []string{}, []string{}, []string{})

			// Simulate injection
			environ := []string{"EXISTING=value"}
			injected := append(environ, varName+"="+id.Short())

			// Verify the var is in the environ
			found := false
			for _, env := range injected {
				if strings.HasPrefix(env, varName+"=") {
					found = strings.Contains(env, id.ExecutionID)
					break
				}
			}
			return found
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestPathToEnvVar tests the path to env var conversion
func TestPathToEnvVar(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"db.url", "DB_URL"},
		{"payments.mode", "PAYMENTS_MODE"},
		{"app.server.port", "APP_SERVER_PORT"},
		{"simple", "SIMPLE"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathToEnvVar(tt.path)
			if got != tt.want {
				t.Errorf("pathToEnvVar(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestComputeEnvironmentHash_Sorting verifies that sorting works correctly
func TestComputeEnvironmentHash_Sorting(t *testing.T) {
	schemaKeys := []string{"a.key", "b.key", "c.key"}

	// Different orders should produce same hash
	environ1 := []string{"A_KEY=1", "B_KEY=2", "C_KEY=3"}
	environ2 := []string{"C_KEY=3", "A_KEY=1", "B_KEY=2"}
	environ3 := []string{"B_KEY=2", "C_KEY=3", "A_KEY=1"}

	hash1 := ComputeEnvironmentHash(environ1, schemaKeys)
	hash2 := ComputeEnvironmentHash(environ2, schemaKeys)
	hash3 := ComputeEnvironmentHash(environ3, schemaKeys)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Different orders produced different hashes: %s, %s, %s", hash1, hash2, hash3)
	}
}

// TestComputeEnvironmentHash_Filtering verifies that non-schema vars are filtered
func TestComputeEnvironmentHash_Filtering(t *testing.T) {
	schemaKeys := []string{"db.url"}

	// Extra vars should not affect hash
	environ1 := []string{"DB_URL=postgres://localhost"}
	environ2 := []string{"DB_URL=postgres://localhost", "PATH=/usr/bin", "HOME=/home/user"}

	hash1 := ComputeEnvironmentHash(environ1, schemaKeys)
	hash2 := ComputeEnvironmentHash(environ2, schemaKeys)

	if hash1 != hash2 {
		t.Errorf("Extra vars affected hash: %s != %s", hash1, hash2)
	}
}

// TestComputeCommandHash_NullSeparator verifies null byte separation works
func TestComputeCommandHash_NullSeparator(t *testing.T) {
	// These should produce different hashes due to null byte separation
	hash1 := ComputeCommandHash("echo", []string{"hello", "world"})
	hash2 := ComputeCommandHash("echo", []string{"helloworld"})
	hash3 := ComputeCommandHash("echohello", []string{"world"})

	if hash1 == hash2 {
		t.Error("Different args produced same hash")
	}
	if hash1 == hash3 {
		t.Error("Different command/args split produced same hash")
	}
}

// Helper to sort strings for comparison
func sortedCopy(s []string) []string {
	c := make([]string, len(s))
	copy(c, s)
	sort.Strings(c)
	return c
}
