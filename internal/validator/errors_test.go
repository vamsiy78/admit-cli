package validator

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-cli, Property 12: Missing Required Error Message
// Validates: Requirements 6.1
// For any missing required config, the error message SHALL contain both
// the config key path and the expected environment variable name.
func TestFormatError_MissingRequired_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: missing required error message contains key and env var
	properties.Property("missing required error contains key and env var", prop.ForAll(
		func(key, envVar string) bool {
			if key == "" || envVar == "" {
				return true // Skip empty inputs
			}

			err := ValidationError{
				Key:     key,
				EnvVar:  envVar,
				Message: "required but not set",
				Value:   "",
				Allowed: nil,
			}

			formatted := FormatError(err)

			// Error message must contain the config key
			containsKey := strings.Contains(formatted, key)
			// Error message must contain the environment variable name
			containsEnvVar := strings.Contains(formatted, envVar)

			return containsKey && containsEnvVar
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: missing required error follows expected format
	properties.Property("missing required error follows format", prop.ForAll(
		func(key, envVar string) bool {
			if key == "" || envVar == "" {
				return true // Skip empty inputs
			}

			err := ValidationError{
				Key:     key,
				EnvVar:  envVar,
				Message: "required but not set",
				Value:   "",
				Allowed: nil,
			}

			formatted := FormatError(err)

			// Expected format: "{key}: required but {ENV_VAR} is not set"
			expected := key + ": required but " + envVar + " is not set"
			return formatted == expected
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// Feature: admit-cli, Property 13: Invalid Enum Error Message
// Validates: Requirements 6.2
// For any invalid enum value, the error message SHALL contain the invalid value
// and the list of allowed values.
func TestFormatError_InvalidEnum_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: invalid enum error message contains invalid value and allowed values
	properties.Property("invalid enum error contains value and allowed list", prop.ForAll(
		func(key, envVar, invalidValue string, allowed []string) bool {
			if key == "" || envVar == "" || invalidValue == "" || len(allowed) == 0 {
				return true // Skip invalid inputs
			}

			err := ValidationError{
				Key:     key,
				EnvVar:  envVar,
				Message: "invalid enum value",
				Value:   invalidValue,
				Allowed: allowed,
			}

			formatted := FormatError(err)

			// Error message must contain the config key
			containsKey := strings.Contains(formatted, key)
			// Error message must contain the invalid value
			containsValue := strings.Contains(formatted, invalidValue)
			// Error message must contain all allowed values
			containsAllAllowed := true
			for _, v := range allowed {
				if !strings.Contains(formatted, v) {
					containsAllAllowed = false
					break
				}
			}

			return containsKey && containsValue && containsAllAllowed
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.SliceOfN(3, gen.AlphaString()).SuchThat(func(s []string) bool {
			for _, v := range s {
				if v == "" {
					return false
				}
			}
			return len(s) > 0
		}),
	))

	// Property: invalid enum error follows expected format
	properties.Property("invalid enum error follows format", prop.ForAll(
		func(key, invalidValue string, allowed []string) bool {
			if key == "" || invalidValue == "" || len(allowed) == 0 {
				return true // Skip invalid inputs
			}

			err := ValidationError{
				Key:     key,
				EnvVar:  "TEST_ENV",
				Message: "invalid enum value",
				Value:   invalidValue,
				Allowed: allowed,
			}

			formatted := FormatError(err)

			// Expected format: "{key}: '{value}' is not valid, must be one of: {allowed}"
			expectedPrefix := key + ": '" + invalidValue + "' is not valid, must be one of: "
			return strings.HasPrefix(formatted, expectedPrefix)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.SliceOfN(3, gen.AlphaString()).SuchThat(func(s []string) bool {
			for _, v := range s {
				if v == "" {
					return false
				}
			}
			return len(s) > 0
		}),
	))

	properties.TestingRun(t)
}
