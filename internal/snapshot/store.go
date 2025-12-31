package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSnapshotNotFound is returned when a snapshot doesn't exist.
var ErrSnapshotNotFound = errors.New("snapshot not found")

// Store manages snapshot persistence.
type Store struct {
	Dir string // Base directory for snapshots
}

// NewStore creates a store with the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// DefaultDir returns the default snapshot directory (~/.admit/snapshots).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".admit/snapshots"
	}
	return filepath.Join(home, ".admit", "snapshots")
}

// ResolveDir returns the snapshot directory from env var or default.
func ResolveDir(environ []string) string {
	for _, env := range environ {
		if strings.HasPrefix(env, "ADMIT_SNAPSHOT_DIR=") {
			return strings.TrimPrefix(env, "ADMIT_SNAPSHOT_DIR=")
		}
	}
	return DefaultDir()
}

// Save stores a snapshot, returns the file path.
func (s *Store) Save(snap ExecutionSnapshot) (string, error) {
	// Create directory if needed
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return "", err
	}

	path := s.Path(snap.ExecutionID)

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}

// Load retrieves a snapshot by execution ID.
func (s *Store) Load(executionID string) (ExecutionSnapshot, error) {
	path := s.Path(executionID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ExecutionSnapshot{}, ErrSnapshotNotFound
		}
		return ExecutionSnapshot{}, err
	}

	var snap ExecutionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return ExecutionSnapshot{}, err
	}

	return snap, nil
}

// List returns all stored snapshots as summaries.
func (s *Store) List() ([]SnapshotSummary, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotSummary{}, nil
		}
		return nil, err
	}

	var summaries []SnapshotSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.Dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var snap ExecutionSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue // Skip invalid JSON
		}

		summaries = append(summaries, SnapshotSummary{
			ExecutionID: snap.ExecutionID,
			Command:     snap.Command,
			Timestamp:   snap.Timestamp,
		})
	}

	return summaries, nil
}

// Delete removes a snapshot by execution ID.
func (s *Store) Delete(executionID string) error {
	path := s.Path(executionID)

	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrSnapshotNotFound
		}
		return err
	}

	return nil
}

// Prune removes snapshots older than the given duration.
// Returns the number of snapshots deleted.
func (s *Store) Prune(olderThan time.Duration) (int, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.Dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var snap ExecutionSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}

		if snap.Timestamp.Before(cutoff) {
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// Exists checks if a snapshot exists.
func (s *Store) Exists(executionID string) bool {
	path := s.Path(executionID)
	_, err := os.Stat(path)
	return err == nil
}

// Path returns the file path for an execution ID.
// Replaces ':' with '_' for filesystem compatibility.
func (s *Store) Path(executionID string) string {
	// Replace : with _ for filesystem compatibility
	filename := strings.ReplaceAll(executionID, ":", "_") + ".json"
	return filepath.Join(s.Dir, filename)
}
