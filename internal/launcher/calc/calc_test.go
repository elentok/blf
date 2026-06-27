package calc

import (
	"math"
	"testing"
)

const eps = 1e-9

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func eval(t *testing.T, expr string) float64 {
	t.Helper()
	v, err := Evaluate(expr)
	if err != nil {
		t.Fatalf("Evaluate(%q): %v", expr, err)
	}
	return v
}

func evalErr(t *testing.T, expr string) error {
	t.Helper()
	_, err := Evaluate(expr)
	return err
}

// ── Basic arithmetic ───────────────────────────────────────────────────────

func TestAddSubtract(t *testing.T) {
	approx(t, "1+2", eval(t, "1+2"), 3)
	approx(t, "10-3", eval(t, "10-3"), 7)
	approx(t, "1+2+3", eval(t, "1+2+3"), 6)
}

func TestMultiplyDivide(t *testing.T) {
	approx(t, "3*4", eval(t, "3*4"), 12)
	approx(t, "10/4", eval(t, "10/4"), 2.5) // float division
	approx(t, "7/2", eval(t, "7/2"), 3.5)
}

func TestPrecedence(t *testing.T) {
	approx(t, "2+3*4", eval(t, "2+3*4"), 14)
	approx(t, "10-2*3", eval(t, "10-2*3"), 4)
	approx(t, "2*3+4*5", eval(t, "2*3+4*5"), 26)
}

func TestParentheses(t *testing.T) {
	approx(t, "(2+3)*4", eval(t, "(2+3)*4"), 20)
	approx(t, "2*(3+4)", eval(t, "2*(3+4)"), 14)
}

func TestUnaryMinus(t *testing.T) {
	approx(t, "-5", eval(t, "-5"), -5)
	approx(t, "-5+3", eval(t, "-5+3"), -2)
	approx(t, "2*-3", eval(t, "2*-3"), -6)
	approx(t, "-(-5)", eval(t, "-(-5)"), 5)
}

func TestPower(t *testing.T) {
	approx(t, "2^8", eval(t, "2^8"), 256)
	approx(t, "2^3^2", eval(t, "2^3^2"), 512) // right-associative: 2^(3^2)=2^9=512
	approx(t, "4^0.5", eval(t, "4^0.5"), 2)
}

// ── Percent semantics ──────────────────────────────────────────────────────

func TestPercentBare(t *testing.T) {
	approx(t, "10%", eval(t, "10%"), 0.1)
	approx(t, "50%", eval(t, "50%"), 0.5)
}

func TestPercentAdd(t *testing.T) {
	approx(t, "200+10%", eval(t, "200+10%"), 220)
	approx(t, "100+50%", eval(t, "100+50%"), 150)
}

func TestPercentSubtract(t *testing.T) {
	approx(t, "200-10%", eval(t, "200-10%"), 180)
	approx(t, "100-25%", eval(t, "100-25%"), 75)
}

func TestPercentMultiply(t *testing.T) {
	approx(t, "200*10%", eval(t, "200*10%"), 20)
	approx(t, "50*50%", eval(t, "50*50%"), 25)
}

func TestPercentDivide(t *testing.T) {
	approx(t, "200/10%", eval(t, "200/10%"), 2000)
}

// ── Constants ─────────────────────────────────────────────────────────────

func TestConstants(t *testing.T) {
	approx(t, "pi", eval(t, "pi"), math.Pi)
	approx(t, "e", eval(t, "e"), math.E)
	approx(t, "tau", eval(t, "tau"), 2*math.Pi)
	approx(t, "phi", eval(t, "phi"), (1+math.Sqrt(5))/2)
	approx(t, "2*pi", eval(t, "2*pi"), 2*math.Pi)
}

// ── Functions ─────────────────────────────────────────────────────────────

func TestSqrt(t *testing.T) {
	approx(t, "sqrt(4)", eval(t, "sqrt(4)"), 2)
	approx(t, "sqrt(2)", eval(t, "sqrt(2)"), math.Sqrt(2))
}

