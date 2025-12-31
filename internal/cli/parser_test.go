package cli

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestParseArgs_ValidCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantTarget string
		wantArgs   []string
	}{
		{
			name:       "simple command",
			args:       []string{"run", "echo"},
			wantTarget: "echo",
			wantArgs:   nil,
		},
		{
			name:       "command with one arg",
			args:       []string{"run", "echo", "hello"},
			wantTarget: "echo",
			wantArgs:   []string{"hello"},
		},
		{
			name:       "command with multiple args",
			args:       []string{"run", "node", "index.js", "--port", "3000"},
			wantTarget: "node",
			wantArgs:   []string{"index.js", "--port", "3000"},
		},
		{
			name:       "npm start",
			args:       []string{"run", "npm", "start"},
			wantTarget: "npm",
			wantArgs:   []string{"start"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("Args length = %d, want %d", len(cmd.Args), len(tt.wantArgs))
			}
			for i := range cmd.Args {
				if cmd.Args[i] != tt.wantArgs[i] {
					t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseArgs_Errors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "empty args",
			args:    []string{},
			wantErr: ErrNoRunSubcommand,
		},
		{
			name:    "missing run subcommand",
			args:    []string{"exec", "echo"},
			wantErr: ErrNoRunSubcommand,
		},
		{
			name:    "run without command",
			args:    []string{"run"},
			wantErr: ErrNoCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseArgs(tt.args)
			if err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Feature: admit-cli, Property 1: Argument Parsing Preservation
// Validates: Requirements 1.2
// For any list of command-line arguments after "run", parsing SHALL preserve
// all arguments in order and pass them to the target command.
func TestParseArgs_ArgumentPreservation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: target command is preserved
	properties.Property("target command is preserved", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true // Skip empty targets
			}
			args := []string{"run", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: all arguments are preserved in order
	properties.Property("all arguments preserved in order", prop.ForAll(
		func(target string, cmdArgs []string) bool {
			if target == "" {
				return true // Skip empty targets
			}
			// Build input args: ["run", target, arg1, arg2, ...]
			args := append([]string{"run", target}, cmdArgs...)
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			// Check target is preserved
			if cmd.Target != target {
				return false
			}
			// Check all args are preserved in order
			if len(cmd.Args) != len(cmdArgs) {
				return false
			}
			for i, arg := range cmdArgs {
				if cmd.Args[i] != arg {
					return false
				}
			}
			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: argument count is preserved
	properties.Property("argument count is preserved", prop.ForAll(
		func(target string, numArgs int) bool {
			if target == "" || numArgs < 0 {
				return true
			}
			// Cap numArgs to reasonable size
			if numArgs > 20 {
				numArgs = 20
			}
			// Generate args
			cmdArgs := make([]string, numArgs)
			for i := 0; i < numArgs; i++ {
				cmdArgs[i] = "arg"
			}
			args := append([]string{"run", target}, cmdArgs...)
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return len(cmd.Args) == numArgs
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}


// TestParseArgs_NewFlags tests the new v1 flags
func TestParseArgs_NewFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantTarget     string
		wantArgs       []string
		wantArtFile    string
		wantArtStdout  bool
		wantArtLog     bool
		wantInjectFile string
		wantInjectEnv  string
		wantIdentity   bool
		wantIdFile     string
		wantIdShort    bool
	}{
		{
			name:        "artifact-file flag",
			args:        []string{"run", "--artifact-file", "/tmp/artifact.json", "echo", "hello"},
			wantTarget:  "echo",
			wantArgs:    []string{"hello"},
			wantArtFile: "/tmp/artifact.json",
		},
		{
			name:          "artifact-stdout flag",
			args:          []string{"run", "--artifact-stdout", "echo"},
			wantTarget:    "echo",
			wantArtStdout: true,
		},
		{
			name:       "artifact-log flag",
			args:       []string{"run", "--artifact-log", "echo"},
			wantTarget: "echo",
			wantArtLog: true,
		},
		{
			name:           "inject-file flag",
			args:           []string{"run", "--inject-file", "/tmp/config.json", "node", "app.js"},
			wantTarget:     "node",
			wantArgs:       []string{"app.js"},
			wantInjectFile: "/tmp/config.json",
		},
		{
			name:          "inject-env flag",
			args:          []string{"run", "--inject-env", "ADMIT_CONFIG", "python", "main.py"},
			wantTarget:    "python",
			wantArgs:      []string{"main.py"},
			wantInjectEnv: "ADMIT_CONFIG",
		},
		{
			name:         "identity flag",
			args:         []string{"run", "--identity", "echo"},
			wantTarget:   "echo",
			wantIdentity: true,
		},
		{
			name:       "identity-file flag",
			args:       []string{"run", "--identity-file", "/tmp/identity.json", "echo"},
			wantTarget: "echo",
			wantIdFile: "/tmp/identity.json",
		},
		{
			name:        "identity-short flag",
			args:        []string{"run", "--identity-short", "echo"},
			wantTarget:  "echo",
			wantIdShort: true,
		},
		{
			name:           "multiple flags",
			args:           []string{"run", "--artifact-file", "/tmp/art.json", "--inject-env", "CONFIG", "--identity", "node", "server.js"},
			wantTarget:     "node",
			wantArgs:       []string{"server.js"},
			wantArtFile:    "/tmp/art.json",
			wantInjectEnv:  "CONFIG",
			wantIdentity:   true,
		},
		{
			name:          "all flags",
			args:          []string{"run", "--artifact-file", "a.json", "--artifact-stdout", "--artifact-log", "--inject-file", "i.json", "--inject-env", "VAR", "--identity", "--identity-file", "id.json", "--identity-short", "cmd"},
			wantTarget:    "cmd",
			wantArtFile:   "a.json",
			wantArtStdout: true,
			wantArtLog:    true,
			wantInjectFile: "i.json",
			wantInjectEnv: "VAR",
			wantIdentity:  true,
			wantIdFile:    "id.json",
			wantIdShort:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("Args length = %d, want %d", len(cmd.Args), len(tt.wantArgs))
			}
			for i := range cmd.Args {
				if i < len(tt.wantArgs) && cmd.Args[i] != tt.wantArgs[i] {
					t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], tt.wantArgs[i])
				}
			}
			if cmd.ArtifactFile != tt.wantArtFile {
				t.Errorf("ArtifactFile = %q, want %q", cmd.ArtifactFile, tt.wantArtFile)
			}
			if cmd.ArtifactStdout != tt.wantArtStdout {
				t.Errorf("ArtifactStdout = %v, want %v", cmd.ArtifactStdout, tt.wantArtStdout)
			}
			if cmd.ArtifactLog != tt.wantArtLog {
				t.Errorf("ArtifactLog = %v, want %v", cmd.ArtifactLog, tt.wantArtLog)
			}
			if cmd.InjectFile != tt.wantInjectFile {
				t.Errorf("InjectFile = %q, want %q", cmd.InjectFile, tt.wantInjectFile)
			}
			if cmd.InjectEnv != tt.wantInjectEnv {
				t.Errorf("InjectEnv = %q, want %q", cmd.InjectEnv, tt.wantInjectEnv)
			}
			if cmd.Identity != tt.wantIdentity {
				t.Errorf("Identity = %v, want %v", cmd.Identity, tt.wantIdentity)
			}
			if cmd.IdentityFile != tt.wantIdFile {
				t.Errorf("IdentityFile = %q, want %q", cmd.IdentityFile, tt.wantIdFile)
			}
			if cmd.IdentityShort != tt.wantIdShort {
				t.Errorf("IdentityShort = %v, want %v", cmd.IdentityShort, tt.wantIdShort)
			}
		})
	}
}

func TestParseArgs_FlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "artifact-file without value",
			args:    []string{"run", "--artifact-file"},
			wantErr: ErrMissingFlagValue,
		},
		{
			name:    "inject-file without value",
			args:    []string{"run", "--inject-file"},
			wantErr: ErrMissingFlagValue,
		},
		{
			name:    "inject-env without value",
			args:    []string{"run", "--inject-env"},
			wantErr: ErrMissingFlagValue,
		},
		{
			name:    "identity-file without value",
			args:    []string{"run", "--identity-file"},
			wantErr: ErrMissingFlagValue,
		},
		{
			name:    "only flags no command",
			args:    []string{"run", "--artifact-stdout", "--identity"},
			wantErr: ErrNoCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseArgs(tt.args)
			if err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}


// TestParseArgs_InvariantsJSONFlag tests the --invariants-json flag
func TestParseArgs_InvariantsJSONFlag(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		wantTarget         string
		wantInvariantsJSON bool
	}{
		{
			name:               "invariants-json flag",
			args:               []string{"run", "--invariants-json", "echo"},
			wantTarget:         "echo",
			wantInvariantsJSON: true,
		},
		{
			name:               "invariants-json with other flags",
			args:               []string{"run", "--invariants-json", "--artifact-stdout", "node", "app.js"},
			wantTarget:         "node",
			wantInvariantsJSON: true,
		},
		{
			name:               "no invariants-json flag",
			args:               []string{"run", "echo"},
			wantTarget:         "echo",
			wantInvariantsJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if cmd.InvariantsJSON != tt.wantInvariantsJSON {
				t.Errorf("InvariantsJSON = %v, want %v", cmd.InvariantsJSON, tt.wantInvariantsJSON)
			}
		})
	}
}


// TestParseArgs_CheckSubcommand tests the check subcommand
func TestParseArgs_CheckSubcommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantSubcommand Subcommand
		wantJSON       bool
		wantSchema     string
		wantErr        error
	}{
		{
			name:           "check subcommand",
			args:           []string{"check"},
			wantSubcommand: SubcommandCheck,
		},
		{
			name:           "check with --json",
			args:           []string{"check", "--json"},
			wantSubcommand: SubcommandCheck,
			wantJSON:       true,
		},
		{
			name:           "check with --schema",
			args:           []string{"check", "--schema", "/app/admit.yaml"},
			wantSubcommand: SubcommandCheck,
			wantSchema:     "/app/admit.yaml",
		},
		{
			name:           "check with --json and --schema",
			args:           []string{"check", "--json", "--schema", "/app/admit.yaml"},
			wantSubcommand: SubcommandCheck,
			wantJSON:       true,
			wantSchema:     "/app/admit.yaml",
		},
		{
			name:           "run subcommand still works",
			args:           []string{"run", "echo"},
			wantSubcommand: SubcommandRun,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Subcommand != tt.wantSubcommand {
				t.Errorf("Subcommand = %q, want %q", cmd.Subcommand, tt.wantSubcommand)
			}
			if cmd.JSONOutput != tt.wantJSON {
				t.Errorf("JSONOutput = %v, want %v", cmd.JSONOutput, tt.wantJSON)
			}
			if cmd.SchemaPath != tt.wantSchema {
				t.Errorf("SchemaPath = %q, want %q", cmd.SchemaPath, tt.wantSchema)
			}
		})
	}
}

