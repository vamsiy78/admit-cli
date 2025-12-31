package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"admit/internal/artifact"
	"admit/internal/baseline"
	"admit/internal/cli"
	"admit/internal/contract"
	"admit/internal/drift"
	"admit/internal/execid"
	"admit/internal/identity"
	"admit/internal/injector"
	"admit/internal/invariant"
	"admit/internal/launcher"
	"admit/internal/resolver"
	"admit/internal/schema"
	"admit/internal/snapshot"
	"admit/internal/validator"
)

func main() {
	exitCode := run(os.Args[1:], os.Environ(), ".")
	os.Exit(exitCode)
}

// run orchestrates the full execution flow.
// It returns an exit code (0 for success, non-zero for failure).
// This function is separated from main() to enable testing.
func run(args []string, environ []string, defaultSchemaDir string) int {
	// Parse CLI arguments
	cmd, err := cli.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	// Handle v5 subcommands that don't need schema validation
	if cmd.Subcommand == cli.SubcommandSnapshots {
		return runSnapshots(cmd, environ)
	}

	if cmd.Subcommand == cli.SubcommandReplay {
		return runReplay(cmd, environ)
	}

	// Handle v6 baseline subcommand
	if cmd.Subcommand == cli.SubcommandBaseline {
		return runBaseline(cmd, environ)
	}

	// Resolve schema path
	schemaPath := resolveSchemaPath(cmd.SchemaPath, environ, defaultSchemaDir)

	// Load schema
	s, err := schema.LoadSchemaFromPath(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "schema file not found: %s\n", schemaPath)
			return 3
		}
		fmt.Fprintf(os.Stderr, "failed to parse schema: %v\n", err)
		return 3
	}

	// Resolve config from environment
	resolved := resolver.Resolve(s, environ)

	// Validate config
	result := validator.Validate(s, resolved)

	// Check CI mode
	ciMode := cmd.CIMode || getEnvBool(environ, "ADMIT_CI") || getEnvBool(environ, "CI")

	// If invalid: print errors to stderr, exit non-zero
	// No artifacts are produced when validation fails
	if !result.Valid {
		if ciMode {
			for _, verr := range result.Errors {
				fmt.Fprintln(os.Stderr, formatCIAnnotation(verr))
			}
			fmt.Fprintf(os.Stderr, "\n❌ Validation failed: %d error(s)\n", len(result.Errors))
		} else {
			for _, verr := range result.Errors {
				fmt.Fprintln(os.Stderr, validator.FormatError(verr))
			}
		}
		return 1
	}

	// Evaluate invariants (v2 feature)
	// Skip if no invariants defined (backward compatibility)
	var invResults []invariant.InvariantResult
	if len(s.Invariants) > 0 {
		// Build config values map from resolved values
		configValues := make(map[string]string)
		for _, rv := range resolved {
			if rv.Present {
				configValues[rv.Key] = rv.Value
			}
		}

		// Build evaluation context
		evalCtx := invariant.EvalContext{
			ConfigValues: configValues,
			ExecutionEnv: getAdmitEnv(environ),
		}

		// Evaluate all invariants
		invResults = invariant.EvaluateAll(s.Invariants, evalCtx)

		// Handle --invariants-json flag
		if cmd.InvariantsJSON {
			jsonOutput, err := invariant.FormatJSON(invResults)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot format invariant results: %v\n", err)
				return 1
			}
			fmt.Println(jsonOutput)
		}

		// Check for violations
		if invariant.HasViolations(invResults) {
			// Report all violations to stderr (unless JSON mode already printed)
			if !cmd.InvariantsJSON {
				if ciMode {
					for _, inv := range invResults {
						if !inv.Passed {
							fmt.Fprintf(os.Stderr, "::error file=admit.yaml::INVARIANT VIOLATION: '%s' - %s\n", inv.Name, inv.Message)
						}
					}
					violations := invariant.GetViolations(invResults)
					fmt.Fprintf(os.Stderr, "\n❌ Invariant check failed: %d violation(s)\n", len(violations))
				} else {
					fmt.Fprint(os.Stderr, invariant.FormatViolations(invResults))
				}
			}
			// Exit with code 2 for invariant violations
			return 2
		}
	}

	// Evaluate environment contracts (v7 feature)
	// Skip if no environment specified (--env flag or ADMIT_ENV)
	// Skip if no environments section in schema
	envName := resolveEnvironment(cmd.Env, environ)
	if envName != "" && len(s.Environments) > 0 {
		// Check if the specified environment exists
		envContract, exists := s.Environments[envName]
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: unknown environment '%s'\n", envName)
			return 1
		}

		// Build config values map from resolved values
		configValues := make(map[string]string)
		for _, rv := range resolved {
			if rv.Present {
				configValues[rv.Key] = rv.Value
			}
		}

		// Evaluate contract
		contractResult := contract.Evaluate(envContract, configValues)

		// Handle --contract-json flag
		if cmd.ContractJSON {
			jsonOutput, err := contract.FormatJSON(contractResult)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot format contract results: %v\n", err)
				return 1
			}
			fmt.Println(jsonOutput)
		}

		// Check for violations
		if !contractResult.Passed {
			// Report all violations to stderr (unless JSON mode already printed)
			if !cmd.ContractJSON {
				if ciMode {
					fmt.Fprint(os.Stderr, contract.FormatCI(contractResult))
				} else {
					fmt.Fprint(os.Stderr, contract.FormatCLI(contractResult))
				}
			}
			// Exit with code 5 for contract violations - do NOT execute command
			return 5
		}
	}

	// Handle check subcommand - validation only, no execution
	if cmd.Subcommand == cli.SubcommandCheck {
		// Compute execution ID for check mode (uses placeholder command hash)
		schemaKeys := getSchemaKeys(s)
		art := artifact.GenerateArtifact(resolved)
		execID := execid.ComputeExecutionID(art.ConfigVersion, "", []string{}, environ, schemaKeys)

		// Handle execution ID flags in check mode
		if cmd.ExecutionID {
			fmt.Println(execID.Short())
		}

		if cmd.JSONOutput {
			fmt.Println(formatCheckJSONWithExecID(true, result.Errors, invResults, schemaPath, execID.Short()))
		} else if !cmd.ExecutionID {
			fmt.Println("✓ Config valid")
		}
		return 0
	}

	// Handle dry-run mode - validation only, no execution
	if cmd.DryRun {
		// Compute execution ID for dry-run mode
		schemaKeys := getSchemaKeys(s)
		art := artifact.GenerateArtifact(resolved)
		execID := execid.ComputeExecutionID(art.ConfigVersion, cmd.Target, cmd.Args, environ, schemaKeys)

		// Handle execution ID flags in dry-run mode
		if cmd.ExecutionID {
			fmt.Println(execID.Short())
		}

		if cmd.JSONOutput {
			fmt.Println(formatDryRunJSONWithExecID(true, cmd.Target, cmd.Args, schemaPath, execID.Short()))
		} else if !cmd.ExecutionID {
			fmt.Printf("Config valid, would execute: %s %s\n", cmd.Target, strings.Join(cmd.Args, " "))
		}
		return 0
	}

	// Generate config artifact (always generated after successful validation)
	art := artifact.GenerateArtifact(resolved)

	// Handle artifact output flags
	if cmd.ArtifactFile != "" {
		if err := art.WriteToFile(cmd.ArtifactFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write artifact: %s: %v\n", cmd.ArtifactFile, err)
			return 1
		}
	}

	if cmd.ArtifactStdout {
		jsonBytes, err := art.ToJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot serialize artifact: %v\n", err)
			return 1
		}
		fmt.Println(string(jsonBytes))
	}

	if cmd.ArtifactLog {
		fmt.Fprintf(os.Stderr, "configVersion: %s\n", art.ConfigVersion)
	}

	// Handle identity flags
	if cmd.Identity || cmd.IdentityFile != "" || cmd.IdentityShort {
		id, err := identity.ComputeIdentity(cmd, art)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot compute identity: %v\n", err)
			return 1
		}

		if cmd.IdentityFile != "" {
			if err := id.WriteToFile(cmd.IdentityFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot write identity: %s: %v\n", cmd.IdentityFile, err)
				return 1
			}
		}

		if cmd.IdentityShort {
			fmt.Println(id.Short())
		} else if cmd.Identity {
			jsonBytes, err := id.ToJSON()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot serialize identity: %v\n", err)
				return 1
			}
			fmt.Println(string(jsonBytes))
		}
	}

	// Handle v4 execution identity flags
	if cmd.ExecutionID || cmd.ExecutionIDJSON || cmd.ExecutionIDFile != "" || cmd.ExecutionIDEnv != "" {
		// Get schema keys for environment hash filtering
		schemaKeys := getSchemaKeys(s)

		// Compute v4 execution identity
		execID := execid.ComputeExecutionID(art.ConfigVersion, cmd.Target, cmd.Args, environ, schemaKeys)

		if cmd.ExecutionIDFile != "" {
			if err := execID.WriteToFile(cmd.ExecutionIDFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot write execution identity: %s: %v\n", cmd.ExecutionIDFile, err)
				return 1
			}
		}

		if cmd.ExecutionIDJSON {
			jsonBytes, err := execID.ToJSON()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot serialize execution identity: %v\n", err)
				return 1
			}
			fmt.Println(string(jsonBytes))
		} else if cmd.ExecutionID {
			fmt.Println(execID.Short())
		}

		// Inject execution ID into environment if requested
		if cmd.ExecutionIDEnv != "" {
			environ = append(environ, cmd.ExecutionIDEnv+"="+execID.Short())
		}
	}

	// Handle injection flags
	if cmd.InjectFile != "" {
		if err := injector.InjectFile(art, cmd.InjectFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write config: %s: %v\n", cmd.InjectFile, err)
			return 1
		}
	}

	if cmd.InjectEnv != "" {
		var err error
		environ, err = injector.InjectEnv(art, environ, cmd.InjectEnv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot inject config to env: %v\n", err)
			return 1
		}
	}

	// Handle v5 snapshot storage
	if cmd.Snapshot {
		schemaKeys := getSchemaKeys(s)
		execID := execid.ComputeExecutionID(art.ConfigVersion, cmd.Target, cmd.Args, environ, schemaKeys)

		// Build environment map from schema-referenced vars
		envMap := make(map[string]string)
		for _, key := range schemaKeys {
			envVar := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			for _, env := range environ {
				if strings.HasPrefix(env, envVar+"=") {
					envMap[envVar] = strings.TrimPrefix(env, envVar+"=")
					break
				}
			}
		}

		snap := snapshot.ExecutionSnapshot{
			ExecutionID:   execID.ExecutionID,
			ConfigVersion: art.ConfigVersion,
			Command:       cmd.Target,
			Args:          cmd.Args,
			Environment:   envMap,
			SchemaPath:    schemaPath,
			Timestamp:     time.Now().UTC(),
		}

		store := snapshot.NewStore(snapshot.ResolveDir(environ))
		if _, err := store.Save(snap); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot save snapshot: %v\n", err)
			return 1
		}
	}

	// Handle v6 baseline storage
	if cmd.Baseline != "" {
		schemaKeys := getSchemaKeys(s)
		execID := execid.ComputeExecutionID(art.ConfigVersion, cmd.Target, cmd.Args, environ, schemaKeys)

		// Build command string
		cmdStr := cmd.Target
		if len(cmd.Args) > 0 {
			cmdStr = cmd.Target + " " + strings.Join(cmd.Args, " ")
		}

		b := baseline.Baseline{
			Name:         cmd.Baseline,
			ExecutionID:  execID.ExecutionID,
			ConfigHash:   art.ConfigVersion,
			ConfigValues: art.Values,
			Command:      cmdStr,
			Timestamp:    time.Now().UTC(),
		}

		store := baseline.NewStore(baseline.ResolveDir(environ))
		if err := store.Save(b); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot save baseline: %v\n", err)
			return 1
		}
	}

	// Handle v6 drift detection
	if cmd.DetectDrift != "" {
		store := baseline.NewStore(baseline.ResolveDir(environ))
		b, err := store.Load(cmd.DetectDrift)
		if err == nil {
			// Baseline exists, perform drift detection
			report := drift.Detect(b, art.Values, art.ConfigVersion)

			if report.HasDrift {
				// Output drift report (warnings only, never blocks)
				if cmd.DriftJSON {
					jsonOutput, err := drift.FormatJSON(report)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: cannot format drift report: %v\n", err)
					} else {
						fmt.Fprintln(os.Stderr, jsonOutput)
					}
				} else if ciMode {
					fmt.Fprint(os.Stderr, drift.FormatCI(report))
				} else {
					fmt.Fprint(os.Stderr, drift.FormatCLI(report))
				}
			}
		}
		// If baseline not found, silently continue (no error)
	}

	// If valid: exec target command (silent success)
	// Note: Exec replaces the process, so this only returns on error
	err = launcher.Exec(cmd, environ)
	if err != nil {
		if launcher.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "Error: command not found: %s\n", cmd.Target)
			return 127
		}
		if launcher.IsPermissionDenied(err) {
			fmt.Fprintf(os.Stderr, "Error: permission denied: %s\n", cmd.Target)
			return 126
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// This line should never be reached if Exec succeeds
	return 0
}

