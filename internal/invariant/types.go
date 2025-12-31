package invariant

// RuleExpr represents a parsed rule expression in the AST
type RuleExpr interface {
	isRuleExpr()
}

// CompOp represents a comparison operator
type CompOp string

const (
	OpEqual    CompOp = "=="
	OpNotEqual CompOp = "!="
)

// Implication represents an implication expression: A => B (if A then B)
// The implication is true if the antecedent is false OR the consequent is true
type Implication struct {
	Antecedent RuleExpr // Left side (condition)
	Consequent RuleExpr // Right side (must be true if condition is true)
}

func (Implication) isRuleExpr() {}

// Comparison represents a comparison expression: A == B or A != B
type Comparison struct {
	Left     RuleExpr
	Right    RuleExpr
	Operator CompOp
}

func (Comparison) isRuleExpr() {}

// ConfigRef represents a reference to a config value using dot notation (e.g., "db.url.env")
type ConfigRef struct {
	Path string
}

func (ConfigRef) isRuleExpr() {}

// ExecutionEnv represents the special reference to execution.env
// This resolves to the ADMIT_ENV environment variable at runtime
type ExecutionEnv struct{}

func (ExecutionEnv) isRuleExpr() {}

// StringLiteral represents a quoted string value (e.g., "prod")
type StringLiteral struct {
	Value string
}

func (StringLiteral) isRuleExpr() {}

// Invariant represents a named invariant rule
type Invariant struct {
	Name string   // Unique identifier (e.g., "prod-db-guard")
	Rule string   // Original rule string
	Expr RuleExpr // Parsed expression
}

// InvariantResult represents the evaluation result of an invariant
type InvariantResult struct {
	Name       string // Invariant name
	Rule       string // Original rule expression
	Passed     bool   // Whether the invariant passed
	LeftValue  string // Evaluated left operand value
	RightValue string // Evaluated right operand value
	Message    string // Human-readable explanation
}
