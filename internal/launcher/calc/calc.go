// Package calc provides a pure math expression evaluator for the launcher.
//
// Operator semantics:
//   - ^ is power (right-associative)
//   - / is float division
//   - % is context-sensitive (see below)
//   - unary minus is supported
//
// Percent semantics:
//   - a + b%  →  a + a*(b/100)
//   - a - b%  →  a - a*(b/100)
//   - a * b%  →  a * (b/100)
//   - a / b%  →  a / (b/100)
//   - bare b% →  b/100
//
// Trig functions (sin, cos, tan, asin, acos, atan) take degrees by default.
// rad(x) marks x as already-in-radians so trig skips the degree conversion.
// Inverse trig (asin, acos, atan) always returns degrees.
package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// value is an evaluated sub-expression. isPercent and isRad carry semantic
// context through one layer of binary/function application and are then consumed.
type value struct {
	f         float64
	isPercent bool // set by the % postfix operator
	isRad     bool // set by rad(); trig functions skip degree conversion when true
}

// Evaluate parses and evaluates a math expression, returning the result.
func Evaluate(expr string) (float64, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}
	p := &parser{tokens: tokens}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if !p.atEnd() {
		return 0, fmt.Errorf("unexpected token: %s", p.peek().text)
	}
	if v.isPercent {
		return v.f / 100, nil
	}
	return v.f, nil
}

// ── Token ──────────────────────────────────────────────────────────────────

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokIdent
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokCaret
	tokLParen
	tokRParen
	tokComma
	tokEOF
)

type token struct {
	kind tokenKind
	text string
	num  float64
}

func tokenize(s string) ([]token, error) {
	var tokens []token
	i := 0
	runes := []rune(s)
	n := len(runes)
	for i < n {
		ch := runes[i]
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		switch {
		case ch >= '0' && ch <= '9' || ch == '.':
			j := i
			for j < n && (runes[j] >= '0' && runes[j] <= '9' || runes[j] == '.') {
				j++
			}
			// optional 'e'/'E' exponent
			if j < n && (runes[j] == 'e' || runes[j] == 'E') {
				j++
				if j < n && (runes[j] == '+' || runes[j] == '-') {
					j++
				}
				for j < n && runes[j] >= '0' && runes[j] <= '9' {
					j++
				}
			}
			raw := string(runes[i:j])
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", raw)
			}
			tokens = append(tokens, token{kind: tokNumber, text: raw, num: f})
			i = j
		case unicode.IsLetter(ch) || ch == '_':
			j := i
			for j < n && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			name := string(runes[i:j])
			tokens = append(tokens, token{kind: tokIdent, text: name})
			i = j
		case ch == '+':
			tokens = append(tokens, token{kind: tokPlus, text: "+"})
			i++
		case ch == '-':
			tokens = append(tokens, token{kind: tokMinus, text: "-"})
			i++
		case ch == '*':
			tokens = append(tokens, token{kind: tokStar, text: "*"})
			i++
		case ch == '/':
			tokens = append(tokens, token{kind: tokSlash, text: "/"})
			i++
		case ch == '%':
			tokens = append(tokens, token{kind: tokPercent, text: "%"})
			i++
		case ch == '^':
			tokens = append(tokens, token{kind: tokCaret, text: "^"})
			i++
		case ch == '(':
			tokens = append(tokens, token{kind: tokLParen, text: "("})
			i++
		case ch == ')':
			tokens = append(tokens, token{kind: tokRParen, text: ")"})
			i++
		case ch == ',':
			tokens = append(tokens, token{kind: tokComma, text: ","})
			i++
		default:
			return nil, fmt.Errorf("unexpected character: %c", ch)
		}
	}
	tokens = append(tokens, token{kind: tokEOF, text: "EOF"})
	return tokens, nil
}

// ── Parser ─────────────────────────────────────────────────────────────────

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) consume() token {
	t := p.peek()
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) atEnd() bool {
	return p.peek().kind == tokEOF
}

// parseExpr = parseSum
func (p *parser) parseExpr() (value, error) {
	return p.parseSum()
}

