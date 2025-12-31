package invariant

import (
	"fmt"
	"strings"
	"unicode"
)

// tokenType represents the type of a lexical token
type tokenType int

const (
	tokenEOF tokenType = iota
	tokenIdent
	tokenString
	tokenDot
	tokenImply    // => or ⇒
	tokenEqual    // ==
	tokenNotEqual // !=
)

// token represents a lexical token
type token struct {
	typ   tokenType
	value string
}

// lexer tokenizes a rule expression string
type lexer struct {
	input string
	pos   int
}

// newLexer creates a new lexer for the given input
func newLexer(input string) *lexer {
	return &lexer{input: input, pos: 0}
}

// skipWhitespace advances past any whitespace characters
func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

// peek returns the current character without advancing
func (l *lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}


// peekN returns the next n characters without advancing
func (l *lexer) peekN(n int) string {
	end := l.pos + n
	if end > len(l.input) {
		end = len(l.input)
	}
	return l.input[l.pos:end]
}

// advance moves the position forward by n characters
func (l *lexer) advance(n int) {
	l.pos += n
}

// nextToken returns the next token from the input
func (l *lexer) nextToken() (token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return token{typ: tokenEOF}, nil
	}

	// Check for multi-character operators first
	if l.peekN(2) == "=>" {
		l.advance(2)
		return token{typ: tokenImply, value: "=>"}, nil
	}
	if l.peekN(2) == "==" {
		l.advance(2)
		return token{typ: tokenEqual, value: "=="}, nil
	}
	if l.peekN(2) == "!=" {
		l.advance(2)
		return token{typ: tokenNotEqual, value: "!="}, nil
	}

	// Check for Unicode implication arrow ⇒ (3 bytes in UTF-8)
	if strings.HasPrefix(l.input[l.pos:], "⇒") {
		l.advance(len("⇒"))
		return token{typ: tokenImply, value: "⇒"}, nil
	}

	ch := l.peek()

	// Dot
	if ch == '.' {
		l.advance(1)
		return token{typ: tokenDot, value: "."}, nil
	}

	// String literal
	if ch == '"' {
		return l.readString()
	}

	// Identifier
	if isIdentStart(ch) {
		return l.readIdent(), nil
	}

	return token{}, fmt.Errorf("unexpected character '%c' at position %d", ch, l.pos)
}

// readString reads a quoted string literal
func (l *lexer) readString() (token, error) {
	l.advance(1) // skip opening quote
	start := l.pos

	for l.pos < len(l.input) && l.peek() != '"' {
		l.advance(1)
	}

	if l.pos >= len(l.input) {
		return token{}, fmt.Errorf("unterminated string literal")
	}

	value := l.input[start:l.pos]
	l.advance(1) // skip closing quote

	return token{typ: tokenString, value: value}, nil
}

// readIdent reads an identifier
func (l *lexer) readIdent() token {
	start := l.pos
	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.advance(1)
	}
	return token{typ: tokenIdent, value: l.input[start:l.pos]}
}

// isIdentStart returns true if ch can start an identifier
func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

// isIdentChar returns true if ch can be part of an identifier
func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '-'
}


// parser parses rule expressions into an AST
type parser struct {
	lexer   *lexer
	current token
}

// newParser creates a new parser for the given input
func newParser(input string) (*parser, error) {
	p := &parser{lexer: newLexer(input)}
	// Prime the parser with the first token
	tok, err := p.lexer.nextToken()
	if err != nil {
		return nil, err
	}
	p.current = tok
	return p, nil
}

// advance moves to the next token
func (p *parser) advance() error {
	tok, err := p.lexer.nextToken()
	if err != nil {
		return err
	}
	p.current = tok
	return nil
}

// ParseRule parses a rule expression string into an AST
// configKeys is used to validate that all config references exist
func ParseRule(rule string, configKeys []string) (RuleExpr, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, fmt.Errorf("empty rule expression")
	}

	p, err := newParser(rule)
	if err != nil {
		return nil, err
	}

	expr, err := p.parseRule()
	if err != nil {
		return nil, err
	}

	// Ensure we consumed all input
	if p.current.typ != tokenEOF {
		return nil, fmt.Errorf("unexpected token '%s' after expression", p.current.value)
	}

	// Validate config references if configKeys provided
	if configKeys != nil {
		if err := ValidateRuleRefs(expr, configKeys); err != nil {
			return nil, err
		}
	}

	return expr, nil
}

