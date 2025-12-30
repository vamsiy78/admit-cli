package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Helper to build the admit binary for integration tests
func buildAdmitBinary(t *testing.T) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "admit-bin-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	binPath := filepath.Join(tmpDir, "admit")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build admit binary: %v\nOutput: %s", err, output)
	}

	return binPath
}

// Helper to create a temp directory with a schema file
func createTestSchema(t *testing.T, schemaContent string) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "admit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	schemaPath := filepath.Join(tmpDir, "admit.yaml")
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("Failed to write schema: %v", err)
	}

	return tmpDir
}

// Feature: admit-cli, Property 9: Execution Gate Invariant
// Validates: Requirements 5.1, 5.3
// For any schema and environment:
// - If validation fails, the target command SHALL NOT execute
// - If validation succeeds, the target command SHALL execute
func TestExecutionGateInvariant_Property(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: valid config allows command execution
	properties.Property("valid config allows command execution", prop.ForAll(
		func(dbUrl string, modeIdx int) bool {
			modes := []string{"test", "live"}
			mode := modes[modeIdx%len(modes)]

			// Create schema requiring db.url and payments.mode
			schemaContent := `config:
  db.url:
    type: string
    required: true
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			tmpDir := createTestSchema(t, schemaContent)
			defer os.RemoveAll(tmpDir)

			// Create a marker file path to verify command execution
			markerFile := filepath.Join(tmpDir, "executed.marker")

			// Run admit with valid config - use touch to create marker file
			cmd := exec.Command(binPath, "run", "touch", markerFile)
			cmd.Dir = tmpDir
			cmd.Env = []string{
				"DB_URL=" + dbUrl,
				"PAYMENTS_MODE=" + mode,
				"PATH=" + os.Getenv("PATH"),
			}

			err := cmd.Run()

			// Check if marker file was created (command executed)
			_, statErr := os.Stat(markerFile)
			commandExecuted := statErr == nil

			// With valid config, command should execute (marker file should exist)
			// err might be nil or non-nil depending on touch behavior, but marker should exist
			if !commandExecuted {
				t.Logf("Valid config but command did not execute. err=%v, dbUrl=%q, mode=%q", err, dbUrl, mode)
				return false
			}

			return true
		},
		gen.AlphaString().Map(func(s string) string {
			if s == "" {
				return "default_value"
			}
			return s
		}),
		gen.IntRange(0, 1),
	))

	// Property: invalid config blocks command execution
	properties.Property("invalid config blocks command execution", prop.ForAll(
		func(invalidMode string) bool {
			// Create schema requiring payments.mode enum
			schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
			tmpDir := createTestSchema(t, schemaContent)
			defer os.RemoveAll(tmpDir)

			// Create a marker file path to verify command did NOT execute
			markerFile := filepath.Join(tmpDir, "executed.marker")

			// Run admit with invalid config (invalid enum value)
			cmd := exec.Command(binPath, "run", "touch", markerFile)
			cmd.Dir = tmpDir
			cmd.Env = []string{
				"PAYMENTS_MODE=" + invalidMode,
				"PATH=" + os.Getenv("PATH"),
			}

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()

			// Check if marker file was created (command executed)
			_, statErr := os.Stat(markerFile)
			commandExecuted := statErr == nil

			// With invalid config, command should NOT execute
			if commandExecuted {
				t.Logf("Invalid config but command executed! invalidMode=%q", invalidMode)
				return false
			}

			// Should have exited with error
			if err == nil {
				t.Logf("Expected error exit for invalid config, got nil")
				return false
			}

			return true
		},
		// Generate invalid enum values by prefixing with "invalid_"
		gen.AlphaString().Map(func(s string) string {
			return "invalid_" + s
		}),
	))

	// Property: missing required config blocks command execution
	properties.Property("missing required config blocks command execution", prop.ForAll(
		func(randomValue string) bool {
			// Create schema requiring db.url
			schemaContent := `config:
  db.url:
    type: string
    required: true
`
			tmpDir := createTestSchema(t, schemaContent)
			defer os.RemoveAll(tmpDir)

			// Create a marker file path
			markerFile := filepath.Join(tmpDir, "executed.marker")

			// Run admit WITHOUT setting DB_URL (missing required)
			cmd := exec.Command(binPath, "run", "touch", markerFile)
			cmd.Dir = tmpDir
			cmd.Env = []string{
				"SOME_OTHER_VAR=" + randomValue,
				"PATH=" + os.Getenv("PATH"),
			}

			err := cmd.Run()

			// Check if marker file was created
			_, statErr := os.Stat(markerFile)
			commandExecuted := statErr == nil

			// With missing required config, command should NOT execute
			if commandExecuted {
				t.Logf("Missing required config but command executed!")
				return false
			}

			// Should have exited with error
			if err == nil {
				t.Logf("Expected error exit for missing required config")
				return false
			}

			return true
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}


// Feature: admit-cli, Property 10: Environment Passthrough
// Validates: Requirements 5.4
// For any environment variables present when admit runs, those same variables
// SHALL be available to the executed child process.
func TestEnvironmentPassthrough_Property(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: environment variables are passed to child process
	properties.Property("environment variables are passed to child process", prop.ForAll(
		func(suffixIdx int, testVarValue string) bool {
			// Generate a valid env var name using a fixed set of suffixes
			suffixes := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
			testVarName := "TEST_VAR_" + suffixes[suffixIdx%len(suffixes)]

			// Create a minimal schema (no required fields to ensure validation passes)
			schemaContent := `config:
  optional.field:
    type: string
    required: false
`
			tmpDir := createTestSchema(t, schemaContent)
			defer os.RemoveAll(tmpDir)

			// Output file to capture the env var value from child process
			outputFile := filepath.Join(tmpDir, "env_output.txt")

			// Use sh -c to echo the env var to a file
			// This tests that the env var is available in the child process
			shellCmd := "echo $" + testVarName + " > " + outputFile

			cmd := exec.Command(binPath, "run", "sh", "-c", shellCmd)
			cmd.Dir = tmpDir
			cmd.Env = []string{
				testVarName + "=" + testVarValue,
				"PATH=" + os.Getenv("PATH"),
			}

			err := cmd.Run()
			if err != nil {
				t.Logf("Command failed: %v", err)
				return false
			}

			// Read the output file
			content, err := os.ReadFile(outputFile)
			if err != nil {
				t.Logf("Failed to read output file: %v", err)
				return false
			}

			// The output should contain the test value (with trailing newline from echo)
			got := string(bytes.TrimSpace(content))
			if got != testVarValue {
				t.Logf("Environment variable not passed through. Expected %q, got %q", testVarValue, got)
				return false
			}

			return true
		},
		gen.IntRange(0, 9),
		// Generate simple alphanumeric values
		gen.AlphaString().Map(func(s string) string {
			if s == "" {
				return "default_value"
			}
			if len(s) > 30 {
				return s[:30]
			}
			return s
		}),
	))

	properties.TestingRun(t)
}

// Feature: admit-cli, Property 11: Exit Code Propagation
// Validates: Requirements 5.5
// For any exit code returned by the child process, admit SHALL return
// the same exit code to its caller.
func TestExitCodePropagation_Property(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: exit code from child process is propagated
	properties.Property("exit code from child process is propagated", prop.ForAll(
		func(exitCode int) bool {
			// Create a minimal schema
			schemaContent := `config:
  optional.field:
    type: string
    required: false
`
			tmpDir := createTestSchema(t, schemaContent)
			defer os.RemoveAll(tmpDir)

			// Use sh -c "exit N" to exit with specific code
			shellCmd := "exit " + strconv.Itoa(exitCode)

			cmd := exec.Command(binPath, "run", "sh", "-c", shellCmd)
			cmd.Dir = tmpDir
			cmd.Env = []string{
				"PATH=" + os.Getenv("PATH"),
			}

			err := cmd.Run()

			// Get the actual exit code
			var actualExitCode int
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					actualExitCode = exitErr.ExitCode()
				} else {
					t.Logf("Unexpected error type: %v", err)
					return false
				}
			} else {
				actualExitCode = 0
			}

			// The exit code should match
			if actualExitCode != exitCode {
				t.Logf("Exit code not propagated. Expected %d, got %d", exitCode, actualExitCode)
				return false
			}

			return true
		},
		// Generate exit codes in valid range (0-255, but we'll use 0-127 to avoid signal codes)
		gen.IntRange(0, 127),
	))

	properties.TestingRun(t)
}

// TestExecutionGate_ValidConfig_CommandRuns is a simple unit test for the execution gate
func TestExecutionGate_ValidConfig_CommandRuns(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	markerFile := filepath.Join(tmpDir, "executed.marker")

	cmd := exec.Command(binPath, "run", "touch", markerFile)
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Errorf("Expected command to succeed with valid config, got error: %v", err)
	}

	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Errorf("Expected marker file to be created (command should have executed)")
	}
}

// TestExecutionGate_InvalidConfig_CommandBlocked is a simple unit test for the execution gate
func TestExecutionGate_InvalidConfig_CommandBlocked(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	markerFile := filepath.Join(tmpDir, "executed.marker")

	cmd := exec.Command(binPath, "run", "touch", markerFile)
	cmd.Dir = tmpDir
	cmd.Env = []string{
		// Missing DB_URL - required field not set
		"PATH=" + os.Getenv("PATH"),
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should exit with error
	if err == nil {
		t.Errorf("Expected command to fail with invalid config")
	}

	// Marker file should NOT exist
	if _, err := os.Stat(markerFile); err == nil {
		t.Errorf("Marker file should not exist (command should not have executed)")
	}

	// Stderr should contain error message
	if stderr.Len() == 0 {
		t.Errorf("Expected error message on stderr")
	}
}

// TestEnvironmentPassthrough_Simple is a simple unit test for environment passthrough
func TestEnvironmentPassthrough_Simple(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  optional.field:
    type: string
    required: false
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	outputFile := filepath.Join(tmpDir, "env_output.txt")

	cmd := exec.Command(binPath, "run", "sh", "-c", "echo $MY_TEST_VAR > "+outputFile)
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"MY_TEST_VAR=hello_world",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	got := string(bytes.TrimSpace(content))
	if got != "hello_world" {
		t.Errorf("Expected MY_TEST_VAR=hello_world, got %q", got)
	}
}

// TestExitCodePropagation_Simple is a simple unit test for exit code propagation
func TestExitCodePropagation_Simple(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  optional.field:
    type: string
    required: false
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	testCases := []int{0, 1, 42, 127}

	for _, expectedCode := range testCases {
		t.Run("exit_"+strconv.Itoa(expectedCode), func(t *testing.T) {
			cmd := exec.Command(binPath, "run", "sh", "-c", "exit "+strconv.Itoa(expectedCode))
			cmd.Dir = tmpDir
			cmd.Env = []string{
				"PATH=" + os.Getenv("PATH"),
			}

			err := cmd.Run()

			var actualCode int
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					actualCode = exitErr.ExitCode()
				} else {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			if actualCode != expectedCode {
				t.Errorf("Expected exit code %d, got %d", expectedCode, actualCode)
			}
		})
	}
}


// ============================================================================
// V1 Integration Tests - Config Artifact, Identity, and Injection
// ============================================================================

// TestArtifactFileOutput tests that --artifact-file produces correct file
func TestArtifactFileOutput(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
  log.level:
    type: enum
    values: [debug, info, warn, error]
    required: false
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	artifactPath := filepath.Join(tmpDir, "artifact.json")

	cmd := exec.Command(binPath, "run", "--artifact-file", artifactPath, "true")
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"LOG_LEVEL=info",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Check artifact file was created
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("Failed to read artifact file: %v", err)
	}

	// Verify it contains expected fields
	if !bytes.Contains(content, []byte(`"configVersion"`)) {
		t.Errorf("Artifact missing configVersion field")
	}
	if !bytes.Contains(content, []byte(`"values"`)) {
		t.Errorf("Artifact missing values field")
	}
	if !bytes.Contains(content, []byte(`"db.url"`)) {
		t.Errorf("Artifact missing db.url value")
	}
	if !bytes.Contains(content, []byte(`sha256:`)) {
		t.Errorf("Artifact configVersion should have sha256: prefix, got: %s", string(content))
	}
}

// TestArtifactFileParentDirectoryCreation tests that parent directories are created
func TestArtifactFileParentDirectoryCreation(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	// Use a nested path that doesn't exist
	artifactPath := filepath.Join(tmpDir, "nested", "dir", "artifact.json")

	cmd := exec.Command(binPath, "run", "--artifact-file", artifactPath, "true")
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Check artifact file was created in nested directory
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Errorf("Artifact file not created at nested path")
	}
}

// TestInjectEnvMakesArtifactAvailable tests that --inject-env makes artifact available to child
func TestInjectEnvMakesArtifactAvailable(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	outputFile := filepath.Join(tmpDir, "config_output.txt")

	// Use sh -c to echo the injected env var to a file
	cmd := exec.Command(binPath, "run", "--inject-env", "ADMIT_CONFIG", "sh", "-c", "echo $ADMIT_CONFIG > "+outputFile)
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read the output file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Verify it contains artifact JSON
	if !bytes.Contains(content, []byte(`"configVersion"`)) {
		t.Errorf("Injected env var missing configVersion")
	}
	if !bytes.Contains(content, []byte(`"values"`)) {
		t.Errorf("Injected env var missing values")
	}
}

// TestInjectEnvPreservesOriginalEnvVars tests that original env vars are preserved
func TestInjectEnvPreservesOriginalEnvVars(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	outputFile := filepath.Join(tmpDir, "env_output.txt")

	// Echo both the injected var and an original var
	cmd := exec.Command(binPath, "run", "--inject-env", "ADMIT_CONFIG", "sh", "-c", "echo $MY_ORIGINAL_VAR > "+outputFile)
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"MY_ORIGINAL_VAR=preserved_value",
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read the output file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	got := string(bytes.TrimSpace(content))
	if got != "preserved_value" {
		t.Errorf("Original env var not preserved. Expected 'preserved_value', got %q", got)
	}
}

// TestIdentityOutput tests that --identity outputs correct identity JSON
func TestIdentityOutput(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command(binPath, "run", "--identity", "true")
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"PATH=" + os.Getenv("PATH"),
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := stdout.String()

	// Verify identity JSON structure
	if !bytes.Contains([]byte(output), []byte(`"codeHash"`)) {
		t.Errorf("Identity missing codeHash field")
	}
	if !bytes.Contains([]byte(output), []byte(`"configHash"`)) {
		t.Errorf("Identity missing configHash field")
	}
	if !bytes.Contains([]byte(output), []byte(`"executionId"`)) {
		t.Errorf("Identity missing executionId field")
	}
	if !bytes.Contains([]byte(output), []byte(`sha256:`)) {
		t.Errorf("Identity hashes should have sha256: prefix, got: %s", output)
	}
}

// TestIdentityShortOutput tests that --identity-short outputs only executionId
func TestIdentityShortOutput(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  db.url:
    type: string
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command(binPath, "run", "--identity-short", "true")
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"DB_URL=postgres://localhost/test",
		"PATH=" + os.Getenv("PATH"),
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := string(bytes.TrimSpace(stdout.Bytes()))

	// Should be in format sha256:...:sha256:...
	if !bytes.Contains([]byte(output), []byte("sha256:")) {
		t.Errorf("Identity short output should contain sha256: prefix")
	}
	// Should contain the colon separator between hashes
	if bytes.Count([]byte(output), []byte(":")) < 2 {
		t.Errorf("Identity short output should have format codeHash:configHash")
	}
	// Should NOT be JSON (no curly braces)
	if bytes.Contains([]byte(output), []byte("{")) {
		t.Errorf("Identity short output should not be JSON")
	}
}

// TestInvalidConfigNoArtifacts tests that invalid config produces no artifacts even with flags
func TestInvalidConfigNoArtifacts(t *testing.T) {
	binPath := buildAdmitBinary(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	schemaContent := `config:
  payments.mode:
    type: enum
    values: [test, live]
    required: true
`
	tmpDir := createTestSchema(t, schemaContent)
	defer os.RemoveAll(tmpDir)

	artifactPath := filepath.Join(tmpDir, "artifact.json")
	identityPath := filepath.Join(tmpDir, "identity.json")
	injectPath := filepath.Join(tmpDir, "inject.json")

	cmd := exec.Command(binPath, "run",
		"--artifact-file", artifactPath,
		"--identity-file", identityPath,
		"--inject-file", injectPath,
		"true")
	cmd.Dir = tmpDir
	cmd.Env = []string{
		"PAYMENTS_MODE=invalid_value", // Invalid enum value
		"PATH=" + os.Getenv("PATH"),
	}

	err := cmd.Run()

	// Should exit with error
	if err == nil {
		t.Errorf("Expected error for invalid config")
	}

	// No artifact files should be created
	if _, err := os.Stat(artifactPath); err == nil {
		t.Errorf("Artifact file should not be created for invalid config")
	}
	if _, err := os.Stat(identityPath); err == nil {
		t.Errorf("Identity file should not be created for invalid config")
	}
	if _, err := os.Stat(injectPath); err == nil {
		t.Errorf("Inject file should not be created for invalid config")
	}
}
