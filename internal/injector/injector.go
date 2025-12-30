package injector

import (
	"os"
	"path/filepath"
	"strings"

	"admit/internal/artifact"
)

// InjectFile writes the artifact to a file for the target process to read.
// Creates parent directories if they don't exist.
// Path is resolved relative to current working directory if relative.
func InjectFile(art artifact.ConfigArtifact, path string) error {
	// Resolve relative paths to absolute
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = filepath.Join(cwd, path)
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	jsonBytes, err := art.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonBytes, 0644)
}

// InjectEnv adds the artifact JSON to the environment under the specified variable name.
// Returns a new environ slice with the artifact added.
// Original environment variables are preserved.
func InjectEnv(art artifact.ConfigArtifact, environ []string, varName string) ([]string, error) {
	jsonBytes, err := art.ToJSON()
	if err != nil {
		return nil, err
	}

	// Create new environ with the artifact
	// First, copy existing environ (excluding any existing var with same name)
	result := make([]string, 0, len(environ)+1)
	prefix := varName + "="
	for _, env := range environ {
		if !strings.HasPrefix(env, prefix) {
			result = append(result, env)
		}
	}

	// Add the artifact
	result = append(result, varName+"="+string(jsonBytes))

	return result, nil
}