// parseSum handles + and - with percent-aware semantics
func (p *parser) parseSum() (value, error) {
	left, err := p.parseProduct()
	if err != nil {
		return value{}, err
	}
	for {
		tok := p.peek()
		if tok.kind != tokPlus && tok.kind != tokMinus {
			break
		}
		p.consume()
		right, err := p.parseProduct()
		if err != nil {
			return value{}, err
		}
		if right.isPercent {
			// a ± b% → a ± a*(b/100)
			pct := left.f * (right.f / 100)
			if tok.kind == tokPlus {
				left = value{f: left.f + pct}
			} else {
				left = value{f: left.f - pct}
			}
		} else {
			if tok.kind == tokPlus {
				left = value{f: left.f + right.f}
			} else {
				left = value{f: left.f - right.f}
			}
		}
	}
	return left, nil
}

// parseProduct handles * and /
func (p *parser) parseProduct() (value, error) {
	left, err := p.parseUnary()
	if err != nil {
		return value{}, err
	}
	for {
		tok := p.peek()
		if tok.kind != tokStar && tok.kind != tokSlash {
			break
		}
		p.consume()
		right, err := p.parseUnary()
		if err != nil {
			return value{}, err
		}
		r := right.f
		if right.isPercent {
			r = right.f / 100
		}
		if tok.kind == tokStar {
			left = value{f: left.f * r}
		} else {
			left = value{f: left.f / r}
		}
	}
	return left, nil
}

// parseUnary handles unary minus
func (p *parser) parseUnary() (value, error) {
	if p.peek().kind == tokMinus {
		p.consume()
		v, err := p.parseUnary()
		if err != nil {
			return value{}, err
		}
		return value{f: -v.f}, nil
	}
	return p.parsePower()
}

// parsePower handles ^ (right-associative)
func (p *parser) parsePower() (value, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return value{}, err
	}
	if p.peek().kind == tokCaret {
		p.consume()
		exp, err := p.parseUnary() // right-associative: recurse into parseUnary
		if err != nil {
			return value{}, err
		}
		return value{f: math.Pow(base.f, exp.f)}, nil
	}
	return base, nil
}

// parsePostfix handles the % postfix operator
func (p *parser) parsePostfix() (value, error) {
	v, err := p.parsePrimary()
	if err != nil {
		return value{}, err
	}
	if p.peek().kind == tokPercent {
		p.consume()
		return value{f: v.f, isPercent: true}, nil
	}
	return v, nil
}

var constants = map[string]float64{
	"pi":  math.Pi,
	"e":   math.E,
	"tau": 2 * math.Pi,
	"phi": (1 + math.Sqrt(5)) / 2,
}

// parsePrimary handles numbers, constants, function calls, and parentheses
func (p *parser) parsePrimary() (value, error) {
	tok := p.peek()
	switch tok.kind {
	case tokNumber:
		p.consume()
		return value{f: tok.num}, nil

	case tokIdent:
		p.consume()
		name := strings.ToLower(tok.text)
		if c, ok := constants[name]; ok {
			return value{f: c}, nil
		}
		// function call
		if p.peek().kind != tokLParen {
			return value{}, fmt.Errorf("unknown identifier: %s", tok.text)
		}
		p.consume() // consume '('
		args, err := p.parseArgList()
		if err != nil {
			return value{}, err
		}
		if p.peek().kind != tokRParen {
			return value{}, fmt.Errorf("expected ')' after function arguments")
		}
		p.consume() // consume ')'
		return evalFunc(name, args)

	case tokLParen:
		p.consume()
		v, err := p.parseExpr()
		if err != nil {
			return value{}, err
		}
		if p.peek().kind != tokRParen {
			return value{}, fmt.Errorf("expected ')'")
		}
		p.consume()
		return v, nil

	default:
		return value{}, fmt.Errorf("unexpected token: %s", tok.text)
	}
}

// parseArgList parses a comma-separated list of expressions
func (p *parser) parseArgList() ([]value, error) {
	if p.peek().kind == tokRParen {
		return nil, nil
	}
	var args []value
	for {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		if p.peek().kind != tokComma {
			break
		}
		p.consume()
	}
	return args, nil
}

