package power

import (
	"strings"
	"time"
)

// RetentionDays is how many calendar days of samples files are kept.
const RetentionDays = 14

const samplesFilePrefix = "samples-"
const samplesFileSuffix = ".jsonl"
const samplesFileDateLayout = "2006-01-02"

// SamplesFileName returns the day-file name for the given date, e.g.
// "samples-2026-07-31.jsonl".
func SamplesFileName(date time.Time) string {
	return samplesFilePrefix + date.Format(samplesFileDateLayout) + samplesFileSuffix
}

// FilesToPrune returns the subset of names (day-files as produced by
// SamplesFileName) whose date is older than RetentionDays before today.
// Names that don't match the samples-file naming scheme are ignored.
func FilesToPrune(names []string, today time.Time, retentionDays int) []string {
	cutoff := today.AddDate(0, 0, -retentionDays)

	var pruned []string
	for _, name := range names {
		date, ok := parseSamplesFileDate(name)
		if !ok {
			continue
		}
		if date.Before(cutoff) {
			pruned = append(pruned, name)
		}
	}
	return pruned
}

func parseSamplesFileDate(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, samplesFilePrefix) || !strings.HasSuffix(name, samplesFileSuffix) {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, samplesFilePrefix), samplesFileSuffix)
	date, err := time.Parse(samplesFileDateLayout, datePart)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}
