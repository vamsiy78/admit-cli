package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"admit/internal/resolver"
)

// ConfigArtifact represents the immutable config artifact
type ConfigArtifact struct {
	ConfigVersion string            `json:"configVersion"` // sha256:hex
	Values        map[string]string `json:"values"`
}

// GenerateArtifact creates a config artifact from resolved values.
// Only includes values that are present (set in environment).
func GenerateArtifact(resolved []resolver.ResolvedValue) ConfigArtifact {
	values := make(map[string]string)
	for _, rv := range resolved {
		if rv.Present {
			values[rv.Key] = rv.Value
		}
	}

	return ConfigArtifact{
		ConfigVersion: ComputeConfigVersion(values),
		Values:        values,
	}
}

// ComputeConfigVersion computes the SHA-256 hash of the values in canonical form.
// Returns the hash prefixed with "sha256:".
func ComputeConfigVersion(values map[string]string) string {
	canonical := canonicalValuesJSON(values)
	hash := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// ToCanonicalJSON serializes the artifact to canonical JSON (sorted keys, no whitespace).
// This is used for deterministic hashing.
func (a ConfigArtifact) ToCanonicalJSON() ([]byte, error) {
	// Build canonical form manually for deterministic output
	return canonicalArtifactJSON(a), nil
}

// ToJSON serializes the artifact to pretty-printed JSON for human readability.
func (a ConfigArtifact) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// canonicalValuesJSON produces canonical JSON for just the values map.
// Keys are sorted alphabetically, no whitespace.
func canonicalValuesJSON(values map[string]string) []byte {
	if len(values) == 0 {
		return []byte("{}")
	}

	// Get sorted keys
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build JSON manually for canonical form
	result := []byte("{")
	for i, k := range keys {
		if i > 0 {
			result = append(result, ',')
		}
		// JSON encode key and value
		keyJSON, _ := json.Marshal(k)
		valueJSON, _ := json.Marshal(values[k])
		result = append(result, keyJSON...)
		result = append(result, ':')
		result = append(result, valueJSON...)
	}
	result = append(result, '}')
	return result
}

// canonicalArtifactJSON produces canonical JSON for the full artifact.
// Keys are sorted alphabetically, no whitespace.
func canonicalArtifactJSON(a ConfigArtifact) []byte {
	// configVersion comes before values alphabetically
	configVersionJSON, _ := json.Marshal(a.ConfigVersion)
	valuesJSON := canonicalValuesJSON(a.Values)

	result := []byte(`{"configVersion":`)
	result = append(result, configVersionJSON...)
	result = append(result, `,"values":`...)
	result = append(result, valuesJSON...)
	result = append(result, '}')
	return result
}
