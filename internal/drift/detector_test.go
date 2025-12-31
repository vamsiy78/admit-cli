package drift

import (
	"testing"
	"time"

	"admit/internal/baseline"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genIdentifier generates valid identifier strings
func genIdentifier() gopter.Gen {
	return gen.Identifier()
}

// genConfigValues generates random config value maps
func genConfigValues() gopter.Gen {
	return gen.MapOf(genIdentifier(), gen.AlphaString()).Map(func(m map[string]string) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return m
	})
}

// TestNoDriftWhenHashesMatch tests Property 5: No Drift When Hashes Match
// For any baseline and current config with identical hashes, no drift should be reported.
// Validates: Requirements 2.3
func TestNoDriftWhenHashesMatch(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("no drift when config hashes match", prop.ForAll(
		func(configValues map[string]string) bool {
			hash := "sha256:samehash"

			b := baseline.Baseline{
				Name:         "test",
				ExecutionID:  "sha256:exec",
				ConfigHash:   hash,
				ConfigValues: configValues,
				Command:      "cmd",
				Timestamp:    time.Now().UTC(),
			}

			report := Detect(b, configValues, hash)

			// Should have no drift
			if report.HasDrift {
				return false
			}
			if len(report.Changes) != 0 {
				return false
			}

			return true
		},
		genConfigValues(),
	))

	properties.TestingRun(t)
}

// TestDriftReportContainsKeyDifferences tests Property 6: Drift Report Contains Key Differences
// For any baseline and current config with different values, the report should contain all differences.
// Validates: Requirements 2.4, 3.4, 3.5, 3.6, 3.7
func TestDriftReportContainsKeyDifferences(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("drift report contains all key differences", prop.ForAll(
		func(key, baselineVal, currentVal string) bool {
			// Test changed key
			b := baseline.Baseline{
				Name:         "test",
				ExecutionID:  "sha256:exec",
				ConfigHash:   "sha256:oldhash",
				ConfigValues: map[string]string{key: baselineVal},
				Command:      "cmd",
				Timestamp:    time.Now().UTC(),
			}

			currentValues := map[string]string{key: currentVal}
			report := Detect(b, currentValues, "sha256:newhash")

			if baselineVal == currentVal {
				// No change expected
				return !report.HasDrift
			}

			// Should detect change
			if !report.HasDrift {
				return false
			}
			if len(report.Changes) != 1 {
				return false
			}
			change := report.Changes[0]
			if change.Key != key {
				return false
			}
			if change.Type != DriftChanged {
				return false
			}
			if change.BaselineValue != baselineVal {
				return false
			}
			if change.CurrentValue != currentVal {
				return false
			}

			return true
		},
		genIdentifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("added keys are detected", prop.ForAll(
		func(key, value string) bool {
			b := baseline.Baseline{
				Name:         "test",
				ExecutionID:  "sha256:exec",
				ConfigHash:   "sha256:oldhash",
				ConfigValues: map[string]string{},
				Command:      "cmd",
				Timestamp:    time.Now().UTC(),
			}

			currentValues := map[string]string{key: value}
			report := Detect(b, currentValues, "sha256:newhash")

			if !report.HasDrift {
				return false
			}
			if len(report.Changes) != 1 {
				return false
			}
			change := report.Changes[0]
			if change.Key != key {
				return false
			}
			if change.Type != DriftAdded {
				return false
			}
			if change.CurrentValue != value {
				return false
			}

			return true
		},
		genIdentifier(),
		gen.AlphaString(),
	))

	properties.Property("removed keys are detected", prop.ForAll(
		func(key, value string) bool {
			b := baseline.Baseline{
				Name:         "test",
				ExecutionID:  "sha256:exec",
				ConfigHash:   "sha256:oldhash",
				ConfigValues: map[string]string{key: value},
				Command:      "cmd",
				Timestamp:    time.Now().UTC(),
			}

			currentValues := map[string]string{}
			report := Detect(b, currentValues, "sha256:newhash")

			if !report.HasDrift {
				return false
			}
			if len(report.Changes) != 1 {
				return false
			}
			change := report.Changes[0]
			if change.Key != key {
				return false
			}
			if change.Type != DriftRemoved {
				return false
			}
			if change.BaselineValue != value {
				return false
			}

			return true
		},
		genIdentifier(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// TestDriftReportMetadata tests that drift report contains correct metadata
func TestDriftReportMetadata(t *testing.T) {
	baselineTime := time.Now().UTC()
	b := baseline.Baseline{
		Name:         "production",
		ExecutionID:  "sha256:exec123",
		ConfigHash:   "sha256:oldhash",
		ConfigValues: map[string]string{"key": "old"},
		Command:      "cmd",
		Timestamp:    baselineTime,
	}

	currentValues := map[string]string{"key": "new"}
	report := Detect(b, currentValues, "sha256:newhash")

	if report.BaselineName != "production" {
		t.Errorf("expected baseline name 'production', got '%s'", report.BaselineName)
	}
	if report.BaselineHash != "sha256:oldhash" {
		t.Errorf("expected baseline hash 'sha256:oldhash', got '%s'", report.BaselineHash)
	}
	if report.CurrentHash != "sha256:newhash" {
		t.Errorf("expected current hash 'sha256:newhash', got '%s'", report.CurrentHash)
	}
	if !report.BaselineTime.Equal(baselineTime) {
		t.Errorf("expected baseline time %v, got %v", baselineTime, report.BaselineTime)
	}
}
