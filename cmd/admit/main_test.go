package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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


// Feature: admit-v2-invariants, Property 7: Invariant Execution Gate
// Validates: Requirements 4.1, 4.2, 4.7
// For any schema with invariants and resolved config values:
// - IF any invariant evaluates to false, THEN the target command SHALL NOT execute AND exit code SHALL be non-zero (2)
// - IF all invariants evaluate to true, THEN execution SHALL proceed normally
func TestInvariantExecutionGate_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: passing invariants allow execution (exit code 0 or exec error)
	properties.Property("passing invariants allow execution", prop.ForAll(
		func(dbEnv string) bool {
			// Create a temporary directory with a schema containing invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema with an invariant that will PASS
			// The invariant checks: execution.env == "prod" => db.url.env == "prod"
			// If execution.env is NOT "prod", the invariant passes (antecedent is false)
			// If execution.env IS "prod" and db.url.env IS "prod", the invariant passes
			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set up environment where invariant PASSES
			// Either: ADMIT_ENV != "prod" (antecedent false)
			// Or: ADMIT_ENV == "prod" AND DB_URL_ENV == "prod" (both true)
			environ := []string{
				"DB_URL_ENV=" + dbEnv,
				"ADMIT_ENV=" + dbEnv, // Same value ensures invariant passes
			}

			// Run with a command that will succeed
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should NOT be 2 (invariant violation)
			// It could be 0 (success) or 127 (command not found) depending on system
			return exitCode != 2
		},
		gen.OneConstOf("prod", "staging", "dev"),
	))

	// Property: failing invariants block execution with exit code 2
	properties.Property("failing invariants block execution with exit code 2", prop.ForAll(
		func(nonProdEnv string) bool {
			// Create a temporary directory with a schema containing invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema with an invariant that will FAIL
			// The invariant checks: execution.env == "prod" => db.url.env == "prod"
			// If execution.env IS "prod" but db.url.env is NOT "prod", the invariant fails
			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set up environment where invariant FAILS
			// ADMIT_ENV == "prod" but DB_URL_ENV != "prod"
			environ := []string{
				"DB_URL_ENV=" + nonProdEnv, // Non-prod value
				"ADMIT_ENV=prod",            // Prod environment
			}

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code MUST be 2 (invariant violation)
			return exitCode == 2
		},
		gen.OneConstOf("staging", "dev", "test", "local"),
	))

	// Property: command is NOT executed when invariant fails
	properties.Property("command is NOT executed when invariant fails", prop.ForAll(
		func(nonProdEnv string) bool {
			// Create a temporary directory with a schema containing invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema with an invariant that will FAIL
			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Create a marker file that the command would create if executed
			markerFile := filepath.Join(tmpDir, "executed.marker")

			// Set up environment where invariant FAILS
			environ := []string{
				"DB_URL_ENV=" + nonProdEnv,
				"ADMIT_ENV=prod",
			}

			// Run with a command that would create a marker file
			args := []string{"run", "touch", markerFile}
			_ = run(args, environ, tmpDir)

			// The marker file should NOT exist (command was not executed)
			_, err = os.Stat(markerFile)
			return os.IsNotExist(err)
		},
		gen.OneConstOf("staging", "dev", "test", "local"),
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 8: All Violations Reported
// Validates: Requirements 4.6
// For any set of invariants where multiple fail, the error output SHALL contain
// violation information for ALL failed invariants, not just the first.
func TestAllViolationsReported_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: all failing invariants are reported in output
	properties.Property("all failing invariants are reported in output", prop.ForAll(
		func(numInvariants int) bool {
			// Ensure we have at least 2 invariants
			if numInvariants < 2 {
				numInvariants = 2
			}
			if numInvariants > 5 {
				numInvariants = 5
			}

			// Create a temporary directory with a schema containing multiple invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Build schema with multiple invariants that will ALL fail
			// Each invariant checks: execution.env == "prod" => config_N == "prod"
			// With ADMIT_ENV=prod and config_N=staging, all will fail
			var configSection strings.Builder
			var invariantsSection strings.Builder
			var envVars []string

			configSection.WriteString("config:\n")
			invariantsSection.WriteString("invariants:\n")

			invariantNames := make([]string, numInvariants)
			for i := 0; i < numInvariants; i++ {
				configKey := fmt.Sprintf("config_%d", i)
				envKey := fmt.Sprintf("CONFIG_%d", i)
				invName := fmt.Sprintf("guard-%d", i)
				invariantNames[i] = invName

				configSection.WriteString(fmt.Sprintf("  %s:\n    type: string\n    required: true\n", configKey))
				invariantsSection.WriteString(fmt.Sprintf("  - name: %s\n    rule: execution.env == \"prod\" => %s == \"prod\"\n", invName, configKey))
				envVars = append(envVars, fmt.Sprintf("%s=staging", envKey)) // Non-prod value to cause failure
			}

			schemaContent := configSection.String() + "\n" + invariantsSection.String()
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set ADMIT_ENV=prod to trigger invariant checks
			envVars = append(envVars, "ADMIT_ENV=prod")

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Logf("Failed to create pipe: %v", err)
				return true // Skip on setup failure
			}
			os.Stderr = w

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, envVars, tmpDir)

			// Restore stderr and read captured output
			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Exit code should be 2 (invariant violation)
			if exitCode != 2 {
				t.Logf("Expected exit code 2, got %d", exitCode)
				return false
			}

			// Check that ALL invariant names appear in the output
			output := buf.String()
			for _, name := range invariantNames {
				if !strings.Contains(output, name) {
					t.Logf("Missing invariant '%s' in output: %s", name, output)
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 5),
	))

	// Property: JSON output contains all violations
	properties.Property("JSON output contains all violations", prop.ForAll(
		func(numInvariants int) bool {
			// Ensure we have at least 2 invariants
			if numInvariants < 2 {
				numInvariants = 2
			}
			if numInvariants > 5 {
				numInvariants = 5
			}

			// Create a temporary directory with a schema containing multiple invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Build schema with multiple invariants that will ALL fail
			var configSection strings.Builder
			var invariantsSection strings.Builder
			var envVars []string

			configSection.WriteString("config:\n")
			invariantsSection.WriteString("invariants:\n")

			invariantNames := make([]string, numInvariants)
			for i := 0; i < numInvariants; i++ {
				configKey := fmt.Sprintf("config_%d", i)
				envKey := fmt.Sprintf("CONFIG_%d", i)
				invName := fmt.Sprintf("guard-%d", i)
				invariantNames[i] = invName

				configSection.WriteString(fmt.Sprintf("  %s:\n    type: string\n    required: true\n", configKey))
				invariantsSection.WriteString(fmt.Sprintf("  - name: %s\n    rule: execution.env == \"prod\" => %s == \"prod\"\n", invName, configKey))
				envVars = append(envVars, fmt.Sprintf("%s=staging", envKey)) // Non-prod value to cause failure
			}

			schemaContent := configSection.String() + "\n" + invariantsSection.String()
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set ADMIT_ENV=prod to trigger invariant checks
			envVars = append(envVars, "ADMIT_ENV=prod")

			// Capture stdout for JSON output
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Logf("Failed to create pipe: %v", err)
				return true // Skip on setup failure
			}
			os.Stdout = w

			// Run with --invariants-json flag
			args := []string{"run", "--invariants-json", "true"}
			exitCode := run(args, envVars, tmpDir)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Exit code should be 2 (invariant violation)
			if exitCode != 2 {
				t.Logf("Expected exit code 2, got %d", exitCode)
				return false
			}

			// Check that JSON output contains all invariant names
			output := buf.String()
			for _, name := range invariantNames {
				if !strings.Contains(output, name) {
					t.Logf("Missing invariant '%s' in JSON output: %s", name, output)
					return false
				}
			}

			// Check that failedCount matches number of invariants
			expectedCount := fmt.Sprintf(`"failedCount": %d`, numInvariants)
			if !strings.Contains(output, expectedCount) {
				t.Logf("Expected failedCount %d in JSON output: %s", numInvariants, output)
				return false
			}

			return true
		},
		gen.IntRange(2, 5),
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 11: Backward Compatibility - No Invariants
// Validates: Requirements 6.1, 6.4
// For any schema without an `invariants` section (or with empty invariants),
// the Admit_CLI SHALL skip invariant evaluation and proceed with normal v1 execution flow.
func TestBackwardCompatibilityNoInvariants_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: schema without invariants section proceeds normally
	properties.Property("schema without invariants section proceeds normally", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "postgres://localhost/test"
			}

			// Create a temporary directory with a schema WITHOUT invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema WITHOUT invariants section (v1 style)
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

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should NOT be 2 (no invariant violation)
			// It should be 0 (success) or possibly 127 (command not found)
			return exitCode != 2
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.OneConstOf("test", "live"),
	))

	// Property: schema with empty invariants section proceeds normally
	properties.Property("schema with empty invariants section proceeds normally", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "postgres://localhost/test"
			}

			// Create a temporary directory with a schema with EMPTY invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema with empty invariants section
			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true

invariants: []
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

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should NOT be 2 (no invariant violation)
			return exitCode != 2
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.OneConstOf("test", "live"),
	))

	// Property: v1 validation still works without invariants
	properties.Property("v1 validation still works without invariants", prop.ForAll(
		func(invalidMode string) bool {
			// Create a temporary directory with a schema WITHOUT invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema WITHOUT invariants section
			schemaContent := `config:
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

			// Set up INVALID environment (invalid enum value)
			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should be 1 (v1 validation error), NOT 2 (invariant violation)
			return exitCode == 1
		},
		gen.AnyString().SuchThat(func(s string) bool {
			// Generate invalid enum values (not "test" or "live")
			return s != "test" && s != "live" && len(s) > 0
		}),
	))

	// Property: invariants are evaluated AFTER v1 validation
	properties.Property("invariants are evaluated AFTER v1 validation", prop.ForAll(
		func(invalidMode string) bool {
			// Create a temporary directory with a schema WITH invariants
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Write a schema WITH invariants
			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true

invariants:
  - name: test-guard
    rule: execution.env == "prod" => payments.mode == "live"
`
			schemaPath := filepath.Join(tmpDir, "admit.yaml")
			if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
				t.Logf("Failed to write schema: %v", err)
				return true // Skip on setup failure
			}

			// Set up INVALID environment (invalid enum value)
			// This should fail v1 validation BEFORE invariants are checked
			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
				"ADMIT_ENV=prod",
			}

			// Run with a command
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should be 1 (v1 validation error), NOT 2 (invariant violation)
			// Because v1 validation happens BEFORE invariant evaluation
			return exitCode == 1
		},
		gen.AnyString().SuchThat(func(s string) bool {
			// Generate invalid enum values (not "test" or "live")
			return s != "test" && s != "live" && len(s) > 0
		}),
	))

	properties.TestingRun(t)
}
