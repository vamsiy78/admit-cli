package invariant

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ViolationReport represents the JSON output format for invariant results
type ViolationReport struct {
	Invariants  []InvariantResultJSON `json:"invariants"`
	AllPassed   bool                  `json:"allPassed"`
	FailedCount int                   `json:"failedCount"`
}

// InvariantResultJSON represents a single invariant result in JSON format
type InvariantResultJSON struct {
	Name       string `json:"name"`
	Rule       string `json:"rule"`
	Passed     bool   `json:"passed"`
	LeftValue  string `json:"leftValue"`
	RightValue string `json:"rightValue"`
	Message    string `json:"message"`
}

// FormatViolation formats a single invariant violation as a human-readable string
// The output prominently displays the invariant name, rule, and evaluated values
func FormatViolation(result InvariantResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("INVARIANT VIOLATION: '%s'\n", result.Name))
	sb.WriteString(fmt.Sprintf("  Rule: %s\n", result.Rule))

	if result.LeftValue != "" || result.RightValue != "" {
		sb.WriteString(fmt.Sprintf("  Values: left='%s', right='%s'\n", result.LeftValue, result.RightValue))
	}

	if result.Message != "" {
		sb.WriteString(fmt.Sprintf("  Reason: %s\n", result.Message))
	}

	return sb.String()
}

// FormatViolations formats multiple invariant violations as a human-readable string
// All violations are included in the output
func FormatViolations(results []InvariantResult) string {
	var violations []InvariantResult
	for _, r := range results {
		if !r.Passed {
			violations = append(violations, r)
		}
	}

	if len(violations) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Invariant check failed: %d violation(s)\n\n", len(violations)))

	for _, v := range violations {
		sb.WriteString(FormatViolation(v))
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatJSON formats invariant results as JSON for --invariants-json output
func FormatJSON(results []InvariantResult) (string, error) {
	report := ViolationReport{
		Invariants:  make([]InvariantResultJSON, 0, len(results)),
		AllPassed:   true,
		FailedCount: 0,
	}

	for _, r := range results {
		jsonResult := InvariantResultJSON{
			Name:       r.Name,
			Rule:       r.Rule,
			Passed:     r.Passed,
			LeftValue:  r.LeftValue,
			RightValue: r.RightValue,
			Message:    r.Message,
		}
		report.Invariants = append(report.Invariants, jsonResult)

		if !r.Passed {
			report.AllPassed = false
			report.FailedCount++
		}
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal invariant results: %w", err)
	}

	return string(jsonBytes), nil
}

// HasViolations returns true if any invariant result is a violation
func HasViolations(results []InvariantResult) bool {
	for _, r := range results {
		if !r.Passed {
			return true
		}
	}
	return false
}

// GetViolations returns only the failed invariant results
func GetViolations(results []InvariantResult) []InvariantResult {
	var violations []InvariantResult
	for _, r := range results {
		if !r.Passed {
			violations = append(violations, r)
		}
	}
	return violations
}
