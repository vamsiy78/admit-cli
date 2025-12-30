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
