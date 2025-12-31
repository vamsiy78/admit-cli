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
	SubcommandRun       Subcommand = "run"
	SubcommandCheck     Subcommand = "check"
	SubcommandReplay    Subcommand = "replay"    // v5: replay an execution
	SubcommandSnapshots Subcommand = "snapshots" // v5: list/manage snapshots
	SubcommandBaseline  Subcommand = "baseline"  // v6: manage baselines
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

	// v4 Execution Identity flags
	ExecutionID     bool   // --execution-id
	ExecutionIDJSON bool   // --execution-id-json
	ExecutionIDFile string // --execution-id-file <path>
	ExecutionIDEnv  string // --execution-id-env <varname>

	// v5 Snapshot flags
	Snapshot  bool   // --snapshot (for run)
	ReplayID  string // execution ID for replay subcommand
	PruneDays int    // --prune <days> (for snapshots)
	DeleteID  string // --delete <execution_id> (for snapshots)

	// v6 Drift Detection flags
	Baseline       string // --baseline [name] (store baseline, default: "default")
	DetectDrift    string // --detect-drift [name] (compare against baseline)
	DriftJSON      bool   // --drift-json (output drift as JSON)
	BaselineAction string // "list", "show", "delete" for baseline subcommand
	BaselineName   string // name argument for baseline show/delete

	// v7 Environment Contract flags
	Env          string // --env <name> (environment for contract evaluation)
	ContractJSON bool   // --contract-json (output contract violations as JSON)
}

// ParseArgs parses CLI arguments into a Command.
// It expects args to be os.Args[1:] (excluding the program name).
// Returns error if no command provided after "run".
func ParseArgs(args []string) (Command, error) {
	// Need at least a subcommand
	if len(args) == 0 {
		return Command{}, ErrNoRunSubcommand
	}

	// First arg must be a valid subcommand
	subcommand := args[0]
	switch subcommand {
	case "run", "check", "replay", "snapshots", "baseline":
		// Valid subcommands
	default:
		return Command{}, ErrNoRunSubcommand
	}

	cmd := Command{
		Subcommand: Subcommand(subcommand),
	}

	// Handle replay subcommand: admit replay <execution_id> [flags]
	if subcommand == "replay" {
		return parseReplayArgs(args[1:], cmd)
	}

	// Handle snapshots subcommand: admit snapshots [flags]
	if subcommand == "snapshots" {
		return parseSnapshotsArgs(args[1:], cmd)
	}

	// Handle baseline subcommand: admit baseline list|show|delete [name]
	if subcommand == "baseline" {
		return parseBaselineArgs(args[1:], cmd)
	}

	// Parse flags and find the command (for run/check)
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
			case "execution-id":
				cmd.ExecutionID = true
			case "execution-id-json":
				cmd.ExecutionIDJSON = true
			case "execution-id-file":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.ExecutionIDFile = args[i]
			case "execution-id-env":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.ExecutionIDEnv = args[i]
			case "snapshot":
				cmd.Snapshot = true
			case "baseline":
				// --baseline [name] - optional name, default "default"
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					cmd.Baseline = args[i]
				} else {
					cmd.Baseline = "default"
				}
			case "detect-drift":
				// --detect-drift [name] - optional name, default "default"
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					cmd.DetectDrift = args[i]
				} else {
					cmd.DetectDrift = "default"
				}
			case "drift-json":
				cmd.DriftJSON = true
			case "env":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.Env = args[i]
			case "contract-json":
				cmd.ContractJSON = true
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

// parseReplayArgs parses arguments for the replay subcommand.
func parseReplayArgs(args []string, cmd Command) (Command, error) {
	i := 0

	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")
			switch flagName {
			case "dry-run":
				cmd.DryRun = true
			case "json":
				cmd.JSONOutput = true
			case "schema":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.SchemaPath = args[i]
			default:
				// Unknown flag
			}
			i++
			continue
		}

		// Not a flag - this is the execution ID
		if cmd.ReplayID == "" {
			cmd.ReplayID = arg
		}
		i++
	}

	// Replay requires an execution ID
	if cmd.ReplayID == "" {
		return Command{}, errors.New("replay requires an execution ID: usage: admit replay <execution_id>")
	}

	return cmd, nil
}

// parseSnapshotsArgs parses arguments for the snapshots subcommand.
func parseSnapshotsArgs(args []string, cmd Command) (Command, error) {
	i := 0

	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")
			switch flagName {
			case "json":
				cmd.JSONOutput = true
			case "prune":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				days, err := parseInt(args[i])
				if err != nil {
					return Command{}, errors.New("--prune requires a number of days")
				}
				cmd.PruneDays = days
			case "delete":
				if i+1 >= len(args) {
					return Command{}, ErrMissingFlagValue
				}
				i++
				cmd.DeleteID = args[i]
			default:
				// Unknown flag
			}
			i++
			continue
		}

		i++
	}

	return cmd, nil
}

// parseInt parses a string to int.
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}


// parseBaselineArgs parses arguments for the baseline subcommand.
func parseBaselineArgs(args []string, cmd Command) (Command, error) {
	if len(args) == 0 {
		return Command{}, errors.New("baseline requires an action: usage: admit baseline <list|show|delete> [name]")
	}

	action := args[0]
	switch action {
	case "list":
		cmd.BaselineAction = "list"
		// Parse optional --json flag
		for i := 1; i < len(args); i++ {
			if args[i] == "--json" {
				cmd.JSONOutput = true
			}
		}
	case "show":
		cmd.BaselineAction = "show"
		if len(args) < 2 {
			return Command{}, errors.New("baseline show requires a name: usage: admit baseline show <name>")
		}
		cmd.BaselineName = args[1]
		// Parse optional --json flag
		for i := 2; i < len(args); i++ {
			if args[i] == "--json" {
				cmd.JSONOutput = true
			}
		}
	case "delete":
		cmd.BaselineAction = "delete"
		if len(args) < 2 {
			return Command{}, errors.New("baseline delete requires a name: usage: admit baseline delete <name>")
		}
		cmd.BaselineName = args[1]
	default:
		return Command{}, errors.New("unknown baseline action: usage: admit baseline <list|show|delete> [name]")
	}

	return cmd, nil
}