// evalFunc evaluates a named function with the given arguments
func evalFunc(name string, args []value) (value, error) {
	switch name {
	case "rad":
		if len(args) != 1 {
			return value{}, fmt.Errorf("rad() takes 1 argument")
		}
		return value{f: args[0].f, isRad: true}, nil

	case "sqrt":
		if len(args) != 1 {
			return value{}, fmt.Errorf("sqrt() takes 1 argument")
		}
		return value{f: math.Sqrt(args[0].f)}, nil

	case "cbrt":
		if len(args) != 1 {
			return value{}, fmt.Errorf("cbrt() takes 1 argument")
		}
		return value{f: math.Cbrt(args[0].f)}, nil

	case "abs":
		if len(args) != 1 {
			return value{}, fmt.Errorf("abs() takes 1 argument")
		}
		return value{f: math.Abs(args[0].f)}, nil

	case "round":
		if len(args) != 1 {
			return value{}, fmt.Errorf("round() takes 1 argument")
		}
		return value{f: math.Round(args[0].f)}, nil

	case "floor":
		if len(args) != 1 {
			return value{}, fmt.Errorf("floor() takes 1 argument")
		}
		return value{f: math.Floor(args[0].f)}, nil

	case "ceil":
		if len(args) != 1 {
			return value{}, fmt.Errorf("ceil() takes 1 argument")
		}
		return value{f: math.Ceil(args[0].f)}, nil

	case "ln":
		if len(args) != 1 {
			return value{}, fmt.Errorf("ln() takes 1 argument")
		}
		return value{f: math.Log(args[0].f)}, nil

	case "log":
		if len(args) != 1 {
			return value{}, fmt.Errorf("log() takes 1 argument")
		}
		return value{f: math.Log10(args[0].f)}, nil

	case "log2":
		if len(args) != 1 {
			return value{}, fmt.Errorf("log2() takes 1 argument")
		}
		return value{f: math.Log2(args[0].f)}, nil

	case "exp":
		if len(args) != 1 {
			return value{}, fmt.Errorf("exp() takes 1 argument")
		}
		return value{f: math.Exp(args[0].f)}, nil

	case "pow":
		if len(args) != 2 {
			return value{}, fmt.Errorf("pow() takes 2 arguments")
		}
		return value{f: math.Pow(args[0].f, args[1].f)}, nil

	case "min":
		if len(args) < 1 {
			return value{}, fmt.Errorf("min() takes at least 1 argument")
		}
		m := args[0].f
		for _, a := range args[1:] {
			m = math.Min(m, a.f)
		}
		return value{f: m}, nil

	case "max":
		if len(args) < 1 {
			return value{}, fmt.Errorf("max() takes at least 1 argument")
		}
		m := args[0].f
		for _, a := range args[1:] {
			m = math.Max(m, a.f)
		}
		return value{f: m}, nil

	// Trig — degrees by default; rad() arg bypasses conversion
	case "sin":
		if len(args) != 1 {
			return value{}, fmt.Errorf("sin() takes 1 argument")
		}
		return value{f: math.Sin(toRad(args[0]))}, nil

	case "cos":
		if len(args) != 1 {
			return value{}, fmt.Errorf("cos() takes 1 argument")
		}
		return value{f: math.Cos(toRad(args[0]))}, nil

	case "tan":
		if len(args) != 1 {
			return value{}, fmt.Errorf("tan() takes 1 argument")
		}
		return value{f: math.Tan(toRad(args[0]))}, nil

	// Inverse trig — always returns degrees
	case "asin":
		if len(args) != 1 {
			return value{}, fmt.Errorf("asin() takes 1 argument")
		}
		return value{f: math.Asin(args[0].f) * 180 / math.Pi}, nil

	case "acos":
		if len(args) != 1 {
			return value{}, fmt.Errorf("acos() takes 1 argument")
		}
		return value{f: math.Acos(args[0].f) * 180 / math.Pi}, nil

	case "atan":
		if len(args) != 1 {
			return value{}, fmt.Errorf("atan() takes 1 argument")
		}
		return value{f: math.Atan(args[0].f) * 180 / math.Pi}, nil

	default:
		return value{}, fmt.Errorf("unknown function: %s()", name)
	}
}

// toRad converts a value to radians for trig. If isRad is set, the value is
// already in radians; otherwise it is treated as degrees.
func toRad(v value) float64 {
	if v.isRad {
		return v.f
	}
	return v.f * math.Pi / 180
}
