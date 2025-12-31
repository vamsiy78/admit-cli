package contract

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-v7-environment-contracts, Property 6: Glob Pattern Matching
// For any deny pattern containing `*` and any value, the pattern SHALL match
// if the value contains the literal parts of the pattern with any characters
// in place of `*`. Patterns without `*` SHALL only match exact strings.
// **Validates: Requirements 5.1, 5.2, 5.3**
func TestProperty6_GlobPatternMatching(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Property: Patterns without * only match exact strings
	properties.Property("patterns without * only match exact strings", prop.ForAll(
		func(pattern string, value string) bool {
			if strings.Contains(pattern, "*") {
				return true // Skip patterns with wildcards for this property
			}

			result := MatchGlob(pattern, value)
			expected := pattern == value

			if result != expected {
				t.Logf("Exact match violation: pattern=%q, value=%q, got %v, want %v",
					pattern, value, result, expected)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: Pattern "*" matches any string
	properties.Property("pattern * matches any string", prop.ForAll(
		func(value string) bool {
			result := MatchGlob("*", value)
			if !result {
				t.Logf("Wildcard * should match any string, got false for %q", value)
				return false
			}
			return true
		},
		genSimpleString,
	))

	// Property: Prefix pattern "prefix*" matches strings starting with prefix
	properties.Property("prefix* matches strings starting with prefix", prop.ForAll(
		func(prefix string, suffix string) bool {
			if len(prefix) == 0 {
				return true // Skip empty prefix
			}
			pattern := prefix + "*"
			value := prefix + suffix

			result := MatchGlob(pattern, value)
			if !result {
				t.Logf("Prefix pattern %q should match %q", pattern, value)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: Suffix pattern "*suffix" matches strings ending with suffix
	properties.Property("*suffix matches strings ending with suffix", prop.ForAll(
		func(prefix string, suffix string) bool {
			if len(suffix) == 0 {
				return true // Skip empty suffix
			}
			pattern := "*" + suffix
			value := prefix + suffix

			result := MatchGlob(pattern, value)
			if !result {
				t.Logf("Suffix pattern %q should match %q", pattern, value)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: Infix pattern "*middle*" matches strings containing middle
	properties.Property("*middle* matches strings containing middle", prop.ForAll(
		func(prefix string, middle string, suffix string) bool {
			if len(middle) == 0 {
				return true // Skip empty middle
			}
			pattern := "*" + middle + "*"
			value := prefix + middle + suffix

			result := MatchGlob(pattern, value)
			if !result {
				t.Logf("Infix pattern %q should match %q", pattern, value)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 2: Allow Rule Violation Detection
// For any config key with an allow rule and a resolved value NOT in the allowed
// values list, the evaluator SHALL generate a violation with ruleType "allow".
// **Validates: Requirements 2.2**
func TestProperty2_AllowRuleViolationDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Generator for non-empty list of allowed values
	genAllowedValues := gen.SliceOfN(3, genSimpleString).Map(func(vals []string) []string {
		// Ensure unique values
		seen := make(map[string]bool)
		result := []string{}
		for _, v := range vals {
			if !seen[v] {
				seen[v] = true
				result = append(result, v)
			}
		}
		if len(result) == 0 {
			return []string{"allowed1", "allowed2"}
		}
		return result
	})

	// Property: Value in allow list produces no violation
	properties.Property("value in allow list produces no violation", prop.ForAll(
		func(key string, allowedValues []string) bool {
			if len(allowedValues) == 0 {
				return true
			}
			// Pick first allowed value as the actual value
			value := allowedValues[0]
			rule := Rule{Values: allowedValues}

			violation := CheckAllowRule(key, value, rule)
			if violation != nil {
				t.Logf("Value %q in allow list %v should not produce violation", value, allowedValues)
				return false
			}
			return true
		},
		genSimpleString,
		genAllowedValues,
	))

	// Property: Value NOT in allow list produces violation with ruleType "allow"
	properties.Property("value not in allow list produces allow violation", prop.ForAll(
		func(key string, allowedValues []string, actualValue string) bool {
			if len(allowedValues) == 0 {
				return true
			}
			// Ensure actualValue is not in allowedValues
			for _, v := range allowedValues {
				if v == actualValue {
					return true // Skip this case
				}
			}

			rule := Rule{Values: allowedValues}
			violation := CheckAllowRule(key, actualValue, rule)

			if violation == nil {
				t.Logf("Value %q not in allow list %v should produce violation", actualValue, allowedValues)
				return false
			}
			if violation.RuleType != "allow" {
				t.Logf("Violation ruleType should be 'allow', got %q", violation.RuleType)
				return false
			}
			if violation.Key != key {
				t.Logf("Violation key should be %q, got %q", key, violation.Key)
				return false
			}
			if violation.ActualValue != actualValue {
				t.Logf("Violation actualValue should be %q, got %q", actualValue, violation.ActualValue)
				return false
			}
			return true
		},
		genSimpleString,
		genAllowedValues,
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 3: Deny Rule Violation Detection
// For any config key with a deny rule and a resolved value matching any deny pattern,
// the evaluator SHALL generate a violation with ruleType "deny" and the matching pattern.
// **Validates: Requirements 2.3**
func TestProperty3_DenyRuleViolationDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Property: Value matching exact deny pattern produces violation
	properties.Property("value matching exact deny pattern produces deny violation", prop.ForAll(
		func(key string, deniedValue string) bool {
			if len(deniedValue) == 0 {
				return true
			}
			rule := Rule{Values: []string{deniedValue}, IsGlob: false}

			violation := CheckDenyRule(key, deniedValue, rule)

			if violation == nil {
				t.Logf("Value %q matching deny pattern should produce violation", deniedValue)
				return false
			}
			if violation.RuleType != "deny" {
				t.Logf("Violation ruleType should be 'deny', got %q", violation.RuleType)
				return false
			}
			if violation.Pattern != deniedValue {
				t.Logf("Violation pattern should be %q, got %q", deniedValue, violation.Pattern)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: Value matching glob deny pattern produces violation
	properties.Property("value matching glob deny pattern produces deny violation", prop.ForAll(
		func(key string, prefix string, suffix string) bool {
			if len(prefix) == 0 || len(suffix) == 0 {
				return true
			}
			// Create a glob pattern that matches prefix*suffix
			pattern := prefix + "*" + suffix
			value := prefix + "middle" + suffix
			rule := Rule{Values: []string{pattern}, IsGlob: true}

			violation := CheckDenyRule(key, value, rule)

			if violation == nil {
				t.Logf("Value %q matching glob pattern %q should produce violation", value, pattern)
				return false
			}
			if violation.RuleType != "deny" {
				t.Logf("Violation ruleType should be 'deny', got %q", violation.RuleType)
				return false
			}
			if violation.Pattern != pattern {
				t.Logf("Violation pattern should be %q, got %q", pattern, violation.Pattern)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	// Property: Value NOT matching deny pattern produces no violation
	properties.Property("value not matching deny pattern produces no violation", prop.ForAll(
		func(key string, deniedValue string, actualValue string) bool {
			if deniedValue == actualValue {
				return true // Skip when values match
			}
			rule := Rule{Values: []string{deniedValue}, IsGlob: false}

			violation := CheckDenyRule(key, actualValue, rule)

			if violation != nil {
				t.Logf("Value %q not matching deny pattern %q should not produce violation", actualValue, deniedValue)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 4: Deny Precedence Over Allow
// For any config key with both allow and deny rules where the value is in the
// allow list AND matches a deny pattern, the evaluator SHALL generate a violation
// (deny wins).
// **Validates: Requirements 2.4**
func TestProperty4_DenyPrecedenceOverAllow(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Property: When value is both allowed and denied, deny takes precedence
	properties.Property("deny takes precedence over allow", prop.ForAll(
		func(key string, value string) bool {
			if len(value) == 0 {
				return true
			}

			// Create a contract where value is both allowed AND denied
			contract := Contract{
				Name: "test",
				Allow: map[string]Rule{
					key: {Values: []string{value}},
				},
				Deny: map[string]Rule{
					key: {Values: []string{value}, IsGlob: false},
				},
			}

			configValues := map[string]string{key: value}
			result := Evaluate(contract, configValues)

			// Deny should take precedence - result should be a violation
			if result.Passed {
				t.Logf("Value %q is both allowed and denied - deny should take precedence", value)
				return false
			}
			if len(result.Violations) == 0 {
				t.Logf("Expected violation when value is both allowed and denied")
				return false
			}
			if result.Violations[0].RuleType != "deny" {
				t.Logf("Violation should be 'deny' type, got %q", result.Violations[0].RuleType)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: When value is allowed but matches deny glob, deny takes precedence
	properties.Property("deny glob takes precedence over allow", prop.ForAll(
		func(key string, prefix string) bool {
			if len(prefix) == 0 {
				return true
			}

			value := prefix + "suffix"
			pattern := prefix + "*"

			// Create a contract where value is allowed but matches deny glob
			contract := Contract{
				Name: "test",
				Allow: map[string]Rule{
					key: {Values: []string{value}},
				},
				Deny: map[string]Rule{
					key: {Values: []string{pattern}, IsGlob: true},
				},
			}

			configValues := map[string]string{key: value}
			result := Evaluate(contract, configValues)

			// Deny should take precedence
			if result.Passed {
				t.Logf("Value %q matches deny glob %q - deny should take precedence", value, pattern)
				return false
			}
			if len(result.Violations) == 0 {
				t.Logf("Expected violation when value matches deny glob")
				return false
			}
			if result.Violations[0].RuleType != "deny" {
				t.Logf("Violation should be 'deny' type, got %q", result.Violations[0].RuleType)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 5: Unmentioned Keys Pass
// For any config key NOT mentioned in the contract (neither allow nor deny),
// the evaluator SHALL NOT generate a violation regardless of the key's value.
// **Validates: Requirements 2.5**
func TestProperty5_UnmentionedKeysPass(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric strings
	genSimpleString := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		if len(s) > 20 {
			return s[:20]
		}
		return s
	})

	// Property: Keys not in contract produce no violations
	properties.Property("unmentioned keys pass without violation", prop.ForAll(
		func(mentionedKey string, unmentionedKey string, value string) bool {
			if mentionedKey == unmentionedKey {
				return true // Skip when keys are the same
			}

			// Create a contract that only mentions one key
			contract := Contract{
				Name: "test",
				Allow: map[string]Rule{
					mentionedKey: {Values: []string{"allowed"}},
				},
				Deny: map[string]Rule{},
			}

			// Config has only the unmentioned key
			configValues := map[string]string{unmentionedKey: value}
			result := Evaluate(contract, configValues)

			// Should pass - unmentioned key should not cause violation
			if !result.Passed {
				t.Logf("Unmentioned key %q should not cause violation", unmentionedKey)
				return false
			}
			if len(result.Violations) != 0 {
				t.Logf("Expected no violations for unmentioned key, got %d", len(result.Violations))
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	// Property: Empty contract allows any config
	properties.Property("empty contract allows any config", prop.ForAll(
		func(key string, value string) bool {
			// Create an empty contract
			contract := Contract{
				Name:  "test",
				Allow: map[string]Rule{},
				Deny:  map[string]Rule{},
			}

			configValues := map[string]string{key: value}
			result := Evaluate(contract, configValues)

			if !result.Passed {
				t.Logf("Empty contract should allow any config")
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
	))

	// Property: Contract with rules for other keys doesn't affect unmentioned keys
	properties.Property("rules for other keys don't affect unmentioned keys", prop.ForAll(
		func(key1 string, key2 string, value1 string, value2 string) bool {
			if key1 == key2 {
				return true
			}

			// Create a contract with strict rules for key1
			contract := Contract{
				Name: "test",
				Allow: map[string]Rule{
					key1: {Values: []string{"only-this"}},
				},
				Deny: map[string]Rule{
					key1: {Values: []string{"*bad*"}, IsGlob: true},
				},
			}

			// Config has key2 with any value - should pass
			configValues := map[string]string{key2: value2}
			result := Evaluate(contract, configValues)

			if !result.Passed {
				t.Logf("Key %q not in contract should pass regardless of value", key2)
				return false
			}
			return true
		},
		genSimpleString,
		genSimpleString,
		genSimpleString,
		genSimpleString,
	))

	properties.TestingRun(t)
}


// Feature: admit-v7-environment-contracts, Property 9: All Violations Reported
// For any config with multiple contract violations, the evaluator SHALL report
// ALL violations in the result (not short-circuit on first).
// **Validates: Requirements 3.6**
func TestProperty9_AllViolationsReported(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for number of violations (2-5)
	genNumViolations := gen.IntRange(2, 5)

	// Property: Multiple allow violations are all reported
	properties.Property("multiple allow violations are all reported", prop.ForAll(
		func(numViolations int) bool {
			// Create a contract with multiple allow rules
			contract := Contract{
				Name:  "test",
				Allow: map[string]Rule{},
				Deny:  map[string]Rule{},
			}

			configValues := map[string]string{}

			// Create numViolations keys, each with a value not in allow list
			for i := 0; i < numViolations; i++ {
				key := "key" + string(rune('a'+i))
				contract.Allow[key] = Rule{Values: []string{"allowed"}}
				configValues[key] = "not-allowed"
			}

			result := Evaluate(contract, configValues)

			if result.Passed {
				t.Logf("Expected violations but result passed")
				return false
			}
			if len(result.Violations) != numViolations {
				t.Logf("Expected %d violations, got %d", numViolations, len(result.Violations))
				return false
			}
			return true
		},
		genNumViolations,
	))

	// Property: Multiple deny violations are all reported
	properties.Property("multiple deny violations are all reported", prop.ForAll(
		func(numViolations int) bool {
			// Create a contract with multiple deny rules
			contract := Contract{
				Name:  "test",
				Allow: map[string]Rule{},
				Deny:  map[string]Rule{},
			}

			configValues := map[string]string{}

			// Create numViolations keys, each matching a deny rule
			for i := 0; i < numViolations; i++ {
				key := "key" + string(rune('a'+i))
				contract.Deny[key] = Rule{Values: []string{"denied"}, IsGlob: false}
				configValues[key] = "denied"
			}

			result := Evaluate(contract, configValues)

			if result.Passed {
				t.Logf("Expected violations but result passed")
				return false
			}
			if len(result.Violations) != numViolations {
				t.Logf("Expected %d violations, got %d", numViolations, len(result.Violations))
				return false
			}
			return true
		},
		genNumViolations,
	))

	// Property: Mixed allow and deny violations are all reported
	properties.Property("mixed allow and deny violations are all reported", prop.ForAll(
		func(numAllow int, numDeny int) bool {
			if numAllow < 1 || numDeny < 1 {
				return true
			}

			contract := Contract{
				Name:  "test",
				Allow: map[string]Rule{},
				Deny:  map[string]Rule{},
			}

			configValues := map[string]string{}

			// Create allow violations
			for i := 0; i < numAllow; i++ {
				key := "allow" + string(rune('a'+i))
				contract.Allow[key] = Rule{Values: []string{"allowed"}}
				configValues[key] = "not-allowed"
			}

			// Create deny violations
			for i := 0; i < numDeny; i++ {
				key := "deny" + string(rune('a'+i))
				contract.Deny[key] = Rule{Values: []string{"denied"}, IsGlob: false}
				configValues[key] = "denied"
			}

			result := Evaluate(contract, configValues)

			expectedViolations := numAllow + numDeny
			if result.Passed {
				t.Logf("Expected violations but result passed")
				return false
			}
			if len(result.Violations) != expectedViolations {
				t.Logf("Expected %d violations, got %d", expectedViolations, len(result.Violations))
				return false
			}
			return true
		},
		gen.IntRange(1, 3),
		gen.IntRange(1, 3),
	))

	properties.TestingRun(t)
}
