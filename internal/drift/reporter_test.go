package drift

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestDriftReportFormatting tests Property 8: Drift Report Formatting
// For any drift report with changes, all formats should contain the key information.
// Validates: Requirements 3.1, 3.2, 3.3
func TestDriftReportFormatting(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("CLI format contains key names and values", prop.ForAll(
		func(key, oldVal, newVal string) bool {
			report := DriftReport{
				HasDrift:     true,
				BaselineName: "test",
				BaselineHash: "sha256:old",
				CurrentHash:  "sha256:new",
				BaselineTime: time.Now().UTC(),
				Changes: []KeyDrift{
					{Key: key, Type: DriftChanged, BaselineValue: oldVal, CurrentValue: newVal},
				},
			}

			output := FormatCLI(report)

			// Should contain key name
			if !strings.Contains(output, key) {
				return false
			}
			// Should contain old value
			if !strings.Contains(output, oldVal) {
				return false
			}
			// Should contain new value
			if !strings.Contains(output, newVal) {
				return false
			}
			// Should indicate drift detected
			if !strings.Contains(output, "drift") {
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("CI format contains warning annotations", prop.ForAll(
		func(key, value string) bool {
			report := DriftReport{
				HasDrift:     true,
				BaselineName: "test",
				BaselineHash: "sha256:old",
				CurrentHash:  "sha256:new",
				BaselineTime: time.Now().UTC(),
				Changes: []KeyDrift{
					{Key: key, Type: DriftAdded, CurrentValue: value},
				},
			}

			output := FormatCI(report)

			// Should contain ::warning:: annotation
			if !strings.Contains(output, "::warning") {
				return false
			}
			// Should contain key name
			if !strings.Contains(output, key) {
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.AlphaString(),
	))

	properties.Property("JSON format is valid JSON with all fields", prop.ForAll(
		func(key, oldVal, newVal string) bool {
			report := DriftReport{
				HasDrift:     true,
				BaselineName: "test",
				BaselineHash: "sha256:old",
				CurrentHash:  "sha256:new",
				BaselineTime: time.Now().UTC(),
				Changes: []KeyDrift{
					{Key: key, Type: DriftChanged, BaselineValue: oldVal, CurrentValue: newVal},
				},
			}

			output, err := FormatJSON(report)
			if err != nil {
				return false
			}

			// Should be valid JSON
			var parsed DriftReport
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				return false
			}

			// Should contain all fields
			if !parsed.HasDrift {
				return false
			}
			if parsed.BaselineName != "test" {
				return false
			}
			if len(parsed.Changes) != 1 {
				return false
			}
			if parsed.Changes[0].Key != key {
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// TestFormatCLIEmpty tests that empty drift report produces no output
func TestFormatCLIEmpty(t *testing.T) {
	report := DriftReport{HasDrift: false}
	output := FormatCLI(report)
	if output != "" {
		t.Errorf("expected empty output for no drift, got: %s", output)
	}
}

// TestFormatCIEmpty tests that empty drift report produces no output
func TestFormatCIEmpty(t *testing.T) {
	report := DriftReport{HasDrift: false}
	output := FormatCI(report)
	if output != "" {
		t.Errorf("expected empty output for no drift, got: %s", output)
	}
}

// TestFormatCLIDriftTypes tests all drift types are formatted correctly
func TestFormatCLIDriftTypes(t *testing.T) {
	report := DriftReport{
		HasDrift:     true,
		BaselineName: "test",
		BaselineHash: "sha256:old",
		CurrentHash:  "sha256:new",
		BaselineTime: time.Now().UTC(),
		Changes: []KeyDrift{
			{Key: "added.key", Type: DriftAdded, CurrentValue: "newval"},
			{Key: "removed.key", Type: DriftRemoved, BaselineValue: "oldval"},
			{Key: "changed.key", Type: DriftChanged, BaselineValue: "old", CurrentValue: "new"},
		},
	}

	output := FormatCLI(report)

	// Check added key format
	if !strings.Contains(output, "+ added.key") {
		t.Error("expected '+' prefix for added key")
	}
	// Check removed key format
	if !strings.Contains(output, "- removed.key") {
		t.Error("expected '-' prefix for removed key")
	}
	// Check changed key format
	if !strings.Contains(output, "~ changed.key") {
		t.Error("expected '~' prefix for changed key")
	}
}
