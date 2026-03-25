package tmuxtargets

import (
	"strings"
	"testing"
)

func TestViewColorsNonSelectedTargetsBlue(t *testing.T) {
	lines := []string{"a https://one.test b https://two.test"}
	targets := []target{
		{line: 0, start: 2, end: 18, text: "https://one.test", openable: true, openTarget: "https://one.test"},
		{line: 0, start: 21, end: 37, text: "https://two.test", openable: true, openTarget: "https://two.test"},
	}

	m := newModel(lines, targets, func(string) {})
	v := m.View()

	if !strings.Contains(v.Content, selectedColorPrefix+"https://one.test"+resetColor) {
		t.Fatalf("expected selected target styling in view content: %q", v.Content)
	}
	if !strings.Contains(v.Content, targetColorPrefix+"https://two.test"+resetColor) {
		t.Fatalf("expected non-selected target blue styling in view content: %q", v.Content)
	}
}
