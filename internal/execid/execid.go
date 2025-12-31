// Package execid provides v4 execution identity computation.
// Unlike v1 identity (which hashes executable file content), v4 execution identity
// captures the complete execution context: validated config, command with arguments,
// and relevant environment variables.
package execid

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExecutionIdentityV4 represents the v4 execution fingerprint.
// It provides a deterministic identifier for "what exactly ran".
type ExecutionIdentityV4 struct {
	ExecutionID     string   `json:"executionId"`     // Final fingerprint hash
	ConfigVersion   string   `json:"configVersion"`   // Hash of config values (from artifact)
	CommandHash     string   `json:"commandHash"`     // Hash of command + args
	EnvironmentHash string   `json:"environmentHash"` // Hash of relevant env vars
	Command         string   `json:"command"`         // The target command
	Args            []string `json:"args"`            // Command arguments
}

// ComputeExecutionID generates the v4 execution identity.
// It combines config version, command hash, and environment hash into a single fingerprint.
func ComputeExecutionID(
	configVersion string,
	command string,
	args []string,
	environ []string,
	schemaKeys []string,
) ExecutionIdentityV4 {
	commandHash := ComputeCommandHash(command, args)
	environmentHash := ComputeEnvironmentHash(environ, schemaKeys)

	// Compute final execution ID by hashing the concatenation of all components
	combined := configVersion + commandHash + environmentHash
	executionID := hashString(combined)

	return ExecutionIdentityV4{
		ExecutionID:     executionID,
		ConfigVersion:   configVersion,
		CommandHash:     commandHash,
		EnvironmentHash: environmentHash,
		Command:         command,
		Args:            args,
	}
}

// ComputeCommandHash hashes the command and arguments.
// Uses null byte separator to ensure unambiguous parsing.
func ComputeCommandHash(command string, args []string) string {
	// Build: command + "\x00" + arg1 + "\x00" + arg2 + ...
	var parts []string
	parts = append(parts, command)
	parts = append(parts, args...)
	
	// Join with null byte separator
	data := strings.Join(parts, "\x00")
	return hashString(data)
}

// ComputeEnvironmentHash hashes relevant environment variables.
// Only includes env vars that correspond to schema config keys.
// Sorts by key name for determinism.
func ComputeEnvironmentHash(environ []string, schemaKeys []string) string {
	// Build a set of expected env var names from schema keys
	expectedEnvVars := make(map[string]bool)
	for _, key := range schemaKeys {
		envVar := pathToEnvVar(key)
		expectedEnvVars[envVar] = true
	}

	// Filter environ to only schema-referenced vars
	var relevantVars []string
	for _, env := range environ {
		idx := strings.Index(env, "=")
		if idx == -1 {
			continue
		}
		key := env[:idx]
		if expectedEnvVars[key] {
			relevantVars = append(relevantVars, env)
		}
	}

	// Sort for determinism
	sort.Strings(relevantVars)

	// Join with null byte separator
	data := strings.Join(relevantVars, "\x00")
	return hashString(data)
}

// ToJSON serializes the identity to pretty-printed JSON.
func (e ExecutionIdentityV4) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// WriteToFile writes the identity to the specified path.
func (e ExecutionIdentityV4) WriteToFile(path string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	jsonBytes, err := e.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonBytes, 0644)
}

// Short returns just the executionId string.
func (e ExecutionIdentityV4) Short() string {
	return e.ExecutionID
}

// hashString computes SHA-256 hash of a string and returns sha256:hex format.
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// pathToEnvVar converts a config path to environment variable name.
// e.g., "db.url" -> "DB_URL"
func pathToEnvVar(path string) string {
	return strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
}
