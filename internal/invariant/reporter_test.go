package invariant

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestFormatViolation(t *testing.T) {
	result := InvariantResult{
		Name:       "prod-db-guard",
		Rule:       `execution.env == "prod" => db.env == "prod"`,
		Passed:     false,
		LeftValue:  "prod",
		RightValue: "staging",
		Message:    "condition 'execution.env == \"prod\"' is true but 'db.env == \"prod\"' is false",
	}

	output := FormatViolation(result)

	// Verify name is prominently displayed
	if !strings.Contains(output, "prod-db-guard") {
		t.Errorf("FormatViolation() output missing invariant name")
	}

	// Verify rule is shown
	if !strings.Contains(output, `execution.env == "prod" => db.env == "prod"`) {
		t.Errorf("FormatViolation() output missing rule expression")
	}

	// Verify values are shown
	if !strings.Contains(output, "prod") || !strings.Contains(output, "staging") {
		t.Errorf("FormatViolation() output missing evaluated values")
	}
}

func TestFormatViolations_MultipleFailures(t *testing.T) {
	results := []InvariantResult{
		{
			Name:       "inv1",
			Rule:       `a == "x"`,
			Passed:     false,
			LeftValue:  "y",
			RightValue: "x",
			Message:    "'y' != 'x'",
		},
		{
			Name:       "inv2",
			Rule:       `b == "z"`,
			Passed:     true, // This one passes, should not appear
			LeftValue:  "z",
			RightValue: "z",
		},
		{
			Name:       "inv3",
			Rule:       `c != "w"`,
			Passed:     false,
			LeftValue:  "w",
			RightValue: "w",
			Message:    "'w' == 'w'",
		},
	}

	output := FormatViolations(results)

	// Should contain both failing invariants
	if !strings.Contains(output, "inv1") {
		t.Errorf("FormatViolations() missing inv1")
	}
	if !strings.Contains(output, "inv3") {
		t.Errorf("FormatViolations() missing inv3")
	}

	// Should indicate 2 violations
	if !strings.Contains(output, "2 violation") {
		t.Errorf("FormatViolations() should indicate 2 violations")
	}
}

func TestFormatViolations_NoFailures(t *testing.T) {
	results := []InvariantResult{
		{
			Name:   "inv1",
			Rule:   `a == "x"`,
			Passed: true,
		},
	}

	output := FormatViolations(results)

	if output != "" {
		t.Errorf("FormatViolations() should return empty string when no violations, got %q", output)
	}
}

func TestFormatJSON(t *testing.T) {
	results := []InvariantResult{
		{
			Name:       "prod-db-guard",
			Rule:       `execution.env == "prod" => db.env == "prod"`,
			Passed:     false,
			LeftValue:  "prod",
			RightValue: "staging",
			Message:    "condition is true but consequent is false",
		},
		{
			Name:       "other-guard",
			Rule:       `a == "b"`,
			Passed:     true,
			LeftValue:  "b",
			RightValue: "b",
		},
	}

	jsonStr, err := FormatJSON(results)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}

	// Parse the JSON to verify structure
	var report ViolationReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		t.Fatalf("FormatJSON() produced invalid JSON: %v", err)
	}

	if len(report.Invariants) != 2 {
		t.Errorf("FormatJSON() invariants count = %d, want 2", len(report.Invariants))
	}

	if report.AllPassed {
		t.Errorf("FormatJSON() allPassed = true, want false")
	}

	if report.FailedCount != 1 {
		t.Errorf("FormatJSON() failedCount = %d, want 1", report.FailedCount)
	}

	// Verify first invariant details
	if report.Invariants[0].Name != "prod-db-guard" {
		t.Errorf("FormatJSON() first invariant name = %q, want %q", report.Invariants[0].Name, "prod-db-guard")
	}
}

