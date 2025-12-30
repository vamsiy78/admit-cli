package validator

import (
	"fmt"
	"strings"
)

// FormatError formats a ValidationError into a human-readable error message.
// Requirements: 6.1, 6.2
func FormatError(err ValidationError) string {
	// Check if this is a missing required error (no value, no allowed list)
	if err.Value == "" && len(err.Allowed) == 0 {
		// Requirement 6.1: Missing required error format
		// Format: "{key}: required but {ENV_VAR} is not set"
		return fmt.Sprintf("%s: required but %s is not set", err.Key, err.EnvVar)
	}

	// Check if this is an invalid enum error (has value and allowed list)
	if len(err.Allowed) > 0 {
		// Requirement 6.2: Invalid enum error format
		// Format: "{key}: '{value}' is not valid, must be one of: {allowed}"
		return fmt.Sprintf("%s: '%s' is not valid, must be one of: %s",
			err.Key, err.Value, strings.Join(err.Allowed, ", "))
	}

	// Fallback to generic message
	return fmt.Sprintf("%s: %s", err.Key, err.Message)
}

// FormatErrors formats all validation errors into a slice of human-readable messages.
func FormatErrors(result ValidationResult) []string {
	messages := make([]string, len(result.Errors))
	for i, err := range result.Errors {
		messages[i] = FormatError(err)
	}
	return messages
}
