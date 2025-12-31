package invariant

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestParseRule_SimpleComparison(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr bool
	}{
		{
			name:    "equality with string literal",
			rule:    `db.url.env == "prod"`,
			wantErr: false,
		},
		{
			name:    "inequality with string literal",
			rule:    `execution.env != "prod"`,
			wantErr: false,
		},
		{
			name:    "config ref comparison",
			rule:    `db.url.env == region.env`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRule(tt.rule, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRule_Implication(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr bool
	}{
		{
			name:    "implication with =>",
			rule:    `execution.env == "prod" => db.url.env == "prod"`,
			wantErr: false,
		},
		{
			name:    "implication with ⇒",
			rule:    `execution.env == "prod" ⇒ db.url.env == "prod"`,
			wantErr: false,
		},
		{
			name:    "implication with inequality",
			rule:    `execution.env != "prod" => payments.mode != "live"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRule(tt.rule, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


func TestParseRule_ExecutionEnv(t *testing.T) {
	rule := `execution.env == "prod"`
	expr, err := ParseRule(rule, nil)
	if err != nil {
		t.Fatalf("ParseRule() error = %v", err)
	}

	comp, ok := expr.(Comparison)
	if !ok {
		t.Fatalf("expected Comparison, got %T", expr)
	}

	if _, ok := comp.Left.(ExecutionEnv); !ok {
		t.Errorf("expected ExecutionEnv on left, got %T", comp.Left)
	}
}

func TestParseRule_Errors(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr string
	}{
		{
			name:    "empty rule",
			rule:    "",
			wantErr: "empty rule expression",
		},
		{
			name:    "unterminated string",
			rule:    `db.url == "prod`,
			wantErr: "unterminated string literal",
		},
		{
			name:    "unexpected character",
			rule:    `db.url @ "prod"`,
			wantErr: "unexpected character",
		},
		{
			name:    "missing operand after dot",
			rule:    `db. == "prod"`,
			wantErr: "expected identifier after '.'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRule(tt.rule, nil)
			if err == nil {
				t.Errorf("ParseRule() expected error containing %q", tt.wantErr)
				return
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseRule() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateRuleRefs(t *testing.T) {
	tests := []struct {
		name       string
		rule       string
		configKeys []string
		wantErr    bool
	}{
		{
			name:       "valid refs",
			rule:       `db.url.env == "prod"`,
			configKeys: []string{"db.url.env"},
			wantErr:    false,
		},
		{
			name:       "undefined ref",
			rule:       `db.url.env == "prod"`,
			configKeys: []string{"other.key"},
			wantErr:    true,
		},
		{
			name:       "execution.env not validated",
			rule:       `execution.env == "prod"`,
			configKeys: []string{},
			wantErr:    false,
		},
		{
			name:       "string literal not validated",
			rule:       `"prod" == "prod"`,
			configKeys: []string{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseRule(tt.rule, nil)
			if err != nil {
				t.Fatalf("ParseRule() error = %v", err)
			}

			err = ValidateRuleRefs(expr, tt.configKeys)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRuleRefs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


func TestFormatRule(t *testing.T) {
	tests := []struct {
		name     string
		expr     RuleExpr
		expected string
	}{
		{
			name:     "string literal",
			expr:     StringLiteral{Value: "prod"},
			expected: `"prod"`,
		},
		{
			name:     "config ref",
			expr:     ConfigRef{Path: "db.url.env"},
			expected: "db.url.env",
		},
		{
			name:     "execution env",
			expr:     ExecutionEnv{},
			expected: "execution.env",
		},
		{
			name: "equality comparison",
			expr: Comparison{
				Left:     ConfigRef{Path: "db.url.env"},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpEqual,
			},
			expected: `db.url.env == "prod"`,
		},
		{
			name: "inequality comparison",
			expr: Comparison{
				Left:     ExecutionEnv{},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpNotEqual,
			},
			expected: `execution.env != "prod"`,
		},
		{
			name: "implication",
			expr: Implication{
				Antecedent: Comparison{
					Left:     ExecutionEnv{},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpEqual,
				},
				Consequent: Comparison{
					Left:     ConfigRef{Path: "db.url.env"},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpEqual,
				},
			},
			expected: `execution.env == "prod" => db.url.env == "prod"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatRule(tt.expr)
			if result != tt.expected {
				t.Errorf("FormatRule() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Feature: admit-v2-invariants, Property 3: Rule Expression Round-Trip
// For any valid RuleExpr AST, formatting it to a string and parsing it back
// SHALL produce an equivalent AST.
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.8, 2.9**
func TestProperty3_RuleExpressionRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid identifier parts (simple lowercase letters)
	genIdentPart := gen.AnyString().Map(func(s string) string {
		// Generate simple identifiers like "a", "ab", "abc"
		if len(s) == 0 {
			return "x"
		}
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 10; i++ {
			ch := s[i]
			if ch >= 'a' && ch <= 'z' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "x"
		}
		return string(result)
	})

	// Generator for config ref paths (1-3 dot-separated parts)
	genConfigPath := gen.SliceOfN(3, genIdentPart).Map(func(parts []string) string {
		// Filter out empty parts and join
		var validParts []string
		for _, p := range parts {
			if p != "" {
				validParts = append(validParts, p)
			}
		}
		if len(validParts) == 0 {
			return "config"
		}
		result := validParts[0]
		for i := 1; i < len(validParts); i++ {
			result += "." + validParts[i]
		}
		// Avoid "execution.env" as a config path since it's special
		if result == "execution.env" {
			return "config.env"
		}
		return result
	})

	// Generator for string literal values (simple alphanumeric)
	genStringValue := gen.AnyString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 10; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "value"
		}
		return string(result)
	})

	// Generator for operands (ConfigRef, ExecutionEnv, or StringLiteral)
	genOperand := gen.OneGenOf(
		genConfigPath.Map(func(path string) RuleExpr {
			return ConfigRef{Path: path}
		}),
		gen.Const(ExecutionEnv{}).Map(func(e ExecutionEnv) RuleExpr {
			return e
		}),
		genStringValue.Map(func(val string) RuleExpr {
			return StringLiteral{Value: val}
		}),
	)

	// Generator for comparison operators
	genCompOp := gen.OneConstOf(OpEqual, OpNotEqual)

	// Generator for comparison expressions
	genComparison := gopter.CombineGens(
		genOperand,
		genCompOp,
		genOperand,
	).Map(func(vals []interface{}) RuleExpr {
		return Comparison{
			Left:     vals[0].(RuleExpr),
			Operator: vals[1].(CompOp),
			Right:    vals[2].(RuleExpr),
		}
	})

	// Generator for rule expressions (comparison or implication)
	genRuleExpr := gen.OneGenOf(
		genComparison,
		gopter.CombineGens(genComparison, genComparison).Map(func(vals []interface{}) RuleExpr {
			return Implication{
				Antecedent: vals[0].(RuleExpr),
				Consequent: vals[1].(RuleExpr),
			}
		}),
	)

	properties.Property("round-trip preserves rule expression", prop.ForAll(
		func(original RuleExpr) bool {
			// Format to string
			formatted := FormatRule(original)

			// Parse back (without config key validation)
			parsed, err := ParseRule(formatted, nil)
			if err != nil {
				t.Logf("ParseRule failed for %q: %v", formatted, err)
				return false
			}

			// Compare ASTs
			if !reflect.DeepEqual(original, parsed) {
				t.Logf("Round-trip mismatch:\n  original: %+v\n  formatted: %q\n  parsed: %+v", original, formatted, parsed)
				return false
			}

			return true
		},
		genRuleExpr,
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 4: Undefined Key Detection
// For any rule expression referencing a config key not defined in the schema,
// the Rule_Parser SHALL return an error identifying the undefined key.
// **Validates: Requirements 2.7**
func TestProperty4_UndefinedKeyDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for valid identifier parts
	genIdentPart := gen.AnyString().Map(func(s string) string {
		if len(s) == 0 {
			return "x"
		}
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 10; i++ {
			ch := s[i]
			if ch >= 'a' && ch <= 'z' {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "x"
		}
		return string(result)
	})

	// Generator for config ref paths
	genConfigPath := gen.SliceOfN(3, genIdentPart).Map(func(parts []string) string {
		var validParts []string
		for _, p := range parts {
			if p != "" {
				validParts = append(validParts, p)
			}
		}
		if len(validParts) == 0 {
			return "config"
		}
		result := validParts[0]
		for i := 1; i < len(validParts); i++ {
			result += "." + validParts[i]
		}
		if result == "execution.env" {
			return "config.env"
		}
		return result
	})

	// Generator for a rule key and schema keys where rule key is NOT in schema
	genUndefinedKeyScenario := gopter.CombineGens(
		genConfigPath,
		gen.IntRange(0, 5),
	).Map(func(vals []interface{}) struct {
		ruleKey    string
		schemaKeys []string
	} {
		ruleKey := vals[0].(string)
		numSchemaKeys := vals[1].(int)

		// Generate schema keys that are different from ruleKey
		schemaKeys := make([]string, numSchemaKeys)
		for i := 0; i < numSchemaKeys; i++ {
			// Create keys that are guaranteed to be different
			schemaKeys[i] = "schema" + string(rune('a'+i)) + ".key"
		}

		// Ensure ruleKey is not in schemaKeys by modifying it if needed
		for _, k := range schemaKeys {
			if k == ruleKey {
				ruleKey = "undefined.key.path"
				break
			}
		}

		return struct {
			ruleKey    string
			schemaKeys []string
		}{
			ruleKey:    ruleKey,
			schemaKeys: schemaKeys,
		}
	})

	properties.Property("undefined config key produces error", prop.ForAll(
		func(data struct {
			ruleKey    string
			schemaKeys []string
		}) bool {
			// Create a simple comparison rule using the undefined key
			rule := data.ruleKey + ` == "value"`

			// Parse the rule without validation first
			expr, err := ParseRule(rule, nil)
			if err != nil {
				t.Logf("ParseRule failed unexpectedly: %v", err)
				return false
			}

			// Validate against schema keys - should fail
			err = ValidateRuleRefs(expr, data.schemaKeys)
			if err == nil {
				t.Logf("Expected error for undefined key %q with schema keys %v", data.ruleKey, data.schemaKeys)
				return false
			}

			// Error message should mention the undefined key
			if !contains(err.Error(), data.ruleKey) {
				t.Logf("Error message %q should contain undefined key %q", err.Error(), data.ruleKey)
				return false
			}

			return true
		},
		genUndefinedKeyScenario,
	))

	properties.TestingRun(t)
}