// TestParseArgs_V3Flags tests the new v3 flags
func TestParseArgs_V3Flags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantTarget string
		wantSchema string
		wantDryRun bool
		wantCI     bool
		wantJSON   bool
	}{
		{
			name:       "schema flag",
			args:       []string{"run", "--schema", "/app/admit.yaml", "echo"},
			wantTarget: "echo",
			wantSchema: "/app/admit.yaml",
		},
		{
			name:       "dry-run flag",
			args:       []string{"run", "--dry-run", "echo"},
			wantTarget: "echo",
			wantDryRun: true,
		},
		{
			name:       "ci flag",
			args:       []string{"run", "--ci", "echo"},
			wantTarget: "echo",
			wantCI:     true,
		},
		{
			name:       "json flag",
			args:       []string{"run", "--json", "echo"},
			wantTarget: "echo",
			wantJSON:   true,
		},
		{
			name:       "all v3 flags",
			args:       []string{"run", "--schema", "/app/admit.yaml", "--dry-run", "--ci", "--json", "node", "app.js"},
			wantTarget: "node",
			wantSchema: "/app/admit.yaml",
			wantDryRun: true,
			wantCI:     true,
			wantJSON:   true,
		},
		{
			name:       "v3 flags with v1 flags",
			args:       []string{"run", "--schema", "/app/admit.yaml", "--artifact-stdout", "--identity", "echo"},
			wantTarget: "echo",
			wantSchema: "/app/admit.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if cmd.SchemaPath != tt.wantSchema {
				t.Errorf("SchemaPath = %q, want %q", cmd.SchemaPath, tt.wantSchema)
			}
			if cmd.DryRun != tt.wantDryRun {
				t.Errorf("DryRun = %v, want %v", cmd.DryRun, tt.wantDryRun)
			}
			if cmd.CIMode != tt.wantCI {
				t.Errorf("CIMode = %v, want %v", cmd.CIMode, tt.wantCI)
			}
			if cmd.JSONOutput != tt.wantJSON {
				t.Errorf("JSONOutput = %v, want %v", cmd.JSONOutput, tt.wantJSON)
			}
		})
	}
}

