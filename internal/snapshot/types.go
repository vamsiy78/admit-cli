// Package snapshot provides v5 execution replay functionality.
// It stores complete execution contexts and enables replaying them later.
package snapshot

import "time"

// ExecutionSnapshot represents a complete execution context for replay.
// It captures everything needed to reproduce an exact execution.
type ExecutionSnapshot struct {
	ExecutionID   string            `json:"executionId"`   // v4 execution fingerprint
	ConfigVersion string            `json:"configVersion"` // Hash of config values
	Command       string            `json:"command"`       // Target command
	Args          []string          `json:"args"`          // Command arguments
	Environment   map[string]string `json:"environment"`   // Schema-referenced env vars
	SchemaPath    string            `json:"schemaPath"`    // Path to schema used
	Timestamp     time.Time         `json:"timestamp"`     // When snapshot was created
}

// SnapshotSummary is a lightweight view for listing snapshots.
type SnapshotSummary struct {
	ExecutionID string    `json:"executionId"`
	Command     string    `json:"command"`
	Timestamp   time.Time `json:"timestamp"`
}
