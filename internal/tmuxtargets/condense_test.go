package tmuxtargets

import "testing"

func TestCondenseViewportKeepsContextAndFoldsGaps(t *testing.T) {
	lines := []string{
		"l0",
		"l1",
		"l2 target",
		"l3",
		"l4",
		"l5",
		"l6 target",
		"l7",
		"l8",
	}
	targets := []target{
		{line: 2, start: 3, end: 9, text: "target-a"},
		{line: 6, start: 3, end: 9, text: "target-b"},
	}

	gotLines, gotTargets := condenseViewport(lines, targets, 1)

	wantLines := []string{"...", "l1", "l2 target", "l3", "...", "l5", "l6 target", "l7", "..."}
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count = %d, want %d (%#v)", len(gotLines), len(wantLines), gotLines)
	}
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("line %d = %q, want %q", i, gotLines[i], wantLines[i])
		}
	}

	if len(gotTargets) != 2 {
		t.Fatalf("target count = %d, want 2", len(gotTargets))
	}
	if gotTargets[0].line != 2 {
		t.Fatalf("first target line = %d, want 2", gotTargets[0].line)
	}
	if gotTargets[1].line != 6 {
		t.Fatalf("second target line = %d, want 6", gotTargets[1].line)
	}
}

func TestCondenseViewportNoTargetsNoChange(t *testing.T) {
	lines := []string{"a", "b"}
	gotLines, gotTargets := condenseViewport(lines, nil, 1)
	if len(gotLines) != 2 || gotLines[0] != "a" || gotLines[1] != "b" {
		t.Fatalf("unexpected lines: %#v", gotLines)
	}
	if gotTargets != nil {
		t.Fatalf("expected nil targets, got %#v", gotTargets)
	}
}
