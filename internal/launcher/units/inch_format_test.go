package units

import "testing"

func TestFormatInchFraction_ExactMark(t *testing.T) {
	if got := formatInchFraction(0.375); got != "3/8 inch" {
		t.Errorf("got %q, want %q", got, "3/8 inch")
	}
}

func TestFormatInchFraction_ApproxTolerance(t *testing.T) {
	// 0.062992... is within 1/64" of 1/16 (0.0625)
	if got := formatInchFraction(0.062992); got != "~1/16 inch" {
		t.Errorf("got %q, want %q", got, "~1/16 inch")
	}
}

func TestFormatInchFraction_Range(t *testing.T) {
	if got := formatInchFraction(0.4); got != "between 3/8 and 7/16 inch" {
		t.Errorf("got %q, want %q", got, "between 3/8 and 7/16 inch")
	}
}

func TestFormatInchFraction_MixedNumber(t *testing.T) {
	if got := formatInchFraction(1.25); got != "1 1/4 inch" {
		t.Errorf("got %q, want %q", got, "1 1/4 inch")
	}
}

func TestFormatInchFraction_Negative(t *testing.T) {
	if got := formatInchFraction(-0.4); got != "between -7/16 and -3/8 inch" {
		t.Errorf("got %q, want %q", got, "between -7/16 and -3/8 inch")
	}
}

func TestFormatInchFraction_Zero(t *testing.T) {
	if got := formatInchFraction(0); got != "0 inch" {
		t.Errorf("got %q, want %q", got, "0 inch")
	}
}
