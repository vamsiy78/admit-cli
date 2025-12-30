package artifact

import (
	"os"
	"path/filepath"
)

// WriteToFile writes the artifact to the specified path, creating parent directories if needed.
func (a ConfigArtifact) WriteToFile(path string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	jsonBytes, err := a.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonBytes, 0644)
}
