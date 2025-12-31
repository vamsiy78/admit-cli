package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-v7-environment-contracts, Property 10: CLI Output Contains Required Fields
// For any violation, CLI format output SHALL contain the key name, actual value,
// rule type, and expected/forbidden values.
// **Validates: Requirements 3.3**
func TestProperty10_CLIOutputContainsRequiredFields(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Generator for non-empty list of values
	genValues := gen.SliceOfN(3, genSimpleString).Map(func(vals []string) []string {
		result := []string{}
		for _, v := range vals {
			if len(v) > 0 {
				result = append(result, v)
			}
		}
		if len(result) == 0 {
			return []string{"expected1", "expected2"}
		}
		return result
	})

	// Property: CLI output for allow violation contains all required fields
	properties.Property("CLI output for allow violation contains required fields", prop.ForAll(
		func(env string, key string, actualValue string, expectedValues []string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(expectedValues) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "allow",
						ExpectedValues: expectedValues,
					},
				},
			}

			output := FormatCLI(result)

			// Check that output contains key
			if !strings.Contains(output, key) {
				t.Logf("CLI output should contain key %q", key)
				return false
			}
			// Check that output contains actual value
			if !strings.Contains(output, actualValue) {
				t.Logf("CLI output should contain actual value %q", actualValue)
				return false
			}
			// Check that output contains rule type
			if !strings.Contains(output, "allow") {
				t.Logf("CLI output should contain rule type 'allow'")
				return false
			}
			// Check that output contains at least one expected value
			foundExpected := false
			for _, ev := range expectedValues {
				if strings.Contains(output, ev) {
					foundExpected = true
					break
				}
			}
			if !foundExpected {
				t.Logf("CLI output should contain at least one expected value")
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genValues,
	))

	// Property: CLI output for deny violation contains all required fields
	properties.Property("CLI output for deny violation contains required fields", prop.ForAll(
		func(env string, key string, actualValue string, pattern string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(pattern) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "deny",
						ExpectedValues: []string{pattern},
						Pattern:        pattern,
					},
				},
			}

			output := FormatCLI(result)

			// Check that output contains key
			if !strings.Contains(output, key) {
				t.Logf("CLI output should contain key %q", key)
				return false
			}
			// Check that output contains actual value
			if !strings.Contains(output, actualValue) {
				t.Logf("CLI output should contain actual value %q", actualValue)
				return false
			}
			// Check that output contains rule type
			if !strings.Contains(output, "deny") {
				t.Logf("CLI output should contain rule type 'deny'")
				return false
			}
			// Check that output contains pattern
			if !strings.Contains(output, pattern) {
				t.Logf("CLI output should contain pattern %q", pattern)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	// Property: CLI output for passed result is empty
	properties.Property("CLI output for passed result is empty", prop.ForAll(
		func(env string) bool {
			if len(env) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      true,
				Violations:  []Violation{},
			}

			output := FormatCLI(result)

			if output != "" {
				t.Logf("CLI output for passed result should be empty, got %q", output)
				return false
			}
			return true
		},
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 11: CI Output Format
// For any violation, CI format output SHALL contain `::error::` GitHub Actions
// annotation with violation details.
// **Validates: Requirements 3.4**
func TestProperty11_CIOutputFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Generator for non-empty list of values
	genValues := gen.SliceOfN(3, genSimpleString).Map(func(vals []string) []string {
		result := []string{}
		for _, v := range vals {
			if len(v) > 0 {
				result = append(result, v)
			}
		}
		if len(result) == 0 {
			return []string{"expected1", "expected2"}
		}
		return result
	})

	// Property: CI output contains ::error:: annotation for allow violation
	properties.Property("CI output contains ::error:: for allow violation", prop.ForAll(
		func(env string, key string, actualValue string, expectedValues []string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(expectedValues) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "allow",
						ExpectedValues: expectedValues,
					},
				},
			}

			output := FormatCI(result)

			// Check that output contains ::error:: annotation
			if !strings.Contains(output, "::error") {
				t.Logf("CI output should contain ::error:: annotation")
				return false
			}
			// Check that output contains key
			if !strings.Contains(output, key) {
				t.Logf("CI output should contain key %q", key)
				return false
			}
			// Check that output contains actual value
			if !strings.Contains(output, actualValue) {
				t.Logf("CI output should contain actual value %q", actualValue)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genValues,
	))

	// Property: CI output contains ::error:: annotation for deny violation
	properties.Property("CI output contains ::error:: for deny violation", prop.ForAll(
		func(env string, key string, actualValue string, pattern string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(pattern) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "deny",
						ExpectedValues: []string{pattern},
						Pattern:        pattern,
					},
				},
			}

			output := FormatCI(result)

			// Check that output contains ::error:: annotation
			if !strings.Contains(output, "::error") {
				t.Logf("CI output should contain ::error:: annotation")
				return false
			}
			// Check that output contains key
			if !strings.Contains(output, key) {
				t.Logf("CI output should contain key %q", key)
				return false
			}
			// Check that output contains actual value
			if !strings.Contains(output, actualValue) {
				t.Logf("CI output should contain actual value %q", actualValue)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	// Property: CI output for passed result is empty
	properties.Property("CI output for passed result is empty", prop.ForAll(
		func(env string) bool {
			if len(env) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      true,
				Violations:  []Violation{},
			}

			output := FormatCI(result)

			if output != "" {
				t.Logf("CI output for passed result should be empty, got %q", output)
				return false
			}
			return true
		},
		genSimpleString,
	))

	// Property: Each violation gets its own ::error:: line
	properties.Property("each violation gets its own ::error:: line", prop.ForAll(
		func(env string, numViolations int) bool {
			if len(env) == 0 || numViolations < 1 {
				return true
			}

			violations := make([]Violation, numViolations)
			for i := 0; i < numViolations; i++ {
				violations[i] = Violation{
					Key:            "key" + string(rune('a'+i)),
					ActualValue:    "value" + string(rune('a'+i)),
					RuleType:       "allow",
					ExpectedValues: []string{"expected"},
				}
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations:  violations,
			}

			output := FormatCI(result)

			// Count ::error:: occurrences
			errorCount := strings.Count(output, "::error")
			if errorCount != numViolations {
				t.Logf("Expected %d ::error:: annotations, got %d", numViolations, errorCount)
				return false
			}
			return true
		},
		genSimpleString,
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 12: JSON Output Validity
// For any evaluation result, JSON format output SHALL be valid parseable JSON
// containing environment, passed status, and violations array.
// **Validates: Requirements 3.5**
func TestProperty12_JSONOutputValidity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Generator for non-empty list of values
	genValues := gen.SliceOfN(3, genSimpleString).Map(func(vals []string) []string {
		result := []string{}
		for _, v := range vals {
			if len(v) > 0 {
				result = append(result, v)
			}
		}
		if len(result) == 0 {
			return []string{"expected1", "expected2"}
		}
		return result
	})

	// Property: JSON output is valid parseable JSON for passed result
	properties.Property("JSON output is valid for passed result", prop.ForAll(
		func(env string) bool {
			if len(env) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      true,
				Violations:  []Violation{},
			}

			output, err := FormatJSON(result)
			if err != nil {
				t.Logf("FormatJSON should not return error, got %v", err)
				return false
			}

			// Verify it's valid JSON
			var parsed EvalResult
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Logf("JSON output should be parseable, got error: %v", err)
				return false
			}

			// Verify fields are preserved
			if parsed.Environment != env {
				t.Logf("Environment should be %q, got %q", env, parsed.Environment)
				return false
			}
			if !parsed.Passed {
				t.Logf("Passed should be true")
				return false
			}
			if parsed.Violations == nil {
				t.Logf("Violations should not be nil")
				return false
			}
			return true
		},
		genSimpleString,
	))

	// Property: JSON output is valid parseable JSON for failed result with violations
	properties.Property("JSON output is valid for failed result with violations", prop.ForAll(
		func(env string, key string, actualValue string, expectedValues []string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(expectedValues) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "allow",
						ExpectedValues: expectedValues,
					},
				},
			}

			output, err := FormatJSON(result)
			if err != nil {
				t.Logf("FormatJSON should not return error, got %v", err)
				return false
			}

			// Verify it's valid JSON
			var parsed EvalResult
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Logf("JSON output should be parseable, got error: %v", err)
				return false
			}

			// Verify fields are preserved
			if parsed.Environment != env {
				t.Logf("Environment should be %q, got %q", env, parsed.Environment)
				return false
			}
			if parsed.Passed {
				t.Logf("Passed should be false")
				return false
			}
			if len(parsed.Violations) != 1 {
				t.Logf("Should have 1 violation, got %d", len(parsed.Violations))
				return false
			}
			if parsed.Violations[0].Key != key {
				t.Logf("Violation key should be %q, got %q", key, parsed.Violations[0].Key)
				return false
			}
			if parsed.Violations[0].ActualValue != actualValue {
				t.Logf("Violation actualValue should be %q, got %q", actualValue, parsed.Violations[0].ActualValue)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genValues,
	))

	// Property: JSON output preserves multiple violations
	properties.Property("JSON output preserves multiple violations", prop.ForAll(
		func(env string, numViolations int) bool {
			if len(env) == 0 || numViolations < 1 {
				return true
			}

			violations := make([]Violation, numViolations)
			for i := 0; i < numViolations; i++ {
				violations[i] = Violation{
					Key:            "key" + string(rune('a'+i)),
					ActualValue:    "value" + string(rune('a'+i)),
					RuleType:       "allow",
					ExpectedValues: []string{"expected"},
				}
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations:  violations,
			}

			output, err := FormatJSON(result)
			if err != nil {
				t.Logf("FormatJSON should not return error, got %v", err)
				return false
			}

			// Verify it's valid JSON
			var parsed EvalResult
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Logf("JSON output should be parseable, got error: %v", err)
				return false
			}

			// Verify all violations are preserved
			if len(parsed.Violations) != numViolations {
				t.Logf("Should have %d violations, got %d", numViolations, len(parsed.Violations))
				return false
			}
			return true
		},
		genSimpleString,
		gen.IntRange(1, 5),
	))

	// Property: JSON output for deny violation includes pattern
	properties.Property("JSON output for deny violation includes pattern", prop.ForAll(
		func(env string, key string, actualValue string, pattern string) bool {
			if len(env) == 0 || len(key) == 0 || len(actualValue) == 0 || len(pattern) == 0 {
				return true
			}

			result := EvalResult{
				Environment: env,
				Passed:      false,
				Violations: []Violation{
					{
						Key:            key,
						ActualValue:    actualValue,
						RuleType:       "deny",
						ExpectedValues: []string{pattern},
						Pattern:        pattern,
					},
				},
			}

			output, err := FormatJSON(result)
			if err != nil {
				t.Logf("FormatJSON should not return error, got %v", err)
				return false
			}

			// Verify it's valid JSON
			var parsed EvalResult
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Logf("JSON output should be parseable, got error: %v", err)
				return false
			}

			// Verify pattern is preserved
			if parsed.Violations[0].Pattern != pattern {
				t.Logf("Violation pattern should be %q, got %q", pattern, parsed.Violations[0].Pattern)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	properties.TestingRun(t)
}