// TestParseArgs_V3FlagErrors tests error cases for v3 flags
func TestParseArgs_V3FlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "schema without value",
			args:    []string{"run", "--schema"},
			wantErr: ErrMissingFlagValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseArgs(tt.args)
			if err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}


// Feature: admit-v3-container-ci, Property 7: Backward Compatibility
// Validates: Requirements 7.1, 7.2, 7.3
// For any v2-compatible invocation (no new flags), behavior SHALL be identical to v2.
func TestParseArgs_BackwardCompatibility_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: v2 invocations still work with run subcommand
	properties.Property("v2 invocations parse correctly", prop.ForAll(
		func(target string, args []string) bool {
			if target == "" {
				return true // Skip empty targets
			}
			// Build v2-style args: ["run", target, arg1, arg2, ...]
			inputArgs := append([]string{"run", target}, args...)
			cmd, err := ParseArgs(inputArgs)
			if err != nil {
				return false
			}
			// Verify subcommand is run
			if cmd.Subcommand != SubcommandRun {
				return false
			}
			// Verify target is preserved
			if cmd.Target != target {
				return false
			}
			// Verify args are preserved
			if len(cmd.Args) != len(args) {
				return false
			}
			for i, arg := range args {
				if cmd.Args[i] != arg {
					return false
				}
			}
			// Verify new v3 flags are not set
			if cmd.SchemaPath != "" || cmd.DryRun || cmd.CIMode || cmd.JSONOutput {
				return false
			}
			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: v1 flags still work
	properties.Property("v1 flags still work", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			// Test with v1 flags
			args := []string{"run", "--artifact-stdout", "--identity", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.ArtifactStdout && cmd.Identity
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Property: v2 invariants-json flag still works
	properties.Property("v2 invariants-json flag still works", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			args := []string{"run", "--invariants-json", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.InvariantsJSON
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}


// TestParseArgs_V4Flags tests the new v4 execution identity flags
func TestParseArgs_V4Flags(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantTarget      string
		wantExecID      bool
		wantExecIDJSON  bool
		wantExecIDFile  string
		wantExecIDEnv   string
	}{
		{
			name:       "execution-id flag",
			args:       []string{"run", "--execution-id", "echo"},
			wantTarget: "echo",
			wantExecID: true,
		},
		{
			name:           "execution-id-json flag",
			args:           []string{"run", "--execution-id-json", "echo"},
			wantTarget:     "echo",
			wantExecIDJSON: true,
		},
		{
			name:           "execution-id-file flag",
			args:           []string{"run", "--execution-id-file", "/tmp/execid.json", "echo"},
			wantTarget:     "echo",
			wantExecIDFile: "/tmp/execid.json",
		},
		{
			name:          "execution-id-env flag",
			args:          []string{"run", "--execution-id-env", "EXEC_ID", "echo"},
			wantTarget:    "echo",
			wantExecIDEnv: "EXEC_ID",
		},
		{
			name:           "all v4 flags",
			args:           []string{"run", "--execution-id", "--execution-id-json", "--execution-id-file", "/tmp/id.json", "--execution-id-env", "EXEC_ID", "node", "app.js"},
			wantTarget:     "node",
			wantExecID:     true,
			wantExecIDJSON: true,
			wantExecIDFile: "/tmp/id.json",
			wantExecIDEnv:  "EXEC_ID",
		},
		{
			name:       "v4 flags with v1 identity flags",
			args:       []string{"run", "--execution-id", "--identity", "--identity-short", "echo"},
			wantTarget: "echo",
			wantExecID: true,
		},
		{
			name:       "v4 flags with v3 flags",
			args:       []string{"run", "--execution-id", "--dry-run", "echo"},
			wantTarget: "echo",
			wantExecID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if cmd.ExecutionID != tt.wantExecID {
				t.Errorf("ExecutionID = %v, want %v", cmd.ExecutionID, tt.wantExecID)
			}
			if cmd.ExecutionIDJSON != tt.wantExecIDJSON {
				t.Errorf("ExecutionIDJSON = %v, want %v", cmd.ExecutionIDJSON, tt.wantExecIDJSON)
			}
			if cmd.ExecutionIDFile != tt.wantExecIDFile {
				t.Errorf("ExecutionIDFile = %q, want %q", cmd.ExecutionIDFile, tt.wantExecIDFile)
			}
			if cmd.ExecutionIDEnv != tt.wantExecIDEnv {
				t.Errorf("ExecutionIDEnv = %q, want %q", cmd.ExecutionIDEnv, tt.wantExecIDEnv)
			}
		})
	}
}

// TestParseArgs_V4FlagErrors tests error cases for v4 flags
func TestParseArgs_V4FlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "execution-id-file without value",
			args:    []string{"run", "--execution-id-file"},
			wantErr: ErrMissingFlagValue,
		},
		{
			name:    "execution-id-env without value",
			args:    []string{"run", "--execution-id-env"},
			wantErr: ErrMissingFlagValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseArgs(tt.args)
			if err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Feature: admit-v4-execution-identity, Property 10: Backward Compatibility
// Validates: Requirements 6.1, 6.2, 6.3
// For any v3-compatible invocation (no v4 flags), behavior SHALL be identical to v3.
// The v1 identity flags SHALL continue to function independently of v4 execution-id flags.
func TestParseArgs_V4BackwardCompatibility_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: v3 invocations still work
	properties.Property("v3 invocations parse correctly without v4 flags set", prop.ForAll(
		func(target string, args []string) bool {
			if target == "" {
				return true // Skip empty targets
			}
			// Build v3-style args: ["run", target, arg1, arg2, ...]
			inputArgs := append([]string{"run", target}, args...)
			cmd, err := ParseArgs(inputArgs)
			if err != nil {
				return false
			}
			// Verify subcommand is run
			if cmd.Subcommand != SubcommandRun {
				return false
			}
			// Verify target is preserved
			if cmd.Target != target {
				return false
			}
			// Verify args are preserved
			if len(cmd.Args) != len(args) {
				return false
			}
			for i, arg := range args {
				if cmd.Args[i] != arg {
					return false
				}
			}
			// Verify v4 flags are not set
			if cmd.ExecutionID || cmd.ExecutionIDJSON || cmd.ExecutionIDFile != "" || cmd.ExecutionIDEnv != "" {
				return false
			}
			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: v1 identity flags still work independently
	properties.Property("v1 identity flags work independently of v4 flags", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			// Test with v1 identity flags
			args := []string{"run", "--identity", "--identity-short", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.Identity && cmd.IdentityShort
		},
		gen.Identifier(),
	))

	// Property: v1 and v4 identity flags can be used together
	properties.Property("v1 and v4 identity flags can coexist", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			// Test with both v1 and v4 identity flags
			args := []string{"run", "--identity", "--execution-id", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.Identity && cmd.ExecutionID
		},
		gen.Identifier(),
	))

	// Property: v3 flags still work with v4 flags
	properties.Property("v3 flags work alongside v4 flags", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			args := []string{"run", "--dry-run", "--execution-id", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.DryRun && cmd.ExecutionID
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// TestParseArgs_V5ReplaySubcommand tests the replay subcommand
func TestParseArgs_V5ReplaySubcommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantSubcommand Subcommand
		wantReplayID   string
		wantDryRun     bool
		wantJSON       bool
		wantErr        bool
	}{
		{
			name:           "replay with execution ID",
			args:           []string{"replay", "sha256:abc123"},
			wantSubcommand: SubcommandReplay,
			wantReplayID:   "sha256:abc123",
		},
		{
			name:           "replay with --dry-run",
			args:           []string{"replay", "--dry-run", "sha256:abc123"},
			wantSubcommand: SubcommandReplay,
			wantReplayID:   "sha256:abc123",
			wantDryRun:     true,
		},
		{
			name:           "replay with --json",
			args:           []string{"replay", "--json", "sha256:abc123"},
			wantSubcommand: SubcommandReplay,
			wantReplayID:   "sha256:abc123",
			wantJSON:       true,
		},
		{
			name:           "replay with flags after ID",
			args:           []string{"replay", "sha256:abc123", "--dry-run"},
			wantSubcommand: SubcommandReplay,
			wantReplayID:   "sha256:abc123",
			wantDryRun:     true,
		},
		{
			name:    "replay without execution ID",
			args:    []string{"replay"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Subcommand != tt.wantSubcommand {
				t.Errorf("Subcommand = %q, want %q", cmd.Subcommand, tt.wantSubcommand)
			}
			if cmd.ReplayID != tt.wantReplayID {
				t.Errorf("ReplayID = %q, want %q", cmd.ReplayID, tt.wantReplayID)
			}
			if cmd.DryRun != tt.wantDryRun {
				t.Errorf("DryRun = %v, want %v", cmd.DryRun, tt.wantDryRun)
			}
			if cmd.JSONOutput != tt.wantJSON {
				t.Errorf("JSONOutput = %v, want %v", cmd.JSONOutput, tt.wantJSON)
			}
		})
	}
}

// TestParseArgs_V5SnapshotsSubcommand tests the snapshots subcommand
func TestParseArgs_V5SnapshotsSubcommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantSubcommand Subcommand
		wantJSON       bool
		wantPruneDays  int
		wantDeleteID   string
		wantErr        bool
	}{
		{
			name:           "snapshots list",
			args:           []string{"snapshots"},
			wantSubcommand: SubcommandSnapshots,
		},
		{
			name:           "snapshots with --json",
			args:           []string{"snapshots", "--json"},
			wantSubcommand: SubcommandSnapshots,
			wantJSON:       true,
		},
		{
			name:           "snapshots with --prune",
			args:           []string{"snapshots", "--prune", "30"},
			wantSubcommand: SubcommandSnapshots,
			wantPruneDays:  30,
		},
		{
			name:           "snapshots with --delete",
			args:           []string{"snapshots", "--delete", "sha256:abc123"},
			wantSubcommand: SubcommandSnapshots,
			wantDeleteID:   "sha256:abc123",
		},
		{
			name:    "snapshots --prune without value",
			args:    []string{"snapshots", "--prune"},
			wantErr: true,
		},
		{
			name:    "snapshots --delete without value",
			args:    []string{"snapshots", "--delete"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Subcommand != tt.wantSubcommand {
				t.Errorf("Subcommand = %q, want %q", cmd.Subcommand, tt.wantSubcommand)
			}
			if cmd.JSONOutput != tt.wantJSON {
				t.Errorf("JSONOutput = %v, want %v", cmd.JSONOutput, tt.wantJSON)
			}
			if cmd.PruneDays != tt.wantPruneDays {
				t.Errorf("PruneDays = %d, want %d", cmd.PruneDays, tt.wantPruneDays)
			}
			if cmd.DeleteID != tt.wantDeleteID {
				t.Errorf("DeleteID = %q, want %q", cmd.DeleteID, tt.wantDeleteID)
			}
		})
	}
}

// TestParseArgs_V5SnapshotFlag tests the --snapshot flag for run
func TestParseArgs_V5SnapshotFlag(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantTarget   string
		wantSnapshot bool
	}{
		{
			name:         "run with --snapshot",
			args:         []string{"run", "--snapshot", "echo"},
			wantTarget:   "echo",
			wantSnapshot: true,
		},
		{
			name:         "run without --snapshot",
			args:         []string{"run", "echo"},
			wantTarget:   "echo",
			wantSnapshot: false,
		},
		{
			name:         "run with --snapshot and other flags",
			args:         []string{"run", "--snapshot", "--execution-id", "node", "app.js"},
			wantTarget:   "node",
			wantSnapshot: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", cmd.Target, tt.wantTarget)
			}
			if cmd.Snapshot != tt.wantSnapshot {
				t.Errorf("Snapshot = %v, want %v", cmd.Snapshot, tt.wantSnapshot)
			}
		})
	}
}

