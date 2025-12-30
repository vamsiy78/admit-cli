package resolver

import (
	"strings"

	"admit/internal/schema"
)

// ResolvedValue represents a resolved config value
type ResolvedValue struct {
	Key     string // The config key path (e.g., "db.url")
	EnvVar  string // The environment variable name (e.g., "DB_URL")
	Value   string // The resolved value (empty if not set)
	Present bool   // Whether the env var was set
}

// Resolve looks up all config values from the environment.
// It takes a schema and an environ slice (format: "KEY=VALUE") and returns
// resolved values for each config key in the schema.
func Resolve(s schema.Schema, environ []string) []ResolvedValue {
	// Build a map from environ slice for O(1) lookups
	envMap := parseEnviron(environ)

	var results []ResolvedValue
	for path, configKey := range s.Config {
		envVar := PathToEnvVar(configKey.Path)
		value, present := envMap[envVar]

		results = append(results, ResolvedValue{
			Key:     path,
			EnvVar:  envVar,
			Value:   value,
			Present: present,
		})
	}

	return results
}

// parseEnviron converts an environ slice (["KEY=VALUE", ...]) into a map.
// Handles edge cases like empty values ("KEY=") and values containing "=" ("KEY=a=b").
func parseEnviron(environ []string) map[string]string {
	result := make(map[string]string)
	for _, entry := range environ {
		// Split on first "=" only - values can contain "="
		idx := strings.Index(entry, "=")
		if idx == -1 {
			// No "=" found, skip malformed entry
			continue
		}
		key := entry[:idx]
		value := entry[idx+1:]
		result[key] = value
	}
	return result
}
