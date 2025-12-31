package baseline

import "time"

// Baseline represents a known-good execution state for drift comparison.
type Baseline struct {
	Name         string            `json:"name"`         // Baseline identifier
	ExecutionID  string            `json:"executionId"`  // v4 execution fingerprint
	ConfigHash   string            `json:"configHash"`   // Config artifact hash (configVersion)
	ConfigValues map[string]string `json:"configValues"` // Resolved config values
	Command      string            `json:"command"`      // Command that was run
	Timestamp    time.Time         `json:"timestamp"`    // When baseline was created
}

// BaselineSummary is a lightweight view for listing baselines.
type BaselineSummary struct {
	Name       string    `json:"name"`
	ConfigHash string    `json:"configHash"`
	Command    string    `json:"command"`
	Timestamp  time.Time `json:"timestamp"`
}
