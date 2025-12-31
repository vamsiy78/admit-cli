package cli

import (
	"errors"
	"strings"
)

// ErrNoCommand is returned when no command is provided after "run"
var ErrNoCommand = errors.New("no command provided: usage: admit run [flags] <command> [args...]")

// ErrNoRunSubcommand is returned when "run" subcommand is not provided
var ErrNoRunSubcommand = errors.New("missing subcommand: usage: admit run [flags] <command> [args...]")

// ErrMissingFlagValue is returned when a flag requires a value but none is provided
var ErrMissingFlagValue = errors.New("flag requires a value")

// Command represents the parsed CLI input
type Command struct {
	Target string   // The command to execute (e.g., "node")
	Args   []string // Arguments to pass (e.g., ["index.js"])

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
}

// ParseArgs parses CLI arguments into a Command.
// It expects args to be os.Args[1:] (excluding the program name).
// Returns error if no command provided after "run".
func ParseArgs(args []string) (Command, error) {
	// Need at least "run" and a command
	if len(args) == 0 {
		return Command{}, ErrNoRunSubcommand
	}

	// First arg must be "run"
	if args[0] != "run" {
		return Command{}, ErrNoRunSubcommand
	}

	// Parse flags and find the command
	cmd := Command{}
	i := 1 // Start after "run"

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
			default:
				// Unknown flag - treat as start of command
				break
			}
			i++
			continue
		}

		// Not a flag - this is the command
		cmd.Target = arg
		if i+1 < len(args) {
			cmd.Args = args[i+1:]
		}
		break
	}

	// Check we have a command
	if cmd.Target == "" {
		return Command{}, ErrNoCommand
	}

	return cmd, nil
}
