package power

import (
	"os"
	"testing"
	"time"
)

func TestParsePowermetrics(t *testing.T) {
	data, err := os.ReadFile("testdata/powermetrics.plist")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	result, err := ParsePowermetrics(data)
	if err != nil {
		t.Fatalf("ParsePowermetrics returned error: %v", err)
	}

	if result.CPUPowerMW != 1488.3 {
		t.Errorf("CPUPowerMW = %v, want 1488.3", result.CPUPowerMW)
	}
	if result.GPUPowerMW != 3863.26 {
		t.Errorf("GPUPowerMW = %v, want 3863.26", result.GPUPowerMW)
	}
	if result.CombinedPowerMW != 5351.56 {
		t.Errorf("CombinedPowerMW = %v, want 5351.56", result.CombinedPowerMW)
	}
	if result.ThermalPressure != "Nominal" {
		t.Errorf("ThermalPressure = %q, want Nominal", result.ThermalPressure)
	}
	if len(result.Processes) != 3 {
		t.Fatalf("len(Processes) = %d, want 3", len(result.Processes))
	}
	if result.Processes[0] != (ProcessSample{PID: 392, Name: "WindowServer", EnergyImpactPerS: 1253.33, CPUMsPerS: 408.669}) {
		t.Errorf("Processes[0] = %+v", result.Processes[0])
	}
}

func TestParseIoreg(t *testing.T) {
	data, err := os.ReadFile("testdata/ioreg.plist")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	batt, err := ParseIoreg(data)
	if err != nil {
		t.Fatalf("ParseIoreg returned error: %v", err)
	}

	want := BatteryInfo{
		CurrentCapacity:         28,
		AppleRawCurrentCapacity: 1672,
		AppleRawMaxCapacity:     6191,
		Amperage:                3583,
		Voltage:                 11818,
		IsCharging:              true,
		ExternalConnected:       true,
		CycleCount:              143,
		FullyCharged:            false,
	}
	if batt != want {
		t.Errorf("ParseIoreg = %+v, want %+v", batt, want)
	}
}

func TestBuildSampleKeepsTopProcessesByEnergyImpact(t *testing.T) {
	now := time.Date(2026, 7, 31, 19, 25, 12, 0, time.FixedZone("+03:00", 3*60*60))
	pm := PowermetricsResult{
		CPUPowerMW:      1000,
		GPUPowerMW:      2000,
		CombinedPowerMW: 3000,
		ThermalPressure: "Nominal",
		Processes: []ProcessSample{
			{PID: 1, Name: "low", EnergyImpactPerS: 5},
			{PID: 2, Name: "high", EnergyImpactPerS: 500},
			{PID: 3, Name: "mid", EnergyImpactPerS: 50},
		},
	}
	batt := BatteryInfo{CurrentCapacity: 42, IsCharging: true}

	sample := BuildSample(now, pm, batt)

	if sample.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", sample.SchemaVersion, CurrentSchemaVersion)
	}
	if !sample.Ts.Equal(now) {
		t.Errorf("Ts = %v, want %v", sample.Ts, now)
	}
	if sample.BatteryPct != 42 || !sample.BatteryCharging {
		t.Errorf("battery fields not copied: %+v", sample)
	}

	wantOrder := []string{"high", "mid", "low"}
	if len(sample.Processes) != len(wantOrder) {
		t.Fatalf("len(Processes) = %d, want %d", len(sample.Processes), len(wantOrder))
	}
	for i, name := range wantOrder {
		if sample.Processes[i].Name != name {
			t.Errorf("Processes[%d].Name = %q, want %q", i, sample.Processes[i].Name, name)
		}
	}
}

func TestBuildSampleTruncatesToTopProcessCount(t *testing.T) {
	processes := make([]ProcessSample, 15)
	for i := range processes {
		processes[i] = ProcessSample{PID: i, Name: "p", EnergyImpactPerS: float64(i)}
	}
	pm := PowermetricsResult{Processes: processes}

	sample := BuildSample(time.Now(), pm, BatteryInfo{})

	if len(sample.Processes) != topProcessCount {
		t.Fatalf("len(Processes) = %d, want %d", len(sample.Processes), topProcessCount)
	}
	if sample.Processes[0].PID != 14 {
		t.Errorf("Processes[0].PID = %d, want 14 (highest energy impact)", sample.Processes[0].PID)
	}
}

func TestEncodeDecodeLineRoundTrips(t *testing.T) {
	now := time.Date(2026, 7, 31, 19, 25, 12, 0, time.UTC)
	sample := BuildSample(now, PowermetricsResult{CPUPowerMW: 1, ThermalPressure: "Nominal"}, BatteryInfo{CurrentCapacity: 50})

	line, err := EncodeLine(sample)
	if err != nil {
		t.Fatalf("EncodeLine returned error: %v", err)
	}
	if line[len(line)-1] != '\n' {
		t.Fatalf("EncodeLine did not end with newline: %q", line)
	}

	decoded, err := DecodeLine(line)
	if err != nil {
		t.Fatalf("DecodeLine returned error: %v", err)
	}
	if !decoded.Ts.Equal(sample.Ts) || decoded.CPUPowerMW != sample.CPUPowerMW || decoded.BatteryPct != sample.BatteryPct {
		t.Errorf("DecodeLine round-trip mismatch: got %+v, want %+v", decoded, sample)
	}
}
