package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-cli, Property 14: Silent Success
// Validates: Requirements 6.4
// For any valid configuration, admit SHALL produce no output to stdout
// before executing the target command.
func TestRun_SilentSuccess_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: valid config produces no stdout output
	properties.Property("valid config produces no stdout output", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "postgres://localhost/test"
			}

			// Create a temporary directory with a valid schema
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a valid schema file
			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set up valid environment
			environ := []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Logf("Failed to create pipe: %v", err)
				return true // Skip on setup failure
			}
			os.Stdout = w

			// Run with a command that will fail to exec (we just want to check stdout before exec)
			// Using a non-existent command so exec fails but validation passes
			args := []string{"run", "true"}

			// Call run function - it will try to exec and fail, but we're testing stdout before that
			_ = run(args, environ, tmpDir)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// For valid config, stdout should be empty (silent success)
			// Note: errors go to stderr, not stdout
			return buf.Len() == 0
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.OneConstOf("test", "live"),
	))

	properties.TestingRun(t)
}

// TestRun_InvalidConfig_PrintsToStderr verifies that invalid configs print errors to stderr
func TestRun_InvalidConfig_PrintsToStderr(t *testing.T) {
	// Create a temporary directory with a valid schema
	tmpDir, err := os.MkdirTemp("", "admit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a schema requiring db.url
	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	schemaPath := filepath.Join(tmpDir, "admit.yaml")
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("Failed to write schema: %v", err)
	}

	// Empty environment - missing required DB_URL
	environ := []string{}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stderr = w

	args := []string{"run", "echo", "hello"}
	exitCode := run(args, environ, tmpDir)

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	// Should exit non-zero
	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code for invalid config, got 0")
	}

	// Should have error output on stderr
	if buf.Len() == 0 {
		t.Errorf("Expected error output on stderr for invalid config")
	}

	// Error should mention the missing key
	if !bytes.Contains(buf.Bytes(), []byte("db.url")) {
		t.Errorf("Expected error to mention 'db.url', got: %s", buf.String())
	}
}


// Feature: admit-v1-config-artifact, Property 12: Backward Compatibility
// Validates: Requirements 6.1
// For any execution without new flags, the behavior SHALL be identical to v0
// (no artifact output, no injection, no identity output).
func TestBackwardCompatibility_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("no flags produces no artifact output", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "postgres://localhost/test"
			}

			// Create a temporary directory with a valid schema
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a valid schema file
			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				return true // Skip on setup failure
			}

			// Set up valid environment
			environ := []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				return true // Skip on setup failure
			}
			os.Stdout = w

			// Run WITHOUT any new flags
			args := []string{"run", "true"}
			_ = run(args, environ, tmpDir)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Without flags, stdout should be empty (backward compatible)
			return buf.Len() == 0
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.OneConstOf("test", "live"),
	))

	properties.Property("no artifact files created without flags", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "postgres://localhost/test"
			}

			// Create a temporary directory with a valid schema
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a valid schema file
			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				return true // Skip on setup failure
			}

			// Set up valid environment
			environ := []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
			}

			// Run WITHOUT any new flags
			args := []string{"run", "true"}
			_ = run(args, environ, tmpDir)

			// Check no artifact.json or identity.json files were created
			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				return true // Skip on error
			}

			for _, entry := range entries {
				name := entry.Name()
				if name != "admit.yaml" {
					// Unexpected file created
					return false
				}
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.OneConstOf("test", "live"),
	))

	properties.TestingRun(t)
}

// Feature: admit-v1-config-artifact, Property 13: Validation Before Artifacts
// Validates: Requirements 6.2, 6.3
// For any invalid configuration, no artifacts SHALL be generated, no injection
// SHALL occur, and no identity SHALL be computed, regardless of flags provided.
func TestValidationBeforeArtifacts_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("invalid config produces no artifact files even with flags", prop.ForAll(
		func(invalidMode string) bool {
			// Create a temporary directory with a valid schema
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema requiring enum value
			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				return true // Skip on setup failure
			}

			// Set up INVALID environment (invalid enum value)
			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			artifactPath := filepath.Join(tmpDir, "artifact.json")
			identityPath := filepath.Join(tmpDir, "identity.json")
			injectPath := filepath.Join(tmpDir, "inject.json")

			// Run WITH artifact and identity flags
			args := []string{
				"run",
				"--artifact-file", artifactPath,
				"--identity-file", identityPath,
				"--inject-file", injectPath,
				"true",
			}

			exitCode := run(args, environ, tmpDir)

			// Should exit non-zero
			if exitCode == 0 {
				return false
			}

			// No artifact files should be created
			if _, err := os.Stat(artifactPath); err == nil {
				return false // File exists, but shouldn't
			}
			if _, err := os.Stat(identityPath); err == nil {
				return false // File exists, but shouldn't
			}
			if _, err := os.Stat(injectPath); err == nil {
				return false // File exists, but shouldn't
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool {
			// Generate invalid enum values (not "test" or "live")
			return s != "test" && s != "live" && len(s) > 0
		}),
	))

	properties.TestingRun(t)
}