// Feature: admit-v5-execution-replay, Property 10: Backward Compatibility
// Validates: Requirements 7.1, 7.2, 7.3
// For any v4-compatible invocation (no v5 flags), behavior SHALL be identical to v4.
func TestParseArgs_V5BackwardCompatibility_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: v4 invocations still work
	properties.Property("v4 invocations parse correctly without v5 flags set", prop.ForAll(
		func(target string, args []string) bool {
			if target == "" {
				return true // Skip empty targets
			}
			// Build v4-style args: ["run", target, arg1, arg2, ...]
			inputArgs := append([]string{"run", target}, args...)
			cmd, err := ParseArgs(inputArgs)
			if err != nil {
				return false
			}
			// Verify subcommand is run
			if cmd.Subcommand != SubcommandRun {
				return false
			}
			// Verify target is preserved
			if cmd.Target != target {
				return false
			}
			// Verify args are preserved
			if len(cmd.Args) != len(args) {
				return false
			}
			for i, arg := range args {
				if cmd.Args[i] != arg {
					return false
				}
			}
			// Verify v5 flags are not set
			if cmd.Snapshot || cmd.ReplayID != "" || cmd.PruneDays != 0 || cmd.DeleteID != "" {
				return false
			}
			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Property: v4 execution-id flags still work
	properties.Property("v4 execution-id flags work independently of v5 flags", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			args := []string{"run", "--execution-id", "--execution-id-json", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.ExecutionID && cmd.ExecutionIDJSON
		},
		gen.Identifier(),
	))

	// Property: v4 and v5 flags can be used together
	properties.Property("v4 and v5 flags can coexist", prop.ForAll(
		func(target string) bool {
			if target == "" {
				return true
			}
			args := []string{"run", "--execution-id", "--snapshot", target}
			cmd, err := ParseArgs(args)
			if err != nil {
				return false
			}
			return cmd.Target == target && cmd.ExecutionID && cmd.Snapshot
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}