func TestCbrt(t *testing.T) {
	approx(t, "cbrt(8)", eval(t, "cbrt(8)"), 2)
	approx(t, "cbrt(27)", eval(t, "cbrt(27)"), 3)
}

func TestAbs(t *testing.T) {
	approx(t, "abs(-5)", eval(t, "abs(-5)"), 5)
	approx(t, "abs(3)", eval(t, "abs(3)"), 3)
}

func TestRoundFloorCeil(t *testing.T) {
	approx(t, "round(1.5)", eval(t, "round(1.5)"), 2)
	approx(t, "round(1.4)", eval(t, "round(1.4)"), 1)
	approx(t, "floor(1.9)", eval(t, "floor(1.9)"), 1)
	approx(t, "ceil(1.1)", eval(t, "ceil(1.1)"), 2)
}

func TestLog(t *testing.T) {
	approx(t, "ln(e)", eval(t, "ln(e)"), 1)
	approx(t, "log(100)", eval(t, "log(100)"), 2)
	approx(t, "log2(8)", eval(t, "log2(8)"), 3)
	approx(t, "exp(1)", eval(t, "exp(1)"), math.E)
}

func TestPow(t *testing.T) {
	approx(t, "pow(2,10)", eval(t, "pow(2,10)"), 1024)
}

func TestMinMax(t *testing.T) {
	approx(t, "min(3,1,2)", eval(t, "min(3,1,2)"), 1)
	approx(t, "max(3,1,2)", eval(t, "max(3,1,2)"), 3)
}

// ── Trig — degrees by default ─────────────────────────────────────────────

func TestSinCosTanDegrees(t *testing.T) {
	approx(t, "sin(30)", eval(t, "sin(30)"), 0.5)
	approx(t, "cos(60)", eval(t, "cos(60)"), 0.5)
	approx(t, "sin(0)", eval(t, "sin(0)"), 0)
	approx(t, "cos(0)", eval(t, "cos(0)"), 1)
	approx(t, "tan(45)", eval(t, "tan(45)"), 1)
}

// rad() marks the argument as already-in-radians, bypassing degree conversion
func TestRadEscape(t *testing.T) {
	approx(t, "sin(rad(pi/6))", eval(t, "sin(rad(pi/6))"), 0.5)
	approx(t, "cos(rad(pi/3))", eval(t, "cos(rad(pi/3))"), 0.5)
	approx(t, "tan(rad(pi/4))", eval(t, "tan(rad(pi/4))"), 1)
}

// Inverse trig always returns degrees
func TestInverseTrigReturnsDegrees(t *testing.T) {
	approx(t, "asin(0.5)", eval(t, "asin(0.5)"), 30)
	approx(t, "acos(0.5)", eval(t, "acos(0.5)"), 60)
	approx(t, "atan(1)", eval(t, "atan(1)"), 45)
	approx(t, "asin(1)", eval(t, "asin(1)"), 90)
	approx(t, "acos(1)", eval(t, "acos(1)"), 0)
}

// ── Case insensitivity ─────────────────────────────────────────────────────

func TestCaseInsensitiveIdentifiers(t *testing.T) {
	approx(t, "SQRT(4)", eval(t, "SQRT(4)"), 2)
	approx(t, "Sin(30)", eval(t, "Sin(30)"), 0.5)
	approx(t, "PI", eval(t, "PI"), math.Pi)
}

// ── Whitespace ─────────────────────────────────────────────────────────────

func TestWhitespace(t *testing.T) {
	approx(t, "1 + 2", eval(t, "1 + 2"), 3)
	approx(t, "sqrt( 9 )", eval(t, "sqrt( 9 )"), 3)
}

// ── Error cases ────────────────────────────────────────────────────────────

func TestErrors(t *testing.T) {
	if evalErr(t, "1++2") == nil {
		t.Error("expected error for 1++2")
	}
	if evalErr(t, "sqrt(") == nil {
		t.Error("expected error for unclosed paren")
	}
	if evalErr(t, "unknown(1)") == nil {
		t.Error("expected error for unknown function")
	}
	if evalErr(t, "") == nil {
		t.Error("expected error for empty expression")
	}
}
