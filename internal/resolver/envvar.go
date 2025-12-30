package resolver

import "strings"

// PathToEnvVar converts a config path (dot-notation) to an environment variable name.
// e.g., "db.url" -> "DB_URL", "payments.mode" -> "PAYMENTS_MODE"
func PathToEnvVar(path string) string {
	if path == "" {
		return ""
	}
	return strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
}
