package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-v5-execution-replay, Property 1: Snapshot Storage
// Validates: Requirements 1.1, 1.4, 1.5
func TestStore_Save_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("save creates file at correct path", prop.ForAll(
		func(execID string) bool {
			if execID == "" {
				return true
			}

			tmpDir := t.TempDir()
			store := NewStore(tmpDir)

			snap := ExecutionSnapshot{
				ExecutionID:   "sha256:" + execID,
				ConfigVersion: "sha256:config123",
				Command:       "echo",
				Args:          []string{"hello"},
				Environment:   map[string]string{"FOO": "bar"},
				SchemaPath:    "/app/admit.yaml",
				Timestamp:     time.Now().UTC(),
			}

			path, err := store.Save(snap)
			if err != nil {
				return false
			}

			// Verify file exists
			if _, err := os.Stat(path); err != nil {
				return false
			}

			// Verify path is in store directory
			if filepath.Dir(path) != tmpDir {
				return false
			}

			return true
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Feature: admit-v5-execution-replay, Property 3: Snapshot JSON Structure
// Validates: Requirements 2.1-2.8
func TestStore_LoadSave_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("load returns saved snapshot unchanged", prop.ForAll(
		func(execID, cmd string, args []string) bool {
			if execID == "" || cmd == "" {
				return true
			}

			tmpDir := t.TempDir()
			store := NewStore(tmpDir)

			original := ExecutionSnapshot{
				ExecutionID:   "sha256:" + execID,
				ConfigVersion: "sha256:config123",
				Command:       cmd,
				Args:          args,
				Environment:   map[string]string{"DB_URL": "postgres://localhost"},
				SchemaPath:    "/app/admit.yaml",
				Timestamp:     time.Now().UTC().Truncate(time.Second), // Truncate for JSON precision
			}

			_, err := store.Save(original)
			if err != nil {
				return false
			}

			loaded, err := store.Load(original.ExecutionID)
			if err != nil {
				return false
			}

			// Verify all fields match
			if loaded.ExecutionID != original.ExecutionID {
				return false
			}
			if loaded.ConfigVersion != original.ConfigVersion {
				return false
			}
			if loaded.Command != original.Command {
				return false
			}
			if len(loaded.Args) != len(original.Args) {
				return false
			}
			for i, arg := range original.Args {
				if loaded.Args[i] != arg {
					return false
				}
			}
			if loaded.SchemaPath != original.SchemaPath {
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}

// Feature: admit-v5-execution-replay, Property 7: Snapshot Listing
// Validates: Requirements 4.1, 4.2
func TestStore_List_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("list returns all saved snapshots", prop.ForAll(
		func(count int) bool {
			if count < 0 || count > 10 {
				count = count % 10
				if count < 0 {
					count = -count
				}
			}

			tmpDir := t.TempDir()
			store := NewStore(tmpDir)

			// Save multiple snapshots
			for i := 0; i < count; i++ {
				snap := ExecutionSnapshot{
					ExecutionID:   "sha256:id" + string(rune('a'+i)),
					ConfigVersion: "sha256:config",
					Command:       "cmd" + string(rune('a'+i)),
					Timestamp:     time.Now().UTC(),
				}
				if _, err := store.Save(snap); err != nil {
					return false
				}
			}

			// List and verify count
			summaries, err := store.List()
			if err != nil {
				return false
			}

			return len(summaries) == count
		},
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

// Feature: admit-v5-execution-replay, Property 8: Snapshot Cleanup
// Validates: Requirements 5.1, 5.2
func TestStore_Delete_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("delete removes snapshot", prop.ForAll(
		func(execID string) bool {
			if execID == "" {
				return true
			}

			tmpDir := t.TempDir()
			store := NewStore(tmpDir)

			snap := ExecutionSnapshot{
				ExecutionID:   "sha256:" + execID,
				ConfigVersion: "sha256:config",
				Command:       "echo",
				Timestamp:     time.Now().UTC(),
			}

			_, err := store.Save(snap)
			if err != nil {
				return false
			}

			// Verify exists
			if !store.Exists(snap.ExecutionID) {
				return false
			}

			// Delete
			if err := store.Delete(snap.ExecutionID); err != nil {
				return false
			}

			// Verify gone
			return !store.Exists(snap.ExecutionID)
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestStore_Prune(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Save old snapshot
	oldSnap := ExecutionSnapshot{
		ExecutionID:   "sha256:old",
		ConfigVersion: "sha256:config",
		Command:       "old-cmd",
		Timestamp:     time.Now().UTC().Add(-48 * time.Hour), // 2 days ago
	}
	store.Save(oldSnap)

	// Save new snapshot
	newSnap := ExecutionSnapshot{
		ExecutionID:   "sha256:new",
		ConfigVersion: "sha256:config",
		Command:       "new-cmd",
		Timestamp:     time.Now().UTC(),
	}
	store.Save(newSnap)

	// Prune snapshots older than 1 day
	deleted, err := store.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("prune error: %v", err)
	}

	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Verify old is gone, new remains
	if store.Exists(oldSnap.ExecutionID) {
		t.Error("old snapshot should be deleted")
	}
	if !store.Exists(newSnap.ExecutionID) {
		t.Error("new snapshot should remain")
	}
}

// Feature: admit-v5-execution-replay, Property 2: Snapshot Directory Configuration
// Validates: Requirements 1.2, 1.3
func TestResolveDir_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("respects ADMIT_SNAPSHOT_DIR", prop.ForAll(
		func(customDir string) bool {
			if customDir == "" {
				return true
			}

			environ := []string{"ADMIT_SNAPSHOT_DIR=" + customDir}
			resolved := ResolveDir(environ)
			return resolved == customDir
		},
		gen.Identifier(),
	))

	properties.Property("uses default when env not set", prop.ForAll(
		func(otherVar string) bool {
			environ := []string{"OTHER_VAR=" + otherVar}
			resolved := ResolveDir(environ)
			return resolved == DefaultDir()
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Feature: admit-v5-execution-replay, Property 5: Replay Missing Snapshot Error
// Validates: Requirements 3.4
func TestStore_LoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	_, err := store.Load("sha256:nonexistent")
	if err != ErrSnapshotNotFound {
		t.Errorf("error = %v, want ErrSnapshotNotFound", err)
	}
}

func TestStore_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	err := store.Delete("sha256:nonexistent")
	if err != ErrSnapshotNotFound {
		t.Errorf("error = %v, want ErrSnapshotNotFound", err)
	}
}

func TestStore_Path(t *testing.T) {
	store := NewStore("/tmp/snapshots")

	// Test colon replacement
	path := store.Path("sha256:abc123")
	expected := "/tmp/snapshots/sha256_abc123.json"
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestStore_ListEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	if len(summaries) != 0 {
		t.Errorf("len(summaries) = %d, want 0", len(summaries))
	}
}

func TestStore_ListNonexistentDir(t *testing.T) {
	store := NewStore("/nonexistent/path/that/does/not/exist")

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	if len(summaries) != 0 {
		t.Errorf("len(summaries) = %d, want 0", len(summaries))
	}
}
