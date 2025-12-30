package validator

import (
	"testing"

	"admit/internal/resolver"
	"admit/internal/schema"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-cli, Property 5: Required Field Validation
// Validates: Requirements 4.1
// For any schema with required fields and any environment missing those values,
// validation SHALL fail with an error for each missing required field.
func TestValidate_RequiredFieldValidation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: missing required fields produce validation errors
	properties.Property("missing required fields produce errors", prop.ForAll(
		func(numRequired int) bool {
			if numRequired < 1 {
				numRequired = 1
			}
			if numRequired > 10 {
				numRequired = 10
			}

			// Create schema with N required fields
			s := schema.Schema{
				Config: make(map[string]schema.ConfigKey),
			}
			var resolved []resolver.ResolvedValue

			for i := 0; i < numRequired; i++ {
				key := genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeString,
					Required: true,
				}
				// All values are missing (Present: false)
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "",
					Present: false,
				})
			}

			result := Validate(s, resolved)

			// Should be invalid with exactly N errors
			return !result.Valid && len(result.Errors) == numRequired
		},
		gen.IntRange(1, 10),
	))

	// Property: present required fields do not produce errors
	properties.Property("present required fields pass validation", prop.ForAll(
		func(numRequired int) bool {
			if numRequired < 1 {
				numRequired = 1
			}
			if numRequired > 10 {
				numRequired = 10
			}

			// Create schema with N required fields
			s := schema.Schema{
				Config: make(map[string]schema.ConfigKey),
			}
			var resolved []resolver.ResolvedValue

			for i := 0; i < numRequired; i++ {
				key := genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeString,
					Required: true,
				}
				// All values are present
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "some_value",
					Present: true,
				})
			}

			result := Validate(s, resolved)

			// Should be valid with no errors
			return result.Valid && len(result.Errors) == 0
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// genConfigKey generates a config key name for testing
func genConfigKey(i int) string {
	return string(rune('a'+i)) + ".config"
}


