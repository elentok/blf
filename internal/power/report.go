package power

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// topReportProcessCount is how many processes `report` shows in its top
// energy-consumers table.
const topReportProcessCount = 10

var sinceWindowPattern = regexp.MustCompile(`^(\d+)([hd])$`)

// ParseSinceWindow parses a `report --since` window string, e.g. "24h" or
// "7d", into a Duration. Deliberately minimal (not bare time.ParseDuration,
// which lacks a day unit): a single leading integer plus a lone "h" (hours)
// or "d" (days) unit -- no minutes, no combined units like "1d12h".
func ParseSinceWindow(s string) (time.Duration, error) {
	m := sinceWindowPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid --since value %q (expected e.g. 24h or 7d)", s)
	}

	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("invalid --since value %q: %w", s, err)
	}

	if m[2] == "d" {
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.Duration(n) * time.Hour, nil
}

// DayFilesForWindow returns the samples-file names (as produced by
// SamplesFileName) for every calendar day overlapping [end-since, end].
func DayFilesForWindow(end time.Time, since time.Duration) []string {
	start := dateOnly(end.Add(-since))
	last := dateOnly(end)

	var names []string
	for d := start; !d.After(last); d = d.AddDate(0, 0, 1) {
		names = append(names, SamplesFileName(d))
	}
	return names
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// ProcessReportEntry is one row of a Report's top energy-consumers table.
type ProcessReportEntry struct {
	Name             string
	MeanEnergyImpact float64
	TicksPresent     int
}

// BatteryReport is a Report's battery section.
type BatteryReport struct {
	StartPct     int
	EndPct       int
	NetChangePct int

	// HasDischargeRate is false when the window contains no discharging
	// stretch of at least two samples (nothing to average a rate over).
	HasDischargeRate        bool
	DischargeRatePctPerHour float64
}

// ThermalReportEntry is one observed thermal_pressure value's share of the
// window's samples.
type ThermalReportEntry struct {
	Value   string
	Percent float64
}

// Report is the aggregated result of summarizing a window of samples for
// `blf power report`.
type Report struct {
	WindowStart time.Time
	WindowEnd   time.Time

	TicksInWindow int
	TopProcesses  []ProcessReportEntry
	Battery       BatteryReport
	Thermal       []ThermalReportEntry

	MeanCPUPowerMW float64
	MeanGPUPowerMW float64
}

// BuildReport aggregates samples -- already filtered to the report window
// and sorted by Ts ascending -- into a Report. end is the wall-clock time
// the report was generated at, used (with since) for the window header.
func BuildReport(samples []Sample, since time.Duration, end time.Time) Report {
	meanCPU, meanGPU := meanCPUGPUPower(samples)
	return Report{
		WindowStart: end.Add(-since),
		WindowEnd:   end,

		TicksInWindow: len(samples),
		TopProcesses:  rankProcesses(samples),
		Battery:       buildBatteryReport(samples),
		Thermal:       buildThermalReport(samples),

		MeanCPUPowerMW: meanCPU,
		MeanGPUPowerMW: meanGPU,
	}
}

func meanCPUGPUPower(samples []Sample) (meanCPU, meanGPU float64) {
	if len(samples) == 0 {
		return 0, 0
	}

	var sumCPU, sumGPU float64
	for _, s := range samples {
		sumCPU += s.CPUPowerMW
		sumGPU += s.GPUPowerMW
	}
	n := float64(len(samples))
	return sumCPU / n, sumGPU / n
}

func rankProcesses(samples []Sample) []ProcessReportEntry {
	type agg struct {
		sum   float64
		ticks int
	}

	byName := make(map[string]*agg)
	var order []string
	for _, s := range samples {
		for _, p := range s.Processes {
			a, ok := byName[p.Name]
			if !ok {
				a = &agg{}
				byName[p.Name] = a
				order = append(order, p.Name)
			}
			a.sum += p.EnergyImpactPerS
			a.ticks++
		}
	}

	entries := make([]ProcessReportEntry, 0, len(order))
	for _, name := range order {
		a := byName[name]
		entries = append(entries, ProcessReportEntry{
			Name:             name,
			MeanEnergyImpact: a.sum / float64(a.ticks),
			TicksPresent:     a.ticks,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].MeanEnergyImpact > entries[j].MeanEnergyImpact
	})
	if len(entries) > topReportProcessCount {
		entries = entries[:topReportProcessCount]
	}
	return entries
}

func buildBatteryReport(samples []Sample) BatteryReport {
	if len(samples) == 0 {
		return BatteryReport{}
	}

	first := samples[0]
	last := samples[len(samples)-1]
	br := BatteryReport{
		StartPct:     first.BatteryPct,
		EndPct:       last.BatteryPct,
		NetChangePct: last.BatteryPct - first.BatteryPct,
	}

	var totalDropPct int
	var totalDuration time.Duration
	stretchStart := -1
	for i, s := range samples {
		if !s.BatteryCharging {
			if stretchStart == -1 {
				stretchStart = i
			}
			continue
		}
		if stretchStart != -1 {
			totalDropPct += samples[stretchStart].BatteryPct - samples[i-1].BatteryPct
			totalDuration += samples[i-1].Ts.Sub(samples[stretchStart].Ts)
			stretchStart = -1
		}
	}
	if stretchStart != -1 {
		endIdx := len(samples) - 1
		totalDropPct += samples[stretchStart].BatteryPct - samples[endIdx].BatteryPct
		totalDuration += samples[endIdx].Ts.Sub(samples[stretchStart].Ts)
	}

	if totalDuration > 0 {
		br.HasDischargeRate = true
		br.DischargeRatePctPerHour = float64(totalDropPct) / totalDuration.Hours()
	}

	return br
}

func buildThermalReport(samples []Sample) []ThermalReportEntry {
	if len(samples) == 0 {
		return nil
	}

	counts := make(map[string]int)
	var order []string
	for _, s := range samples {
		if _, ok := counts[s.ThermalPressure]; !ok {
			order = append(order, s.ThermalPressure)
		}
		counts[s.ThermalPressure]++
	}

	entries := make([]ThermalReportEntry, 0, len(order))
	for _, v := range order {
		entries = append(entries, ThermalReportEntry{
			Value:   v,
			Percent: float64(counts[v]) / float64(len(samples)) * 100,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Percent > entries[j].Percent
	})
	return entries
}
