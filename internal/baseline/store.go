package baseline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrBaselineNotFound is returned when a baseline doesn't exist.
var ErrBaselineNotFound = errors.New("baseline not found")

// Store manages baseline persistence.
type Store struct {
	Dir string // Base directory for baselines
}

// NewStore creates a store with the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// DefaultDir returns the default baseline directory (~/.admit/baselines).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".admit/baselines"
	}
	return filepath.Join(home, ".admit", "baselines")
}

// ResolveDir returns the baseline directory from env var or default.
func ResolveDir(environ []string) string {
	for _, env := range environ {
		if strings.HasPrefix(env, "ADMIT_BASELINE_DIR=") {
			return strings.TrimPrefix(env, "ADMIT_BASELINE_DIR=")
		}
	}
	return DefaultDir()
}

// Save stores a baseline with the given name.
func (s *Store) Save(b Baseline) error {
	// Create directory if needed
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}

	path := s.path(b.Name)

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load retrieves a baseline by name.
func (s *Store) Load(name string) (Baseline, error) {
	path := s.path(name)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Baseline{}, ErrBaselineNotFound
		}
		return Baseline{}, err
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return Baseline{}, err
	}

	return b, nil
}

// List returns all stored baselines as summaries.
func (s *Store) List() ([]BaselineSummary, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BaselineSummary{}, nil
		}
		return nil, err
	}

	var summaries []BaselineSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.Dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var b Baseline
		if err := json.Unmarshal(data, &b); err != nil {
			continue // Skip invalid JSON
		}

		summaries = append(summaries, BaselineSummary{
			Name:       b.Name,
			ConfigHash: b.ConfigHash,
			Command:    b.Command,
			Timestamp:  b.Timestamp,
		})
	}

	return summaries, nil
}

// Delete removes a baseline by name.
func (s *Store) Delete(name string) error {
	path := s.path(name)

	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrBaselineNotFound
		}
		return err
	}

	return nil
}

// Exists checks if a baseline exists.
func (s *Store) Exists(name string) bool {
	path := s.path(name)
	_, err := os.Stat(path)
	return err == nil
}

// path returns the file path for a baseline name.
func (s *Store) path(name string) string {
	// Sanitize name for filesystem
	safeName := strings.ReplaceAll(name, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	return filepath.Join(s.Dir, safeName+".json")
}
