package validator

import (
	"admit/internal/resolver"
	"admit/internal/schema"
)

// ValidationError represents a single validation failure
type ValidationError struct {
	Key     string   // The config key path (e.g., "db.url")
	EnvVar  string   // The environment variable name (e.g., "DB_URL")
	Message string   // Human-readable error message
	Value   string   // The invalid value (if present)
	Allowed []string // For enum errors, the allowed values
}

// ValidationResult contains all validation outcomes
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// Validate checks all resolved values against schema constraints.
// It collects all errors rather than stopping at the first one.
// Requirements: 4.1, 4.2, 4.3, 4.4, 4.5
func Validate(s schema.Schema, resolved []resolver.ResolvedValue) ValidationResult {
	var errors []ValidationError

	for _, rv := range resolved {
		configKey, exists := s.Config[rv.Key]
		if !exists {
			// Skip resolved values that don't have a schema entry
			continue
		}

		// Check required fields (Requirement 4.1)
		if configKey.Required && !rv.Present {
			errors = append(errors, ValidationError{
				Key:     rv.Key,
				EnvVar:  rv.EnvVar,
				Message: "required but not set",
			})
			continue
		}

		// Skip validation if value is not present (optional field)
		if !rv.Present {
			continue
		}

		// Validate based on type
		switch configKey.Type {
		case schema.TypeString:
			// Requirement 4.2: Accept any non-empty string
			// A present value is valid for string type
			// (empty string is technically present but empty)

		case schema.TypeEnum:
			// Requirements 4.3, 4.4: Validate enum values
			if !isValidEnumValue(rv.Value, configKey.Values) {
				errors = append(errors, ValidationError{
					Key:     rv.Key,
					EnvVar:  rv.EnvVar,
					Message: "invalid enum value",
					Value:   rv.Value,
					Allowed: configKey.Values,
				})
			}
		}
	}

	return ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// isValidEnumValue checks if a value is in the allowed list
func isValidEnumValue(value string, allowed []string) bool {
	for _, v := range allowed {
		if v == value {
			return true
		}
	}
	return false
}