// resolveSchemaPath determines the schema path from flag, env var, or default
func resolveSchemaPath(flagValue string, environ []string, defaultDir string) string {
	// Flag takes precedence
	if flagValue != "" {
		if filepath.IsAbs(flagValue) {
			return flagValue
		}
		return filepath.Join(defaultDir, flagValue)
	}

	// Check ADMIT_SCHEMA env var
	for _, env := range environ {
		if strings.HasPrefix(env, "ADMIT_SCHEMA=") {
			path := strings.TrimPrefix(env, "ADMIT_SCHEMA=")
			if filepath.IsAbs(path) {
				return path
			}
			return filepath.Join(defaultDir, path)
		}
	}

	// Default to admit.yaml in default directory
	return filepath.Join(defaultDir, "admit.yaml")
}

// getAdmitEnv extracts the ADMIT_ENV value from the environment slice
func getAdmitEnv(environ []string) string {
	for _, env := range environ {
		if strings.HasPrefix(env, "ADMIT_ENV=") {
			return strings.TrimPrefix(env, "ADMIT_ENV=")
		}
	}
	return ""
}

// resolveEnvironment determines the environment name from --env flag or ADMIT_ENV
// Flag takes precedence over environment variable
func resolveEnvironment(flagValue string, environ []string) string {
	// Flag takes precedence
	if flagValue != "" {
		return flagValue
	}
	// Fall back to ADMIT_ENV environment variable
	return getAdmitEnv(environ)
}

