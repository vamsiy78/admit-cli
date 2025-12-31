package contract

import "strings"

// Evaluate checks resolved config against an environment contract.
// Returns EvalResult with all violations (does not short-circuit).
// Deny rules take precedence over allow rules.
// Keys not mentioned in the contract pass without violation.
func Evaluate(c Contract, configValues map[string]string) EvalResult {
	result := EvalResult{
		Environment: c.Name,
		Passed:      true,
		Violations:  []Violation{},
	}

	// Check all config values against contract rules
	for key, value := range configValues {
		// Check deny rules first (deny takes precedence)
		if denyRule, hasDeny := c.Deny[key]; hasDeny {
			if violation := CheckDenyRule(key, value, denyRule); violation != nil {
				result.Violations = append(result.Violations, *violation)
				result.Passed = false
				continue // Deny violation found, skip allow check for this key
			}
		}

		// Check allow rules (only if no deny violation)
		if allowRule, hasAllow := c.Allow[key]; hasAllow {
			if violation := CheckAllowRule(key, value, allowRule); violation != nil {
				result.Violations = append(result.Violations, *violation)
				result.Passed = false
			}
		}
		// Keys not mentioned in contract pass without violation
	}

	return result
}

// MatchGlob checks if a value matches a glob pattern.
// Supports * wildcard matching any sequence of characters.
// Patterns without * only match exact strings.
func MatchGlob(pattern, value string) bool {
	// If no wildcard, require exact match
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	// Split pattern by * and match parts
	parts := strings.Split(pattern, "*")

	// Handle edge cases
	if len(parts) == 0 {
		return true // Pattern is just "*"
	}

	// Track position in value
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue // Empty part from consecutive * or leading/trailing *
		}

		// Find the part in the remaining value
		idx := strings.Index(value[pos:], part)
		if idx == -1 {
			return false // Part not found
		}

		// For first part, must match at start if pattern doesn't start with *
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}

		// Move position past this match
		pos += idx + len(part)
	}

	// For last part, must match at end if pattern doesn't end with *
	if !strings.HasSuffix(pattern, "*") {
		lastPart := parts[len(parts)-1]
		if lastPart != "" && !strings.HasSuffix(value, lastPart) {
			return false
		}
	}

	return true
}

// CheckAllowRule checks if a value is in the allow list.
// Returns violation if value not in allowed values.
// Allow rules require exact matches (no glob patterns).
func CheckAllowRule(key string, value string, rule Rule) *Violation {
	for _, allowed := range rule.Values {
		if value == allowed {
			return nil // Value is allowed
		}
	}

	// Value not in allow list - violation
	return &Violation{
		Key:            key,
		ActualValue:    value,
		RuleType:       "allow",
		ExpectedValues: rule.Values,
	}
}

// CheckDenyRule checks if a value matches any deny pattern.
// Returns violation if value matches a denied pattern.
func CheckDenyRule(key string, value string, rule Rule) *Violation {
	for _, pattern := range rule.Values {
		var matches bool
		if rule.IsGlob {
			matches = MatchGlob(pattern, value)
		} else {
			matches = (pattern == value)
		}

		if matches {
			return &Violation{
				Key:            key,
				ActualValue:    value,
				RuleType:       "deny",
				ExpectedValues: rule.Values,
				Pattern:        pattern,
			}
		}
	}

	return nil // No deny pattern matched
}
