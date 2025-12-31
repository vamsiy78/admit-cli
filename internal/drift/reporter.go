package drift

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatCLI formats drift report for terminal output.
func FormatCLI(report DriftReport) string {
	if !report.HasDrift {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("⚠️  Configuration drift detected since last execution:\n")

	for _, change := range report.Changes {
		switch change.Type {
		case DriftAdded:
			sb.WriteString(fmt.Sprintf("  + %s: (new) → %s\n", change.Key, change.CurrentValue))
		case DriftRemoved:
			sb.WriteString(fmt.Sprintf("  - %s: %s → (removed)\n", change.Key, change.BaselineValue))
		case DriftChanged:
			sb.WriteString(fmt.Sprintf("  ~ %s: %s → %s\n", change.Key, change.BaselineValue, change.CurrentValue))
		}
	}

	sb.WriteString("\nExecution continues.\n")
	return sb.String()
}

// FormatCI formats drift report as GitHub Actions warning annotations.
func FormatCI(report DriftReport) string {
	if !report.HasDrift {
		return ""
	}

	var sb strings.Builder

	for _, change := range report.Changes {
		var msg string
		switch change.Type {
		case DriftAdded:
			msg = fmt.Sprintf("Config drift: %s added (value: %s)", change.Key, change.CurrentValue)
		case DriftRemoved:
			msg = fmt.Sprintf("Config drift: %s removed (was: %s)", change.Key, change.BaselineValue)
		case DriftChanged:
			msg = fmt.Sprintf("Config drift: %s changed from '%s' to '%s'", change.Key, change.BaselineValue, change.CurrentValue)
		}
		sb.WriteString(fmt.Sprintf("::warning file=admit.yaml::%s\n", msg))
	}

	sb.WriteString(fmt.Sprintf("\n⚠️  Configuration drift detected: %d change(s) since baseline '%s'\n", len(report.Changes), report.BaselineName))
	return sb.String()
}

// FormatJSON formats drift report as JSON.
func FormatJSON(report DriftReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