func TestHasViolations(t *testing.T) {
	tests := []struct {
		name    string
		results []InvariantResult
		want    bool
	}{
		{
			name:    "empty results",
			results: []InvariantResult{},
			want:    false,
		},
		{
			name: "all pass",
			results: []InvariantResult{
				{Passed: true},
				{Passed: true},
			},
			want: false,
		},
		{
			name: "one fails",
			results: []InvariantResult{
				{Passed: true},
				{Passed: false},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasViolations(tt.results); got != tt.want {
				t.Errorf("HasViolations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetViolations(t *testing.T) {
	results := []InvariantResult{
		{Name: "pass1", Passed: true},
		{Name: "fail1", Passed: false},
		{Name: "pass2", Passed: true},
		{Name: "fail2", Passed: false},
	}

	violations := GetViolations(results)

	if len(violations) != 2 {
		t.Fatalf("GetViolations() returned %d violations, want 2", len(violations))
	}

	if violations[0].Name != "fail1" || violations[1].Name != "fail2" {
		t.Errorf("GetViolations() returned wrong violations")
	}
}


// Feature: admit-v2-invariants, Property 9: Violation Output Completeness
// For any invariant violation, the error output SHALL contain:
// - The invariant name
// - The original rule expression
// - The actual evaluated values that caused the failure
// **Validates: Requirements 4.3, 4.4, 4.5, 5.1, 5.2, 5.3**
func TestProperty9_ViolationOutputCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid invariant names (alphanumeric, hyphens, underscores)
	genInvariantName := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 20; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "test-invariant"
		}
		return string(result)
	})

	// Generator for simple string values (alphanumeric)
	genStringValue := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 15; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "value"
		}
		return string(result)
	})

	// Generator for rule expressions
	genRule := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 30; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') ||
				ch == '.' || ch == '=' || ch == '!' || ch == ' ' || ch == '"' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return `a.b == "x"`
		}
		return string(result)
	})

	// Generator for invariant violations
	genViolation := gopter.CombineGens(
		genInvariantName,
		genRule,
		genStringValue,
		genStringValue,
	).Map(func(vals []interface{}) InvariantResult {
		return InvariantResult{
			Name:       vals[0].(string),
			Rule:       vals[1].(string),
			Passed:     false, // Always a violation
			LeftValue:  vals[2].(string),
			RightValue: vals[3].(string),
			Message:    "test violation message",
		}
	})

	properties.Property("violation output contains name, rule, and values", prop.ForAll(
		func(violation InvariantResult) bool {
			output := FormatViolation(violation)

			// Check that name is present
			if !strings.Contains(output, violation.Name) {
				t.Logf("Output missing invariant name %q in: %s", violation.Name, output)
				return false
			}

			// Check that rule is present
			if !strings.Contains(output, violation.Rule) {
				t.Logf("Output missing rule %q in: %s", violation.Rule, output)
				return false
			}

			// Check that left value is present (if non-empty)
			if violation.LeftValue != "" && !strings.Contains(output, violation.LeftValue) {
				t.Logf("Output missing left value %q in: %s", violation.LeftValue, output)
				return false
			}

			// Check that right value is present (if non-empty)
			if violation.RightValue != "" && !strings.Contains(output, violation.RightValue) {
				t.Logf("Output missing right value %q in: %s", violation.RightValue, output)
				return false
			}

			return true
		},
		genViolation,
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 10: JSON Output Format
// For any invariant evaluation with --invariants-json flag, the output SHALL be
// valid JSON containing the invariant results with name, rule, passed status, and values.
// **Validates: Requirements 5.5**
func TestProperty10_JSONOutputFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid invariant names
	genInvariantName := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 20; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "test-invariant"
		}
		return string(result)
	})

	// Generator for simple string values
	genStringValue := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 15; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "value"
		}
		return string(result)
	})

	// Generator for rule expressions (avoiding special JSON chars)
	genRule := gen.AnyString().Map(func(s string) string {
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 30; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') ||
				ch == '.' || ch == '=' || ch == '!' || ch == ' ' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "a.b == x"
		}
		return string(result)
	})

	// Generator for a single invariant result
	genInvariantResult := gopter.CombineGens(
		genInvariantName,
		genRule,
		gen.Bool(),
		genStringValue,
		genStringValue,
	).Map(func(vals []interface{}) InvariantResult {
		return InvariantResult{
			Name:       vals[0].(string),
			Rule:       vals[1].(string),
			Passed:     vals[2].(bool),
			LeftValue:  vals[3].(string),
			RightValue: vals[4].(string),
			Message:    "test message",
		}
	})

	// Generator for a list of invariant results (1-5 results)
	genResultList := gen.SliceOfN(5, genInvariantResult).Map(func(results []InvariantResult) []InvariantResult {
		if len(results) == 0 {
			return []InvariantResult{{
				Name:   "default",
				Rule:   "a == b",
				Passed: true,
			}}
		}
		return results
	})

	properties.Property("JSON output is valid and contains required fields", prop.ForAll(
		func(results []InvariantResult) bool {
			jsonStr, err := FormatJSON(results)
			if err != nil {
				t.Logf("FormatJSON error: %v", err)
				return false
			}

			// Parse the JSON to verify it's valid
			var report ViolationReport
			if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
				t.Logf("Invalid JSON: %v\nJSON: %s", err, jsonStr)
				return false
			}

			// Verify invariants count matches
			if len(report.Invariants) != len(results) {
				t.Logf("Invariants count mismatch: got %d, want %d", len(report.Invariants), len(results))
				return false
			}

			// Verify each invariant has required fields
			for i, inv := range report.Invariants {
				if inv.Name != results[i].Name {
					t.Logf("Name mismatch at %d: got %q, want %q", i, inv.Name, results[i].Name)
					return false
				}
				if inv.Rule != results[i].Rule {
					t.Logf("Rule mismatch at %d: got %q, want %q", i, inv.Rule, results[i].Rule)
					return false
				}
				if inv.Passed != results[i].Passed {
					t.Logf("Passed mismatch at %d: got %v, want %v", i, inv.Passed, results[i].Passed)
					return false
				}
				if inv.LeftValue != results[i].LeftValue {
					t.Logf("LeftValue mismatch at %d: got %q, want %q", i, inv.LeftValue, results[i].LeftValue)
					return false
				}
				if inv.RightValue != results[i].RightValue {
					t.Logf("RightValue mismatch at %d: got %q, want %q", i, inv.RightValue, results[i].RightValue)
					return false
				}
			}

			// Verify allPassed and failedCount are correct
			expectedFailedCount := 0
			expectedAllPassed := true
			for _, r := range results {
				if !r.Passed {
					expectedFailedCount++
					expectedAllPassed = false
				}
			}

			if report.AllPassed != expectedAllPassed {
				t.Logf("AllPassed mismatch: got %v, want %v", report.AllPassed, expectedAllPassed)
				return false
			}

			if report.FailedCount != expectedFailedCount {
				t.Logf("FailedCount mismatch: got %d, want %d", report.FailedCount, expectedFailedCount)
				return false
			}

			return true
		},
		genResultList,
	))

	properties.TestingRun(t)
}
