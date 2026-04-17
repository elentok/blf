package kitty

import "testing"

func TestParseOSWindowIDStripsANSI(t *testing.T) {
	got, err := parseOSWindowID("\x1b[1;34m12: shell, logs\x1b[0m")
	if err != nil {
		t.Fatalf("parseOSWindowID returned error: %v", err)
	}
	if got != "12" {
		t.Fatalf("id = %q", got)
	}
}

func TestParseOSWindowIDRejectsInvalidSelection(t *testing.T) {
	_, err := parseOSWindowID("not a selection")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSessionSelection(t *testing.T) {
	got, err := parseSessionSelection("/tmp/proj.kitty-session\tproj\t2 tabs")
	if err != nil {
		t.Fatalf("parseSessionSelection returned error: %v", err)
	}
	if got != "/tmp/proj.kitty-session" {
		t.Fatalf("path = %q", got)
	}
}

func TestParseSessionSelectionRejectsInvalidLine(t *testing.T) {
	_, err := parseSessionSelection("proj only")
	if err == nil {
		t.Fatal("expected error")
	}
}