// getEnvBool checks if an environment variable is set to a truthy value
func getEnvBool(environ []string, name string) bool {
	prefix := name + "="
	for _, env := range environ {
		if strings.HasPrefix(env, prefix) {
			val := strings.ToLower(strings.TrimPrefix(env, prefix))
			return val == "true" || val == "1" || val == "yes"
		}
	}
	return false
}

// formatCIAnnotation formats a validation error as GitHub Actions annotation
func formatCIAnnotation(err validator.ValidationError) string {
	return fmt.Sprintf("::error file=admit.yaml::%s", validator.FormatError(err))
}

// formatCheckJSON formats check results as JSON
func formatCheckJSON(valid bool, valErrors []validator.ValidationError, invResults []invariant.InvariantResult, schemaPath string) string {
	return formatCheckJSONWithExecID(valid, valErrors, invResults, schemaPath, "")
}

// formatCheckJSONWithExecID formats check results as JSON with optional execution ID
func formatCheckJSONWithExecID(valid bool, valErrors []validator.ValidationError, invResults []invariant.InvariantResult, schemaPath string, executionID string) string {
	// Simple JSON formatting without external dependencies
	var sb strings.Builder
	sb.WriteString("{")
	sb.WriteString(fmt.Sprintf(`"valid":%t,`, valid))
	sb.WriteString(`"validationErrors":[`)
	for i, err := range valErrors {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"key":"%s","envVar":"%s","message":"%s"}`, err.Key, err.EnvVar, escapeJSON(err.Message)))
	}
	sb.WriteString("],")
	sb.WriteString(`"invariantResults":[`)
	for i, inv := range invResults {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"name":"%s","rule":"%s","passed":%t}`, inv.Name, escapeJSON(inv.Rule), inv.Passed))
	}
	sb.WriteString("],")
	sb.WriteString(fmt.Sprintf(`"schemaPath":"%s"`, escapeJSON(schemaPath)))
	if executionID != "" {
		sb.WriteString(fmt.Sprintf(`,"executionId":"%s"`, escapeJSON(executionID)))
	}
	sb.WriteString("}")
	return sb.String()
}

