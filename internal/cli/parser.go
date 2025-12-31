package cli

import (
	"errors"
	"strings"
)

// ErrNoCommand is returned when no command is provided after "run"
var ErrNoCommand = errors.New("no command provided: usage: admit run [flags] <command> [args...]")

// ErrNoRunSubcommand is returned when "run" subcommand is not provided
var ErrNoRunSubcommand = errors.New("missing subcommand: usage: admit <run|check> [flags] [command] [args...]")

// ErrMissingFlagValue is returned when a flag requires a value but none is provided
var ErrMissingFlagValue = errors.New("flag requires a value")

// Subcommand represents the CLI subcommand
type Subcommand string

const (
	SubcommandRun   Subcommand = "run"
	SubcommandCheck Subcommand = "check"
)

// Command represents the parsed CLI input
type Command struct {
	Subcommand Subcommand // "run" or "check"
	Target     string     // The command to execute (e.g., "node") - only for run
	Args       []string   // Arguments to pass (e.g., ["index.js"]) - only for run

	// Artifact flags
	ArtifactFile   string // --artifact-file <path>
	ArtifactStdout bool   // --artifact-stdout
	ArtifactLog    bool   // --artifact-log

	// Injection flags
	InjectFile string // --inject-file <path>
	InjectEnv  string // --inject-env <varname>

	// Identity flags
	Identity      bool   // --identity
	IdentityFile  string // --identity-file <path>
	IdentityShort bool   // --identity-short

	// Invariant flags
	InvariantsJSON bool // --invariants-json

	// v3 flags
	SchemaPath string // --schema <path>
	DryRun     bool   // --dry-run
	CIMode     bool   // --ci
	JSONOutput bool   // --json (for check subcommand)
}

// ParseArgs parses CLI arguments into a Command.
// It expects args to be os.Args[1:] (excluding the program name).
// Returns error if no command provided after "run".
func ParseArgs(args []string) (Command, error) {
	// Need at least a subcommand
	if len(args) == 0 {
		return Command{}, ErrNoRunSubcommand
	}

	// First arg must be "run" or "check"
	subcommand := args[0]
	if subcommand != "run" && subcommand != "check" {
		return Command{}, ErrNoRunSubcommand
	}

	cmd := Command{
		Subcommand: Subcommand(subcommand),
	}

	// Parse flags and find the command
	i := 1 // Start after subcommand

	for i < len(args) {
		arg := args[i]

		// Check if this is a flag
		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")

			// Handle flags that take values
			switch flagName {
			case "artifact-file":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.ArtifactFile = args[i]
			case "artifact-stdout":
				cmd.ArtifactStdout = true
			case "artifact-log":
				cmd.ArtifactLog = true
			case "inject-file":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.InjectFile = args[i]
			case "inject-env":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.InjectEnv = args[i]
			case "identity":
				cmd.Identity = true
			case "identity-file":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.IdentityFile = args[i]
			case "identity-short":
				cmd.IdentityShort = true
			case "invariants-json":
				cmd.InvariantsJSON = true
			case "schema":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.SchemaPath = args[i]
			case "dry-run":
				cmd.DryRun = true
			case "ci":
				cmd.CIMode = true
			case "json":
				cmd.JSONOutput = true
			default:
				// Unknown flag - treat as start of command
				break
			}
			i++
			continue
		}

		// Not a flag - this is the command (only for run subcommand)
		cmd.Target = arg
		if i+1 < len(args) {
			cmd.Args = args[i+1:]
		}
		break
	}

	// Check we have a command (only required for run subcommand, unless dry-run)
	if cmd.Subcommand == SubcommandRun && cmd.Target == "" && !cmd.DryRun {
		return Command{}, ErrNoCommand
	}

	return cmd, nil
}