// Feature: admit-cli, Property 6: String Type Acceptance
// Validates: Requirements 4.2
// For any config key with type: string and any non-empty string value,
// validation SHALL accept the value.
func TestValidate_StringTypeAcceptance_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: any non-empty string is accepted for string type
	properties.Property("non-empty strings are accepted", prop.ForAll(
		func(value string) bool {
			if value == "" {
				return true // Skip empty strings for this property
			}

			s := schema.Schema{
				Config: map[string]schema.ConfigKey{
					"test.key": {
						Path:     "test.key",
						Type:     schema.TypeString,
						Required: true,
					},
				},
			}

			resolved := []resolver.ResolvedValue{
				{
					Key:     "test.key",
					EnvVar:  "TEST_KEY",
					Value:   value,
					Present: true,
				},
			}

			result := Validate(s, resolved)
			return result.Valid && len(result.Errors) == 0
		},
		gen.AnyString(),
	))

	// Property: string type accepts any printable string
	properties.Property("string type accepts printable strings", prop.ForAll(
		func(value string) bool {
			s := schema.Schema{
				Config: map[string]schema.ConfigKey{
					"config.value": {
						Path:     "config.value",
						Type:     schema.TypeString,
						Required: false,
					},
				},
			}

			resolved := []resolver.ResolvedValue{
				{
					Key:     "config.value",
					EnvVar:  "CONFIG_VALUE",
					Value:   value,
					Present: true,
				},
			}

			result := Validate(s, resolved)
			// String type should always accept any value when present
			return result.Valid
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}


// Feature: admit-cli, Property 7: Enum Validation Correctness
// Validates: Requirements 4.3, 4.4
// For any config key with type: enum and allowed values list:
// - If the resolved value is in the allowed list, validation SHALL accept it
// - If the resolved value is not in the allowed list, validation SHALL reject it
func TestValidate_EnumValidation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: values in allowed list are accepted
	properties.Property("values in allowed list are accepted", prop.ForAll(
		func(allowedValues []string, selectedIndex int) bool {
			if len(allowedValues) == 0 {
				return true // Skip empty allowed lists
			}

			// Ensure index is valid
			idx := selectedIndex % len(allowedValues)
			if idx < 0 {
				idx = -idx
			}
			selectedValue := allowedValues[idx]

			s := schema.Schema{
				Config: map[string]schema.ConfigKey{
					"test.enum": {
						Path:     "test.enum",
						Type:     schema.TypeEnum,
						Required: true,
						Values:   allowedValues,
					},
				},
			}

			resolved := []resolver.ResolvedValue{
				{
					Key:     "test.enum",
					EnvVar:  "TEST_ENUM",
					Value:   selectedValue,
					Present: true,
				},
			}

			result := Validate(s, resolved)
			return result.Valid && len(result.Errors) == 0
		},
		gen.SliceOfN(3, gen.AlphaString()).SuchThat(func(s []string) bool {
			// Ensure non-empty strings and unique values
			seen := make(map[string]bool)
			for _, v := range s {
				if v == "" || seen[v] {
					return false
				}
				seen[v] = true
			}
			return len(s) > 0
		}),
		gen.Int(),
	))

	// Property: values not in allowed list are rejected
	properties.Property("values not in allowed list are rejected", prop.ForAll(
		func(allowedValues []string, invalidValue string) bool {
			if len(allowedValues) == 0 {
				return true // Skip empty allowed lists
			}

			// Check if invalidValue is actually not in the list
			for _, v := range allowedValues {
				if v == invalidValue {
					return true // Skip if value happens to be in list
				}
			}

			s := schema.Schema{
				Config: map[string]schema.ConfigKey{
					"test.enum": {
						Path:     "test.enum",
						Type:     schema.TypeEnum,
						Required: true,
						Values:   allowedValues,
					},
				},
			}

			resolved := []resolver.ResolvedValue{
				{
					Key:     "test.enum",
					EnvVar:  "TEST_ENUM",
					Value:   invalidValue,
					Present: true,
				},
			}

			result := Validate(s, resolved)
			// Should be invalid with exactly 1 error
			if result.Valid || len(result.Errors) != 1 {
				return false
			}
			// Error should contain the invalid value and allowed values
			err := result.Errors[0]
			return err.Value == invalidValue && len(err.Allowed) == len(allowedValues)
		},
		gen.SliceOfN(3, gen.AlphaString()).SuchThat(func(s []string) bool {
			for _, v := range s {
				if v == "" {
					return false
				}
			}
			return len(s) > 0
		}),
		gen.AnyString().SuchThat(func(s string) bool {
			return s != "" // Non-empty invalid value
		}),
	))

	properties.TestingRun(t)
}


// Feature: admit-cli, Property 8: Error Collection Completeness
// Validates: Requirements 4.5
// For any schema and environment with N validation failures,
// the validator SHALL return exactly N errors.
func TestValidate_ErrorCollectionCompleteness_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: all missing required fields are reported
	properties.Property("all missing required fields are reported", prop.ForAll(
		func(numMissing int) bool {
			if numMissing < 1 {
				numMissing = 1
			}
			if numMissing > 10 {
				numMissing = 10
			}

			s := schema.Schema{
				Config: make(map[string]schema.ConfigKey),
			}
			var resolved []resolver.ResolvedValue

			// Create N missing required fields
			for i := 0; i < numMissing; i++ {
				key := genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeString,
					Required: true,
				}
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "",
					Present: false,
				})
			}

			result := Validate(s, resolved)
			return len(result.Errors) == numMissing
		},
		gen.IntRange(1, 10),
	))

	// Property: all invalid enum values are reported
	properties.Property("all invalid enum values are reported", prop.ForAll(
		func(numInvalid int) bool {
			if numInvalid < 1 {
				numInvalid = 1
			}
			if numInvalid > 10 {
				numInvalid = 10
			}

			s := schema.Schema{
				Config: make(map[string]schema.ConfigKey),
			}
			var resolved []resolver.ResolvedValue

			// Create N enum fields with invalid values
			for i := 0; i < numInvalid; i++ {
				key := genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeEnum,
					Required: true,
					Values:   []string{"valid1", "valid2"},
				}
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "invalid_value",
					Present: true,
				})
			}

			result := Validate(s, resolved)
			return len(result.Errors) == numInvalid
		},
		gen.IntRange(1, 10),
	))

	// Property: mixed errors (missing + invalid enum) are all reported
	properties.Property("mixed errors are all reported", prop.ForAll(
		func(numMissing, numInvalidEnum int) bool {
			if numMissing < 0 {
				numMissing = 0
			}
			if numMissing > 5 {
				numMissing = 5
			}
			if numInvalidEnum < 0 {
				numInvalidEnum = 0
			}
			if numInvalidEnum > 5 {
				numInvalidEnum = 5
			}

			totalExpectedErrors := numMissing + numInvalidEnum
			if totalExpectedErrors == 0 {
				return true // Skip if no errors expected
			}

			s := schema.Schema{
				Config: make(map[string]schema.ConfigKey),
			}
			var resolved []resolver.ResolvedValue

			// Create missing required fields
			for i := 0; i < numMissing; i++ {
				key := "missing" + genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeString,
					Required: true,
				}
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "",
					Present: false,
				})
			}

			// Create invalid enum fields
			for i := 0; i < numInvalidEnum; i++ {
				key := "enum" + genConfigKey(i)
				s.Config[key] = schema.ConfigKey{
					Path:     key,
					Type:     schema.TypeEnum,
					Required: true,
					Values:   []string{"allowed1", "allowed2"},
				}
				resolved = append(resolved, resolver.ResolvedValue{
					Key:     key,
					EnvVar:  "ENV_" + key,
					Value:   "not_allowed",
					Present: true,
				})
			}

			result := Validate(s, resolved)
			return len(result.Errors) == totalExpectedErrors
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}