// formatDryRunJSON formats dry-run results as JSON
func formatDryRunJSON(valid bool, command string, args []string, schemaPath string) string {
	return formatDryRunJSONWithExecID(valid, command, args, schemaPath, "")
}

// formatDryRunJSONWithExecID formats dry-run results as JSON with optional execution ID
func formatDryRunJSONWithExecID(valid bool, command string, args []string, schemaPath string, executionID string) string {
	var sb strings.Builder
	sb.WriteString("{")
	sb.WriteString(fmt.Sprintf(`"valid":%t,`, valid))
	sb.WriteString(fmt.Sprintf(`"command":"%s",`, escapeJSON(command)))
	sb.WriteString(`"args":[`)
	for i, arg := range args {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"%s"`, escapeJSON(arg)))
	}
	sb.WriteString("],")
	sb.WriteString(fmt.Sprintf(`"schemaPath":"%s"`, escapeJSON(schemaPath)))
	if executionID != "" {
		sb.WriteString(fmt.Sprintf(`,"executionId":"%s"`, escapeJSON(executionID)))
	}
	sb.WriteString("}")
	return sb.String()
}

// escapeJSON escapes special characters for JSON strings
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// getSchemaKeys extracts config key paths from the schema
func getSchemaKeys(s schema.Schema) []string {
	keys := make([]string, 0, len(s.Config))
	for key := range s.Config {
		keys = append(keys, key)
	}
	return keys
}

// runSnapshots handles the snapshots subcommand.
func runSnapshots(cmd cli.Command, environ []string) int {
	store := snapshot.NewStore(snapshot.ResolveDir(environ))

	// Handle --delete flag
	if cmd.DeleteID != "" {
		if err := store.Delete(cmd.DeleteID); err != nil {
			if err == snapshot.ErrSnapshotNotFound {
				fmt.Fprintf(os.Stderr, "Error: snapshot not found: %s\n", cmd.DeleteID)
				return 4
			}
			fmt.Fprintf(os.Stderr, "Error: cannot delete snapshot: %v\n", err)
			return 1
		}
		fmt.Printf("Deleted snapshot: %s\n", cmd.DeleteID)
		return 0
	}

	// Handle --prune flag
	if cmd.PruneDays > 0 {
		duration := time.Duration(cmd.PruneDays) * 24 * time.Hour
		deleted, err := store.Prune(duration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot prune snapshots: %v\n", err)
			return 1
		}
		fmt.Printf("Pruned %d snapshot(s) older than %d days\n", deleted, cmd.PruneDays)
		return 0
	}

	// List snapshots
	summaries, err := store.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot list snapshots: %v\n", err)
		return 1
	}

	if len(summaries) == 0 {
		if cmd.JSONOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No snapshots found")
		}
		return 0
	}

	if cmd.JSONOutput {
		data, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot serialize snapshots: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	} else {
		for _, s := range summaries {
			fmt.Printf("%s  %s  %s\n", s.ExecutionID, s.Command, s.Timestamp.Format(time.RFC3339))
		}
	}

	return 0
}

// runReplay handles the replay subcommand.
func runReplay(cmd cli.Command, environ []string) int {
	store := snapshot.NewStore(snapshot.ResolveDir(environ))

	// Load snapshot
	snap, err := store.Load(cmd.ReplayID)
	if err != nil {
		if err == snapshot.ErrSnapshotNotFound {
			fmt.Fprintf(os.Stderr, "Error: snapshot not found: %s\n", cmd.ReplayID)
			return 4
		}
		fmt.Fprintf(os.Stderr, "Error: cannot load snapshot: %v\n", err)
		return 1
	}

	// Get schema keys for verification (from snapshot environment keys)
	var schemaKeys []string
	for k := range snap.Environment {
		// Convert env var back to schema key (e.g., DB_URL -> db.url)
		key := strings.ToLower(strings.ReplaceAll(k, "_", "."))
		schemaKeys = append(schemaKeys, key)
	}

	// Verify snapshot integrity
	verifyResult := snapshot.Verify(snap, schemaKeys)
	if verifyResult.IDMismatch {
		fmt.Fprintln(os.Stderr, "Warning: snapshot may be corrupted (execution ID mismatch)")
	}
	if verifyResult.SchemaChanged {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", verifyResult.SchemaMessage)
	}

	// Handle --json flag
	if cmd.JSONOutput {
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot serialize snapshot: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}

	// Handle --dry-run flag
	if cmd.DryRun {
		fmt.Printf("Would execute: %s %s\n", snap.Command, strings.Join(snap.Args, " "))
		fmt.Println("With environment:")
		for k, v := range snap.Environment {
			fmt.Printf("  %s=%s\n", k, v)
		}
		return 0
	}

	// Restore environment from snapshot
	for k, v := range snap.Environment {
		environ = append(environ, k+"="+v)
	}

	// Execute the command
	replayCmd := cli.Command{
		Target: snap.Command,
		Args:   snap.Args,
	}

	err = launcher.Exec(replayCmd, environ)
	if err != nil {
		if launcher.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "Error: command not found: %s\n", snap.Command)
			return 127
		}
		if launcher.IsPermissionDenied(err) {
			fmt.Fprintf(os.Stderr, "Error: permission denied: %s\n", snap.Command)
			return 126
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}


// runBaseline handles the baseline subcommand.
func runBaseline(cmd cli.Command, environ []string) int {
	store := baseline.NewStore(baseline.ResolveDir(environ))

	switch cmd.BaselineAction {
	case "list":
		summaries, err := store.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot list baselines: %v\n", err)
			return 1
		}

		if len(summaries) == 0 {
			if cmd.JSONOutput {
				fmt.Println("[]")
			} else {
				fmt.Println("No baselines found")
			}
			return 0
		}

		if cmd.JSONOutput {
			data, err := json.MarshalIndent(summaries, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot serialize baselines: %v\n", err)
				return 1
			}
			fmt.Println(string(data))
		} else {
			for _, b := range summaries {
				fmt.Printf("%s  %s  %s  %s\n", b.Name, b.ConfigHash[:20]+"...", b.Command, b.Timestamp.Format(time.RFC3339))
			}
		}
		return 0

	case "show":
		b, err := store.Load(cmd.BaselineName)
		if err != nil {
			if err == baseline.ErrBaselineNotFound {
				fmt.Fprintf(os.Stderr, "Error: baseline not found: %s\n", cmd.BaselineName)
				return 4
			}
			fmt.Fprintf(os.Stderr, "Error: cannot load baseline: %v\n", err)
			return 1
		}

		if cmd.JSONOutput {
			data, err := json.MarshalIndent(b, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot serialize baseline: %v\n", err)
				return 1
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("Name:        %s\n", b.Name)
			fmt.Printf("ConfigHash:  %s\n", b.ConfigHash)
			fmt.Printf("ExecutionID: %s\n", b.ExecutionID)
			fmt.Printf("Command:     %s\n", b.Command)
			fmt.Printf("Timestamp:   %s\n", b.Timestamp.Format(time.RFC3339))
			fmt.Println("Config Values:")
			for k, v := range b.ConfigValues {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
		return 0

	case "delete":
		if err := store.Delete(cmd.BaselineName); err != nil {
			if err == baseline.ErrBaselineNotFound {
				fmt.Fprintf(os.Stderr, "Error: baseline not found: %s\n", cmd.BaselineName)
				return 4
			}
			fmt.Fprintf(os.Stderr, "Error: cannot delete baseline: %v\n", err)
			return 1
		}
		fmt.Printf("Deleted baseline: %s\n", cmd.BaselineName)
		return 0
	}

	return 1
}
