package main

import (
	"fmt"
	"os"
	"strings"

	"admit/internal/artifact"
	"admit/internal/cli"
	"admit/internal/identity"
	"admit/internal/injector"
	"admit/internal/invariant"
	"admit/internal/launcher"
	"admit/internal/resolver"
	"admit/internal/schema"
	"admit/internal/validator"
)

func main() {
	exitCode := run(os.Args[1:], os.Environ(), ".")
	os.Exit(exitCode)
}

// run orchestrates the full execution flow.
// It returns an exit code (0 for success, non-zero for failure).
// This function is separated from main() to enable testing.
func run(args []string, environ []string, schemaDir string) int {
	// Parse CLI arguments
	cmd, err := cli.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	// Load schema from current directory
	s, err := schema.LoadSchema(schemaDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}

	// Resolve config from environment
	resolved := resolver.Resolve(s, environ)

	// Validate config
	result := validator.Validate(s, resolved)

	// If invalid: print errors to stderr, exit non-zero
	// No artifacts are produced when validation fails
	if !result.Valid {
		for _, verr := range result.Errors {
			fmt.Fprintln(os.Stderr, validator.FormatError(verr))
		}
		return 1
	}

	// Evaluate invariants (v2 feature)
	// Skip if no invariants defined (backward compatibility)
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
		invResults := invariant.EvaluateAll(s.Invariants, evalCtx)

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
				fmt.Fprint(os.Stderr, invariant.FormatViolations(invResults))
			}
			// Exit with code 2 for invariant violations
			return 2
		}
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


// getAdmitEnv extracts the ADMIT_ENV value from the environment slice
func getAdmitEnv(environ []string) string {
	for _, env := range environ {
		if strings.HasPrefix(env, "ADMIT_ENV=") {
			return strings.TrimPrefix(env, "ADMIT_ENV=")
		}
	}
	return ""
}
