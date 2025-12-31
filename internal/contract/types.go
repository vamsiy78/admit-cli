package contract

// Contract represents an environment contract that defines
// allowed and denied configuration states for a named environment.
type Contract struct {
	Name  string          // Environment name (e.g., "prod", "staging")
	Allow map[string]Rule // Allowed values per config key
	Deny  map[string]Rule // Denied patterns per config key
}

// Rule represents allowed or denied values for a config key.
type Rule struct {
	Values []string // Exact values (for allow) or patterns (for deny)
	IsGlob bool     // Whether values contain glob patterns (deny only)
}

// Violation represents a contract violation.
type Violation struct {
	Key            string   // Config key that violated
	ActualValue    string   // The resolved value
	RuleType       string   // "allow" or "deny"
	ExpectedValues []string // What was expected (allow) or forbidden (deny)
	Pattern        string   // The pattern that matched (for deny rules)
}

// EvalResult contains the full contract evaluation result.
type EvalResult struct {
	Environment string      // Environment name evaluated
	Passed      bool        // Whether all rules passed
	Violations  []Violation // List of violations (empty if passed)
}
