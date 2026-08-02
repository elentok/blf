package power

import (
	"testing"
	"time"
)

func TestParseSinceWindowValid(t *testing.T) {
	cases := map[string]time.Duration{
		"24h": 24 * time.Hour,
		"1h":  time.Hour,
		"7d":  7 * 24 * time.Hour,
		"3d":  3 * 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseSinceWindow(in)
		if err != nil {
			t.Fatalf("ParseSinceWindow(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseSinceWindow(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseSinceWindowInvalid(t *testing.T) {
	cases := []string{"", "24", "h", "1d12h", "24m", "-1h", "1.5h", "1w"}
	for _, in := range cases {
		if _, err := ParseSinceWindow(in); err == nil {
			t.Errorf("ParseSinceWindow(%q) returned no error, want one", in)
		}
	}
}

func TestDayFilesForWindowSingleDay(t *testing.T) {
	end := time.Date(2026, 7, 31, 19, 30, 0, 0, time.UTC)
	got := DayFilesForWindow(end, time.Hour)
	want := []string{"samples-2026-07-31.jsonl"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("DayFilesForWindow = %v, want %v", got, want)
	}
}

func TestDayFilesForWindowSpansMultipleDays(t *testing.T) {
	end := time.Date(2026, 7, 31, 19, 30, 0, 0, time.UTC)
	got := DayFilesForWindow(end, 3*24*time.Hour)
	want := []string{
		"samples-2026-07-28.jsonl",
		"samples-2026-07-29.jsonl",
		"samples-2026-07-30.jsonl",
		"samples-2026-07-31.jsonl",
	}
	if len(got) != len(want) {
		t.Fatalf("DayFilesForWindow = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DayFilesForWindow[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func makeReportSample(ts time.Time, batteryPct int, charging bool, thermal string, procs ...ProcessSample) Sample {
	return Sample{
		SchemaVersion:   CurrentSchemaVersion,
		Ts:              ts,
		ThermalPressure: thermal,
		BatteryPct:      batteryPct,
		BatteryCharging: charging,
		Processes:       procs,
	}
}

func TestBuildReportRanksProcessesByMeanEnergyImpact(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	samples := []Sample{
		makeReportSample(base, 50, false, "Nominal",
			ProcessSample{Name: "high", EnergyImpactPerS: 100},
			ProcessSample{Name: "spike", EnergyImpactPerS: 900},
		),
		makeReportSample(base.Add(30*time.Second), 50, false, "Nominal",
			ProcessSample{Name: "high", EnergyImpactPerS: 200},
		),
	}

	r := BuildReport(samples, time.Hour, base.Add(time.Minute))

	if r.TicksInWindow != 2 {
		t.Fatalf("TicksInWindow = %d, want 2", r.TicksInWindow)
	}
	if len(r.TopProcesses) != 2 {
		t.Fatalf("len(TopProcesses) = %d, want 2", len(r.TopProcesses))
	}

	// "spike" appeared once at 900, mean energy impact 900 -- ranks first
	// despite lower coverage than "high" (mean (100+200)/2 = 150).
	if r.TopProcesses[0].Name != "spike" || r.TopProcesses[0].MeanEnergyImpact != 900 || r.TopProcesses[0].TicksPresent != 1 {
		t.Errorf("TopProcesses[0] = %+v, want spike/900/1 ticks", r.TopProcesses[0])
	}
	if r.TopProcesses[1].Name != "high" || r.TopProcesses[1].MeanEnergyImpact != 150 || r.TopProcesses[1].TicksPresent != 2 {
		t.Errorf("TopProcesses[1] = %+v, want high/150/2 ticks", r.TopProcesses[1])
	}
}

func TestBuildReportLimitsToTop10Processes(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	var procs []ProcessSample
	for i := range 15 {
		procs = append(procs, ProcessSample{Name: string(rune('a' + i)), EnergyImpactPerS: float64(i)})
	}
	samples := []Sample{makeReportSample(base, 50, false, "Nominal", procs...)}

	r := BuildReport(samples, time.Hour, base)

	if len(r.TopProcesses) != topReportProcessCount {
		t.Fatalf("len(TopProcesses) = %d, want %d", len(r.TopProcesses), topReportProcessCount)
	}
	if r.TopProcesses[0].Name != "o" { // 'o' = 'a'+14, EnergyImpactPerS 14, the highest
		t.Errorf("TopProcesses[0].Name = %q, want %q", r.TopProcesses[0].Name, "o")
	}
}

func TestBuildReportBatteryNetChange(t *testing.T) {
	base := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	samples := []Sample{
		makeReportSample(base, 67, false, "Nominal"),
		makeReportSample(base.Add(time.Hour), 33, false, "Nominal"),
	}

	r := BuildReport(samples, 24*time.Hour, base.Add(time.Hour))

	if r.Battery.StartPct != 67 || r.Battery.EndPct != 33 || r.Battery.NetChangePct != -34 {
		t.Errorf("Battery = %+v, want start=67 end=33 net=-34", r.Battery)
	}
}

func TestBuildReportDischargeOnlyRateStitchesStretches(t *testing.T) {
	base := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	samples := []Sample{
		// Discharge stretch 1: 100 -> 90 over 1h (10%/h).
		makeReportSample(base, 100, false, "Nominal"),
		makeReportSample(base.Add(time.Hour), 90, false, "Nominal"),
		// Charging gap: pct rises, should not dilute the discharge rate.
		makeReportSample(base.Add(2*time.Hour), 95, true, "Nominal"),
		// Discharge stretch 2: 95 -> 85 over 1h (10%/h).
		makeReportSample(base.Add(3*time.Hour), 95, false, "Nominal"),
		makeReportSample(base.Add(4*time.Hour), 85, false, "Nominal"),
	}

	r := BuildReport(samples, 4*time.Hour, base.Add(4*time.Hour))

	if !r.Battery.HasDischargeRate {
		t.Fatal("HasDischargeRate = false, want true")
	}
	// Total drop across stretches: (100-90) + (95-85) = 20, over 2h total -> 10%/h.
	if got := r.Battery.DischargeRatePctPerHour; got != 10 {
		t.Errorf("DischargeRatePctPerHour = %v, want 10", got)
	}
}

func TestBuildReportNoDischargeRateWhenAlwaysCharging(t *testing.T) {
	base := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	samples := []Sample{
		makeReportSample(base, 50, true, "Nominal"),
		makeReportSample(base.Add(time.Hour), 60, true, "Nominal"),
	}

	r := BuildReport(samples, time.Hour, base.Add(time.Hour))

	if r.Battery.HasDischargeRate {
		t.Errorf("HasDischargeRate = true, want false (no discharging stretch)")
	}
}

func TestBuildReportThermalBreakdown(t *testing.T) {
	base := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	var samples []Sample
	for i := range 97 {
		samples = append(samples, makeReportSample(base.Add(time.Duration(i)*time.Minute), 50, false, "Nominal"))
	}
	for i := range 3 {
		samples = append(samples, makeReportSample(base.Add(time.Duration(97+i)*time.Minute), 50, false, "Moderate"))
	}

	r := BuildReport(samples, 24*time.Hour, base.Add(100*time.Minute))

	if len(r.Thermal) != 2 {
		t.Fatalf("len(Thermal) = %d, want 2", len(r.Thermal))
	}
	if r.Thermal[0].Value != "Nominal" || r.Thermal[0].Percent != 97 {
		t.Errorf("Thermal[0] = %+v, want Nominal/97", r.Thermal[0])
	}
	if r.Thermal[1].Value != "Moderate" || r.Thermal[1].Percent != 3 {
		t.Errorf("Thermal[1] = %+v, want Moderate/3", r.Thermal[1])
	}
}
