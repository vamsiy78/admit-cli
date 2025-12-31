package drift

import (
	"sort"
	"time"

	"admit/internal/baseline"
)

// DriftType represents the type of configuration change.
type DriftType string

const (
	DriftAdded   DriftType = "added"   // Key in current but not baseline
	DriftRemoved DriftType = "removed" // Key in baseline but not current
	DriftChanged DriftType = "changed" // Key in both with different values
)

// KeyDrift represents a single key's drift.
type KeyDrift struct {
	Key           string    `json:"key"`
	Type          DriftType `json:"type"`
	BaselineValue string    `json:"baselineValue,omitempty"`
	CurrentValue  string    `json:"currentValue,omitempty"`
}

// DriftReport contains the full drift analysis.
type DriftReport struct {
	HasDrift     bool       `json:"hasDrift"`
	BaselineName string     `json:"baselineName"`
	BaselineHash string     `json:"baselineHash"`
	CurrentHash  string     `json:"currentHash"`
	BaselineTime time.Time  `json:"baselineTime"`
	Changes      []KeyDrift `json:"changes"`
}

// Detect compares current config against baseline and returns drift report.
func Detect(b baseline.Baseline, currentValues map[string]string, currentHash string) DriftReport {
	report := DriftReport{
		BaselineName: b.Name,
		BaselineHash: b.ConfigHash,
		CurrentHash:  currentHash,
		BaselineTime: b.Timestamp,
		Changes:      []KeyDrift{},
	}

	// Quick check: if hashes match, no drift
	if b.ConfigHash == currentHash {
		return report
	}

	// Collect all keys from both configs
	allKeys := make(map[string]bool)
	for k := range b.ConfigValues {
		allKeys[k] = true
	}
	for k := range currentValues {
		allKeys[k] = true
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Compare each key
	for _, key := range keys {
		baselineVal, inBaseline := b.ConfigValues[key]
		currentVal, inCurrent := currentValues[key]

		if inBaseline && !inCurrent {
			// Key removed
			report.Changes = append(report.Changes, KeyDrift{
				Key:           key,
				Type:          DriftRemoved,
				BaselineValue: baselineVal,
			})
		} else if !inBaseline && inCurrent {
			// Key added
			report.Changes = append(report.Changes, KeyDrift{
				Key:          key,
				Type:         DriftAdded,
				CurrentValue: currentVal,
			})
		} else if baselineVal != currentVal {
			// Key changed
			report.Changes = append(report.Changes, KeyDrift{
				Key:           key,
				Type:          DriftChanged,
				BaselineValue: baselineVal,
				CurrentValue:  currentVal,
			})
		}
	}

	report.HasDrift = len(report.Changes) > 0
	return report
}