// parseRule parses a rule (implication or comparison)
func (p *parser) parseRule() (RuleExpr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	// Check for implication
	if p.current.typ == tokenImply {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		return Implication{Antecedent: left, Consequent: right}, nil
	}

	return left, nil
}

// parseComparison parses a comparison expression
func (p *parser) parseComparison() (RuleExpr, error) {
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}

	// Check for comparison operator
	if p.current.typ == tokenEqual || p.current.typ == tokenNotEqual {
		op := OpEqual
		if p.current.typ == tokenNotEqual {
			op = OpNotEqual
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		return Comparison{Left: left, Right: right, Operator: op}, nil
	}

	return left, nil
}

// parseOperand parses an operand (config ref, execution.env, or string literal)
func (p *parser) parseOperand() (RuleExpr, error) {
	switch p.current.typ {
	case tokenString:
		value := p.current.value
		if err := p.advance(); err != nil {
			return nil, err
		}
		return StringLiteral{Value: value}, nil

	case tokenIdent:
		return p.parseRef()

	default:
		return nil, fmt.Errorf("expected operand, got '%s'", p.current.value)
	}
}


// parseRef parses a config reference or execution.env
func (p *parser) parseRef() (RuleExpr, error) {
	var parts []string
	parts = append(parts, p.current.value)

	if err := p.advance(); err != nil {
		return nil, err
	}

	// Collect dot-separated parts
	for p.current.typ == tokenDot {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.current.typ != tokenIdent {
			return nil, fmt.Errorf("expected identifier after '.', got '%s'", p.current.value)
		}
		parts = append(parts, p.current.value)
		if err := p.advance(); err != nil {
			return nil, err
		}
	}

	path := strings.Join(parts, ".")

	// Check for special execution.env reference
	if path == "execution.env" {
		return ExecutionEnv{}, nil
	}

	return ConfigRef{Path: path}, nil
}

// FormatRule formats a RuleExpr back to a string representation
func FormatRule(expr RuleExpr) string {
	switch e := expr.(type) {
	case Implication:
		return fmt.Sprintf("%s => %s", FormatRule(e.Antecedent), FormatRule(e.Consequent))
	case Comparison:
		return fmt.Sprintf("%s %s %s", FormatRule(e.Left), e.Operator, FormatRule(e.Right))
	case ConfigRef:
		return e.Path
	case ExecutionEnv:
		return "execution.env"
	case StringLiteral:
		return fmt.Sprintf(`"%s"`, e.Value)
	default:
		return "<unknown>"
	}
}

// ValidateRuleRefs validates that all config references in the expression exist in the schema
func ValidateRuleRefs(expr RuleExpr, configKeys []string) error {
	refs := collectConfigRefs(expr)
	keySet := make(map[string]bool)
	for _, k := range configKeys {
		keySet[k] = true
	}

	var undefined []string
	for _, ref := range refs {
		if !keySet[ref] {
			undefined = append(undefined, ref)
		}
	}

	if len(undefined) > 0 {
		return fmt.Errorf("undefined config key(s): %s", strings.Join(undefined, ", "))
	}

	return nil
}

// collectConfigRefs walks the AST and collects all ConfigRef paths
func collectConfigRefs(expr RuleExpr) []string {
	var refs []string

	switch e := expr.(type) {
	case Implication:
		refs = append(refs, collectConfigRefs(e.Antecedent)...)
		refs = append(refs, collectConfigRefs(e.Consequent)...)
	case Comparison:
		refs = append(refs, collectConfigRefs(e.Left)...)
		refs = append(refs, collectConfigRefs(e.Right)...)
	case ConfigRef:
		refs = append(refs, e.Path)
	case ExecutionEnv:
		// Not a config ref
	case StringLiteral:
		// Not a config ref
	}

	return refs
}
