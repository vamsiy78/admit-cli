package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"admit/internal/artifact"
	"admit/internal/cli"
)

// ExecutionIdentity represents the unique identity of an execution
type ExecutionIdentity struct {
	CodeHash    string `json:"codeHash"`    // sha256:hex of executable
	ConfigHash  string `json:"configHash"`  // same as configVersion
	ExecutionID string `json:"executionId"` // codeHash:configHash
}

// ComputeIdentity generates the execution identity from command and artifact.
func ComputeIdentity(cmd cli.Command, art artifact.ConfigArtifact) (ExecutionIdentity, error) {
	codeHash, err := ComputeCodeHash(cmd)
	if err != nil {
		return ExecutionIdentity{}, err
	}

	configHash := art.ConfigVersion

	return ExecutionIdentity{
		CodeHash:    codeHash,
		ConfigHash:  configHash,
		ExecutionID: codeHash + ":" + configHash,
	}, nil
}

// ComputeCodeHash computes SHA-256 of the executable file content.
// Falls back to hashing the command string for non-file commands (e.g., shell builtins).
func ComputeCodeHash(cmd cli.Command) (string, error) {
	// Try to find the executable path
	execPath, err := exec.LookPath(cmd.Target)
	if err != nil {
		// Fallback: hash the command string itself
		return hashString(cmd.Target), nil
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(execPath)
	if err != nil {
		return hashString(cmd.Target), nil
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		// Fallback: hash the command string
		return hashString(cmd.Target), nil
	}

	return hashBytes(content), nil
}

// ToJSON serializes the identity to pretty-printed JSON.
func (i ExecutionIdentity) ToJSON() ([]byte, error) {
	return json.MarshalIndent(i, "", "  ")
}

// Short returns just the executionId string.
func (i ExecutionIdentity) Short() string {
	return i.ExecutionID
}

// WriteToFile writes the identity to the specified path.
func (i ExecutionIdentity) WriteToFile(path string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	jsonBytes, err := i.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonBytes, 0644)
}

// hashBytes computes SHA-256 hash of bytes and returns sha256:hex format.
func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// hashString computes SHA-256 hash of a string and returns sha256:hex format.
func hashString(s string) string {
	return hashBytes([]byte(s))
}
