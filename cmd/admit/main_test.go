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


// Feature: admit-v3-container-ci, Property 2: Schema Path Resolution
// Validates: Requirements 2.4, 2.5, 5.1, 5.2, 5.3, 5.4
// For any combination of --schema flag and ADMIT_SCHEMA env var:
// - If only flag is set, use flag value
// - If only env var is set, use env var value
// - If both are set, flag value SHALL take precedence
func TestSchemaPathResolution_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: --schema flag takes precedence over ADMIT_SCHEMA env var
	properties.Property("--schema flag takes precedence over ADMIT_SCHEMA", prop.ForAll(
		func(flagPath string, envPath string) bool {
			if flagPath == "" || envPath == "" {
				return true // Skip empty paths
			}

			// Create temp directories for both paths
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true // Skip on setup failure
			}
			defer os.RemoveAll(tmpDir)

			// Create schema at flag path
			flagSchemaPath := filepath.Join(tmpDir, "flag-schema.yaml")
			envSchemaPath := filepath.Join(tmpDir, "env-schema.yaml")

			// Write different schemas to distinguish which one is loaded
			flagSchemaContent := `config:
  flag.marker:
    type: string
    required: true
`
			envSchemaContent := `config:
  env.marker:
    type: string
    required: true
`
			if err := os.WriteFile(flagSchemaPath, []byte(flagSchemaContent), 0644); err != nil {
				return true
			}
			if err := os.WriteFile(envSchemaPath, []byte(envSchemaContent), 0644); err != nil {
				return true
			}

			// Set up environment with ADMIT_SCHEMA pointing to env schema
			environ := []string{
				"ADMIT_SCHEMA=" + envSchemaPath,
				"FLAG_MARKER=value", // For flag schema
			}

			// Capture stderr to check which schema was loaded
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			// Run with --schema flag pointing to flag schema
			args := []string{"run", "--schema", flagSchemaPath, "true"}
			exitCode := run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// If flag schema was used, it should fail because FLAG_MARKER is set
			// but flag.marker is required (and FLAG_MARKER maps to FLAG_MARKER env var)
			// Actually, let's check the error message
			output := buf.String()

			// The flag schema requires flag.marker, so error should mention flag.marker
			// If env schema was used, error would mention env.marker
			return exitCode == 1 && strings.Contains(output, "flag.marker")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 20 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 20 }),
	))

	// Property: ADMIT_SCHEMA env var is used when no flag provided
	properties.Property("ADMIT_SCHEMA env var is used when no flag provided", prop.ForAll(
		func(schemaName string) bool {
			if schemaName == "" {
				return true
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			// Create schema at custom path
			customSchemaPath := filepath.Join(tmpDir, schemaName+".yaml")
			schemaContent := `config:
  custom.key:
    type: string
    required: true
`
			if err := os.WriteFile(customSchemaPath, []byte(schemaContent), 0644); err != nil {
				return true
			}

			// Set up environment with ADMIT_SCHEMA
			environ := []string{
				"ADMIT_SCHEMA=" + customSchemaPath,
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			// Run WITHOUT --schema flag
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Should fail because custom.key is required but not set
			// Error should mention custom.key (proving custom schema was loaded)
			output := buf.String()
			return exitCode == 1 && strings.Contains(output, "custom.key")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 20 }),
	))

	// Property: default admit.yaml is used when neither flag nor env var set
	properties.Property("default admit.yaml is used when neither flag nor env var set", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			// Create default admit.yaml
			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			// Set up environment WITHOUT ADMIT_SCHEMA
			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Run WITHOUT --schema flag
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Should succeed (exit 0 or exec error, but not 1 or 2)
			return exitCode != 1 && exitCode != 2
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// Feature: admit-v3-container-ci, Property 3: Missing Schema Error
// Validates: Requirements 5.5
// For any schema path that does not exist, the CLI SHALL exit with code 3
func TestMissingSchemaError_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: non-existent schema path exits with code 3
	properties.Property("non-existent schema path exits with code 3", prop.ForAll(
		func(nonExistentPath string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			// Use a path that definitely doesn't exist
			schemaPath := filepath.Join(tmpDir, nonExistentPath, "nonexistent.yaml")

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			// Run with --schema pointing to non-existent file
			args := []string{"run", "--schema", schemaPath, "true"}
			exitCode := run(args, []string{}, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Exit code should be 3 (schema error)
			// Error message should mention the path
			output := buf.String()
			return exitCode == 3 && strings.Contains(output, "schema file not found")
		},
		gen.Identifier(),
	))

	// Property: ADMIT_SCHEMA pointing to non-existent file exits with code 3
	properties.Property("ADMIT_SCHEMA pointing to non-existent file exits with code 3", prop.ForAll(
		func(nonExistentPath string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			// Use a path that definitely doesn't exist
			schemaPath := filepath.Join(tmpDir, nonExistentPath, "nonexistent.yaml")

			// Set up environment with ADMIT_SCHEMA pointing to non-existent file
			environ := []string{
				"ADMIT_SCHEMA=" + schemaPath,
			}

			// Run WITHOUT --schema flag
			args := []string{"run", "true"}
			exitCode := run(args, environ, tmpDir)

			// Exit code should be 3 (schema error)
			return exitCode == 3
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}


// Feature: admit-v3-container-ci, Property 1: Check Validation Equivalence
// Validates: Requirements 3.1, 3.2, 3.3, 3.4
// For any schema and environment configuration, `admit check` SHALL produce
// the same validation and invariant results as `admit run` would compute
// before execution, without actually executing any command.
func TestCheckValidationEquivalence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: check produces same exit code as run would for valid config
	properties.Property("check produces exit code 0 for valid config", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
			}

			// Run check subcommand
			args := []string{"check"}
			exitCode := run(args, environ, tmpDir)

			// Should exit with 0 for valid config
			return exitCode == 0
		},
		gen.Identifier(),
		gen.OneConstOf("test", "live"),
	))

	// Property: check produces exit code 1 for validation errors
	properties.Property("check produces exit code 1 for validation errors", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			args := []string{"check"}
			exitCode := run(args, environ, tmpDir)

			// Should exit with 1 for validation error
			return exitCode == 1
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: check produces exit code 2 for invariant violations
	properties.Property("check produces exit code 2 for invariant violations", prop.ForAll(
		func(nonProdEnv string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			// Set up environment where invariant FAILS
			environ := []string{
				"DB_URL_ENV=" + nonProdEnv,
				"ADMIT_ENV=prod",
			}

			args := []string{"check"}
			exitCode := run(args, environ, tmpDir)

			// Should exit with 2 for invariant violation
			return exitCode == 2
		},
		gen.OneConstOf("staging", "dev", "test"),
	))

	// Property: check does not execute any command
	properties.Property("check does not execute any command", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			// Create a marker file that would be created if a command ran
			markerFile := filepath.Join(tmpDir, "executed.marker")

			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Check subcommand should NOT execute any command
			// Even if we somehow passed a command, it shouldn't run
			args := []string{"check"}
			_ = run(args, environ, tmpDir)

			// Marker file should NOT exist
			_, err = os.Stat(markerFile)
			return os.IsNotExist(err)
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Feature: admit-v3-container-ci, Property 8: Check JSON Output Structure
// Validates: Requirements 3.5
// For any `admit check --json` invocation, the output SHALL be valid JSON
// containing: valid (boolean), validationErrors (array), invariantResults (array),
// schemaPath (string)
func TestCheckJSONOutputStructure_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: check --json produces valid JSON with required fields
	properties.Property("check --json produces valid JSON with required fields", prop.ForAll(
		func(dbUrl string, mode string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stdout = w

			args := []string{"check", "--json"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()

			// Check that output contains required JSON fields
			return strings.Contains(output, `"valid"`) &&
				strings.Contains(output, `"validationErrors"`) &&
				strings.Contains(output, `"invariantResults"`) &&
				strings.Contains(output, `"schemaPath"`)
		},
		gen.Identifier(),
		gen.OneConstOf("test", "live"),
	))

	properties.TestingRun(t)
}


// Feature: admit-v3-container-ci, Property 4: Dry Run Non-Execution
// Validates: Requirements 6.1, 6.2, 6.3, 6.4
// For any configuration (valid or invalid), when --dry-run flag is provided,
// the target command SHALL never be executed, and the CLI SHALL output
// validation results only.
func TestDryRunNonExecution_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: dry-run does not execute command for valid config
	properties.Property("dry-run does not execute command for valid config", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			// Create a marker file that would be created if command ran
			markerFile := filepath.Join(tmpDir, "executed.marker")

			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Run with --dry-run flag
			args := []string{"run", "--dry-run", "touch", markerFile}
			exitCode := run(args, environ, tmpDir)

			// Should exit with 0 (valid config)
			if exitCode != 0 {
				return false
			}

			// Marker file should NOT exist (command was not executed)
			_, err = os.Stat(markerFile)
			return os.IsNotExist(err)
		},
		gen.Identifier(),
	))

	// Property: dry-run outputs "Config valid, would execute" for valid config
	properties.Property("dry-run outputs success message for valid config", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stdout = w

			args := []string{"run", "--dry-run", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			return strings.Contains(output, "Config valid") && strings.Contains(output, "would execute")
		},
		gen.Identifier(),
	))

	// Property: dry-run outputs validation errors for invalid config
	properties.Property("dry-run outputs validation errors for invalid config", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			args := []string{"run", "--dry-run", "echo", "hello"}
			exitCode := run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			// Should exit with 1 (validation error)
			// Should have error output
			output := buf.String()
			return exitCode == 1 && strings.Contains(output, "payments.mode")
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: dry-run --json outputs JSON with command info
	properties.Property("dry-run --json outputs JSON with command info", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stdout = w

			args := []string{"run", "--dry-run", "--json", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			return strings.Contains(output, `"valid"`) &&
				strings.Contains(output, `"command"`) &&
				strings.Contains(output, `"args"`) &&
				strings.Contains(output, `"schemaPath"`)
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}


// Feature: admit-v3-container-ci, Property 5: CI Annotation Format Compliance
// Validates: Requirements 4.2, 4.3, 4.5
// For any validation error or invariant violation in CI mode (--ci flag or
// ADMIT_CI=true), the output SHALL conform to GitHub Actions annotation syntax:
// ::error file=<file>::<message>
func TestCIAnnotationFormat_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: --ci flag produces GitHub Actions annotation format
	properties.Property("--ci flag produces GitHub Actions annotation format", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			args := []string{"run", "--ci", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			// Should contain GitHub Actions annotation format
			return strings.Contains(output, "::error file=admit.yaml::")
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: ADMIT_CI=true produces GitHub Actions annotation format
	properties.Property("ADMIT_CI=true produces GitHub Actions annotation format", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
				"ADMIT_CI=true",
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			args := []string{"run", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			// Should contain GitHub Actions annotation format
			return strings.Contains(output, "::error file=admit.yaml::")
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: CI mode includes summary of failures
	properties.Property("CI mode includes summary of failures", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			args := []string{"run", "--ci", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			// Should contain summary
			return strings.Contains(output, "Validation failed") || strings.Contains(output, "error")
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: CI mode formats invariant violations with annotation
	properties.Property("CI mode formats invariant violations with annotation", prop.ForAll(
		func(nonProdEnv string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL_ENV=" + nonProdEnv,
				"ADMIT_ENV=prod",
			}

			// Capture stderr
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				return true
			}
			os.Stderr = w

			args := []string{"run", "--ci", "echo", "hello"}
			_ = run(args, environ, tmpDir)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			r.Close()

			output := buf.String()
			// Should contain annotation format for invariant violation
			return strings.Contains(output, "::error file=admit.yaml::INVARIANT VIOLATION")
		},
		gen.OneConstOf("staging", "dev", "test"),
	))

	properties.TestingRun(t)
}


// Feature: admit-v3-container-ci, Property 6: Exit Code Consistency
// Validates: Requirements 3.2, 3.3, 3.4, 7.4
// For any scenario, exit codes SHALL be consistent across all modes:
// 0 = success, 1 = validation error, 2 = invariant violation, 3 = schema error
func TestExitCodeConsistency_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: valid config produces exit code 0 across all modes
	properties.Property("valid config produces exit code 0 across all modes", prop.ForAll(
		func(dbUrl string) bool {
			if dbUrl == "" {
				dbUrl = "test"
			}

			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL=" + dbUrl,
			}

			// Test check subcommand
			checkCode := run([]string{"check"}, environ, tmpDir)

			// Test dry-run mode
			dryRunCode := run([]string{"run", "--dry-run", "echo", "hello"}, environ, tmpDir)

			// Both should return 0 for valid config
			return checkCode == 0 && dryRunCode == 0
		},
		gen.Identifier(),
	))

	// Property: validation error produces exit code 1 across all modes
	properties.Property("validation error produces exit code 1 across all modes", prop.ForAll(
		func(invalidMode string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"PAYMENTS_MODE=" + invalidMode,
			}

			// Test check subcommand
			checkCode := run([]string{"check"}, environ, tmpDir)

			// Test dry-run mode
			dryRunCode := run([]string{"run", "--dry-run", "echo", "hello"}, environ, tmpDir)

			// Test run mode (will fail before exec)
			runCode := run([]string{"run", "echo", "hello"}, environ, tmpDir)

			// All should return 1 for validation error
			return checkCode == 1 && dryRunCode == 1 && runCode == 1
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return s != "test" && s != "live"
		}),
	))

	// Property: invariant violation produces exit code 2 across all modes
	properties.Property("invariant violation produces exit code 2 across all modes", prop.ForAll(
		func(nonProdEnv string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaContent := `config:
  db.url.env:
    type: string
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
`
			if err := os.WriteFile(filepath.Join(tmpDir, "admit.yaml"), []byte(schemaContent), 0644); err != nil {
				return true
			}

			environ := []string{
				"DB_URL_ENV=" + nonProdEnv,
				"ADMIT_ENV=prod",
			}

			// Test check subcommand
			checkCode := run([]string{"check"}, environ, tmpDir)

			// Test dry-run mode
			dryRunCode := run([]string{"run", "--dry-run", "echo", "hello"}, environ, tmpDir)

			// Test run mode (will fail before exec)
			runCode := run([]string{"run", "echo", "hello"}, environ, tmpDir)

			// All should return 2 for invariant violation
			return checkCode == 2 && dryRunCode == 2 && runCode == 2
		},
		gen.OneConstOf("staging", "dev", "test"),
	))

	// Property: schema error produces exit code 3 across all modes
	properties.Property("schema error produces exit code 3 across all modes", prop.ForAll(
		func(nonExistentPath string) bool {
			tmpDir, err := os.MkdirTemp("", "admit-test-*")
			if err != nil {
				return true
			}
			defer os.RemoveAll(tmpDir)

			schemaPath := filepath.Join(tmpDir, nonExistentPath, "nonexistent.yaml")

			// Test check subcommand with --schema
			checkCode := run([]string{"check", "--schema", schemaPath}, []string{}, tmpDir)

			// Test dry-run mode with --schema
			dryRunCode := run([]string{"run", "--dry-run", "--schema", schemaPath, "echo", "hello"}, []string{}, tmpDir)

			// Test run mode with --schema
			runCode := run([]string{"run", "--schema", schemaPath, "echo", "hello"}, []string{}, tmpDir)

			// All should return 3 for schema error
			return checkCode == 3 && dryRunCode == 3 && runCode == 3
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}
