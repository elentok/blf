package power

import (
	"testing"
	"time"
)

func TestFormatParseMarkFileRoundTrips(t *testing.T) {
	want := time.Date(2026, 7, 31, 19, 25, 12, 0, time.FixedZone("+03:00", 3*60*60))

	data := FormatMarkFile(want)
	got, err := ParseMarkFile(data)
	if err != nil {
		t.Fatalf("ParseMarkFile returned error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("round-trip mismatch: got %v, want %v", got, want)
	}
}

func TestParseMarkFileMalformed(t *testing.T) {
	if _, err := ParseMarkFile([]byte("not a mark file")); err == nil {
		t.Fatal("expected error for malformed mark file, got nil")
	}
}
