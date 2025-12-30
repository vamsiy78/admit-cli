package launcher

import (
	"os"
	"os/exec"
	"syscall"

	"admit/internal/cli"
)

// Exec replaces the current process with the target command.
// This function does not return on success - the current process is replaced.
// On failure, it returns an error.
//
// Parameters:
//   - cmd: The parsed command containing target and arguments
//   - environ: Environment variables to pass to the new process (typically os.Environ())
//
// Error handling:
//   - Command not found: returns error (caller should exit 127)
//   - Permission denied: returns error (caller should exit 126)
//   - Other execve failures: returns error (caller should exit 1)
func Exec(cmd cli.Command, environ []string) error {
	// Look up the full path to the executable
	execPath, err := exec.LookPath(cmd.Target)
	if err != nil {
		return err
	}

	// Build argv: first element is the command name, followed by arguments
	argv := append([]string{cmd.Target}, cmd.Args...)

	// Replace the current process with the target command
	// syscall.Exec does not return on success
	return syscall.Exec(execPath, argv, environ)
}

// IsNotFound checks if the error indicates the command was not found
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return os.IsNotExist(err) || err == exec.ErrNotFound
}

// IsPermissionDenied checks if the error indicates permission was denied
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	return os.IsPermission(err)
}
