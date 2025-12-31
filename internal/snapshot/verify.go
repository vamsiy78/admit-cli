package snapshot

import (
	"os"

	"admit/internal/execid"
)

// VerifyResult contains the result of snapshot verification.
type VerifyResult struct {
	Valid         bool   // Whether snapshot is valid overall
	IDMismatch    bool   // Execution ID doesn't match computed
	SchemaChanged bool   // Schema file has changed
	SchemaMessage string // Details about schema change
}

// Verify checks snapshot integrity.
// It recomputes the execution ID and checks if the schema still exists.
func Verify(snap ExecutionSnapshot, schemaKeys []string) VerifyResult {
	result := VerifyResult{Valid: true}

	// Build environ from snapshot environment map
	var environ []string
	for k, v := range snap.Environment {
		environ = append(environ, k+"="+v)
	}

	// Recompute execution ID from snapshot contents
	computed := execid.ComputeExecutionID(
		snap.ConfigVersion,
		snap.Command,
		snap.Args,
		environ,
		schemaKeys,
	)

	// Check if execution ID matches
	if computed.ExecutionID != snap.ExecutionID {
		result.Valid = false
		result.IDMismatch = true
	}

	// Check if schema file still exists
	if snap.SchemaPath != "" {
		if _, err := os.Stat(snap.SchemaPath); err != nil {
			if os.IsNotExist(err) {
				result.SchemaChanged = true
				result.SchemaMessage = "schema file no longer exists"
			}
		}
	}

	return result
}
