package baseline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genIdentifier generates valid identifier strings
func genIdentifier() gopter.Gen {
	return gen.Identifier()
}

// genConfigValues generates random config value maps
func genConfigValues() gopter.Gen {
	return gen.MapOf(genIdentifier(), gen.AlphaString()).Map(func(m map[string]string) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return m
	})
}

// genBaseline generates random baselines
func genBaseline() gopter.Gen {
	return gopter.CombineGens(
		genIdentifier(),                    // name
		gen.Identifier(),                   // executionID
		gen.Identifier(),                   // configHash
		genConfigValues(),                  // configValues
		gen.Identifier(),                   // command
	).Map(func(vals []interface{}) Baseline {
		return Baseline{
			Name:         vals[0].(string),
			ExecutionID:  "sha256:" + vals[1].(string),
			ConfigHash:   "sha256:" + vals[2].(string),
			ConfigValues: vals[3].(map[string]string),
			Command:      vals[4].(string),
			Timestamp:    time.Now().UTC().Truncate(time.Second),
		}
	})
}

// TestBaselineRoundTrip tests Property 1: Baseline Round-Trip
// For any valid baseline, saving and loading should preserve all fields.
// Validates: Requirements 1.1, 1.2
func TestBaselineRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("save then load preserves baseline", prop.ForAll(
		func(b Baseline) bool {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "baseline-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			store := NewStore(tmpDir)

			// Save baseline
			if err := store.Save(b); err != nil {
				return false
			}

			// Load baseline
			loaded, err := store.Load(b.Name)
			if err != nil {
				return false
			}

			// Compare fields
			if loaded.Name != b.Name {
				return false
			}
			if loaded.ExecutionID != b.ExecutionID {
				return false
			}
			if loaded.ConfigHash != b.ConfigHash {
				return false
			}
			if loaded.Command != b.Command {
				return false
			}
			if len(loaded.ConfigValues) != len(b.ConfigValues) {
				return false
			}
			for k, v := range b.ConfigValues {
				if loaded.ConfigValues[k] != v {
					return false
				}
			}
			// Timestamps should be equal (within JSON serialization precision)
			if !loaded.Timestamp.Equal(b.Timestamp) {
				return false
			}

			return true
		},
		genBaseline(),
	))

	properties.TestingRun(t)
}

// TestResolveDirRespectsEnvVar tests Property 2: Baseline Directory Configuration
// For any environment with ADMIT_BASELINE_DIR set, that directory should be used.
// Validates: Requirements 1.3, 1.4
func TestResolveDirRespectsEnvVar(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("ResolveDir uses ADMIT_BASELINE_DIR when set", prop.ForAll(
		func(customDir string) bool {
			environ := []string{"ADMIT_BASELINE_DIR=" + customDir}
			resolved := ResolveDir(environ)
			return resolved == customDir
		},
		gen.Identifier().Map(func(s string) string {
			return "/custom/" + s
		}),
	))

	properties.Property("ResolveDir uses default when env var not set", prop.ForAll(
		func(otherVar string) bool {
			environ := []string{"OTHER_VAR=" + otherVar}
			resolved := ResolveDir(environ)
			return resolved == DefaultDir()
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestMultipleNamedBaselines tests Property 3: Multiple Named Baselines
// For any set of baselines with distinct names, they should be stored separately.
// Validates: Requirements 1.5
func TestMultipleNamedBaselines(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("multiple baselines with different names are stored separately", prop.ForAll(
		func(name1, name2 string) bool {
			if name1 == name2 {
				return true // Skip if names are the same
			}

			tmpDir, err := os.MkdirTemp("", "baseline-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			store := NewStore(tmpDir)

			b1 := Baseline{
				Name:         name1,
				ExecutionID:  "sha256:exec1",
				ConfigHash:   "sha256:hash1",
				ConfigValues: map[string]string{"key1": "value1"},
				Command:      "cmd1",
				Timestamp:    time.Now().UTC(),
			}

			b2 := Baseline{
				Name:         name2,
				ExecutionID:  "sha256:exec2",
				ConfigHash:   "sha256:hash2",
				ConfigValues: map[string]string{"key2": "value2"},
				Command:      "cmd2",
				Timestamp:    time.Now().UTC(),
			}

			// Save both
			if err := store.Save(b1); err != nil {
				return false
			}
			if err := store.Save(b2); err != nil {
				return false
			}

			// Load and verify each
			loaded1, err := store.Load(name1)
			if err != nil {
				return false
			}
			loaded2, err := store.Load(name2)
			if err != nil {
				return false
			}

			// Verify they are different
			return loaded1.ConfigHash == "sha256:hash1" && loaded2.ConfigHash == "sha256:hash2"
		},
		genIdentifier(),
		genIdentifier(),
	))

	properties.TestingRun(t)
}

// TestBaselineListAndDelete tests Property 9: Baseline List and Delete
// For any set of saved baselines, list returns all, delete removes specific one.
// Validates: Requirements 4.1, 4.3
func TestBaselineListAndDelete(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("list returns all saved baselines, delete removes specific one", prop.ForAll(
		func(name string) bool {
			tmpDir, err := os.MkdirTemp("", "baseline-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			store := NewStore(tmpDir)

			b := Baseline{
				Name:         name,
				ExecutionID:  "sha256:exec",
				ConfigHash:   "sha256:hash",
				ConfigValues: map[string]string{},
				Command:      "cmd",
				Timestamp:    time.Now().UTC(),
			}

			// Save baseline
			if err := store.Save(b); err != nil {
				return false
			}

			// List should contain it
			summaries, err := store.List()
			if err != nil {
				return false
			}
			found := false
			for _, s := range summaries {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				return false
			}

			// Delete it
			if err := store.Delete(name); err != nil {
				return false
			}

			// Should no longer exist
			if store.Exists(name) {
				return false
			}

			// List should not contain it
			summaries, err = store.List()
			if err != nil {
				return false
			}
			for _, s := range summaries {
				if s.Name == name {
					return false
				}
			}

			return true
		},
		genIdentifier(),
	))

	properties.TestingRun(t)
}

// TestDefaultDir tests that DefaultDir returns expected path
func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Error("DefaultDir returned empty string")
	}
	if !filepath.IsAbs(dir) && dir != ".admit/baselines" {
		t.Errorf("DefaultDir returned unexpected path: %s", dir)
	}
}

// TestLoadNotFound tests that Load returns ErrBaselineNotFound for missing baseline
func TestLoadNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "baseline-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_, err = store.Load("nonexistent")
	if err != ErrBaselineNotFound {
		t.Errorf("expected ErrBaselineNotFound, got %v", err)
	}
}

// TestDeleteNotFound tests that Delete returns ErrBaselineNotFound for missing baseline
func TestDeleteNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "baseline-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	err = store.Delete("nonexistent")
	if err != ErrBaselineNotFound {
		t.Errorf("expected ErrBaselineNotFound, got %v", err)
	}
}
