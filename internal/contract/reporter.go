package contract

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatCLI formats violations for terminal output.
// Output includes key name, actual value, rule type, and expected/forbidden values.
func FormatCLI(result EvalResult) string {
	if result.Passed || len(result.Violations) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("❌ Contract violations for environment '%s':\n\n", result.Environment))

	for _, v := range result.Violations {
		sb.WriteString(fmt.Sprintf("  Key: %s\n", v.Key))
		sb.WriteString(fmt.Sprintf("  Value: %s\n", v.ActualValue))
		sb.WriteString(fmt.Sprintf("  Rule: %s\n", v.RuleType))

		if v.RuleType == "allow" {
			sb.WriteString(fmt.Sprintf("  Expected: %s\n", formatValues(v.ExpectedValues)))
		} else if v.RuleType == "deny" {
			if v.Pattern != "" {
				sb.WriteString(fmt.Sprintf("  Forbidden: %s (matched pattern: %s)\n", formatValues(v.ExpectedValues), v.Pattern))
			} else {
				sb.WriteString(fmt.Sprintf("  Forbidden: %s\n", formatValues(v.ExpectedValues)))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Execution blocked: %d violation(s)\n", len(result.Violations)))
	return sb.String()
}

// FormatCI formats violations as GitHub Actions error annotations.
func FormatCI(result EvalResult) string {
	if result.Passed || len(result.Violations) == 0 {
		return ""
	}

	var sb strings.Builder

	for _, v := range result.Violations {
		var msg string
		if v.RuleType == "allow" {
			msg = fmt.Sprintf("Contract violation: %s has value '%s', expected one of: %s",
				v.Key, v.ActualValue, formatValues(v.ExpectedValues))
		} else if v.RuleType == "deny" {
			if v.Pattern != "" {
				msg = fmt.Sprintf("Contract violation: %s has forbidden value '%s' (matched pattern: %s)",
					v.Key, v.ActualValue, v.Pattern)
			} else {
				msg = fmt.Sprintf("Contract violation: %s has forbidden value '%s'",
					v.Key, v.ActualValue)
			}
		}
		sb.WriteString(fmt.Sprintf("::error file=admit.yaml::%s\n", msg))
	}

	sb.WriteString(fmt.Sprintf("\n❌ Contract violations for environment '%s': %d violation(s)\n",
		result.Environment, len(result.Violations)))
	return sb.String()
}

// FormatJSON formats violations as JSON.
func FormatJSON(result EvalResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// formatValues formats a slice of values for display.
func formatValues(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	if len(values) == 1 {
		return values[0]
	}
	return "[" + strings.Join(values, ", ") + "]"
}
