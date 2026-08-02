package power

import (
	"reflect"
	"testing"
	"time"
)

func TestSamplesFileName(t *testing.T) {
	date := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	if got := SamplesFileName(date); got != "samples-2026-07-31.jsonl" {
		t.Errorf("SamplesFileName = %q, want samples-2026-07-31.jsonl", got)
	}
}

func TestFilesToPrune(t *testing.T) {
	today := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	names := []string{
		"samples-2026-08-02.jsonl", // today, keep
		"samples-2026-07-19.jsonl", // exactly 14 days ago, keep
		"samples-2026-07-18.jsonl", // 15 days ago, prune
		"samples-2026-06-01.jsonl", // long ago, prune
		"daemon.pid",               // not a samples file, ignore
		"samples-bogus.jsonl",      // malformed date, ignore
	}

	got := FilesToPrune(names, today, RetentionDays)
	want := []string{"samples-2026-07-18.jsonl", "samples-2026-06-01.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilesToPrune = %v, want %v", got, want)
	}
}

func TestFilesToPruneNoneOld(t *testing.T) {
	today := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	names := []string{"samples-2026-08-01.jsonl", "samples-2026-08-02.jsonl"}

	got := FilesToPrune(names, today, RetentionDays)
	if len(got) != 0 {
		t.Errorf("FilesToPrune = %v, want empty", got)
	}
}
