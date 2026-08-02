// Package power contains pure parsing/encoding logic for `blf power`'s
// background sampling daemon: powermetrics/ioreg plist parsing, JSONL sample
// encoding, pidfile format, and retention-pruning selection. Nothing in this
// package touches the filesystem or spawns processes — callers (cmd/power.go)
// inject raw bytes and get back plain values.
package power

import (
	"encoding/json"
	"sort"
	"time"

	"howett.net/plist"
)

// CurrentSchemaVersion is written as the first field of every JSONL sample line.
const CurrentSchemaVersion = 1

// topProcessCount is how many processes (ranked by EnergyImpactPerS) are kept
// per sample.
const topProcessCount = 10

// ProcessSample is one process's entry in a Sample's Processes list.
type ProcessSample struct {
	PID              int     `json:"pid"`
	Name             string  `json:"name"`
	EnergyImpactPerS float64 `json:"energy_impact_per_s"`
	CPUMsPerS        float64 `json:"cpu_ms_per_s"`
}

// Sample is one 30s tick, serialized as a single JSONL line.
type Sample struct {
	SchemaVersion int       `json:"schema_version"`
	Ts            time.Time `json:"ts"`

	CPUPowerMW      float64 `json:"cpu_power_mw"`
	GPUPowerMW      float64 `json:"gpu_power_mw"`
	CombinedPowerMW float64 `json:"combined_power_mw"`
	ThermalPressure string  `json:"thermal_pressure"`

	BatteryPct           int   `json:"battery_pct"`
	BatteryRawCurrentMAh int64 `json:"battery_raw_current_mah"`
	BatteryRawMaxMAh     int64 `json:"battery_raw_max_mah"`
	BatteryAmperageMA    int64 `json:"battery_amperage_ma"`
	BatteryVoltageMV     int64 `json:"battery_voltage_mv"`
	BatteryCharging      bool  `json:"battery_charging"`
	BatteryACConnected   bool  `json:"battery_ac_connected"`
	BatteryCycleCount    int   `json:"battery_cycle_count"`
	BatteryFullyCharged  bool  `json:"battery_fully_charged"`

	Processes []ProcessSample `json:"processes"`
}

// PowermetricsResult is the parsed subset of `powermetrics -f plist` output
// that `blf power` cares about.
type PowermetricsResult struct {
	CPUPowerMW      float64
	GPUPowerMW      float64
	CombinedPowerMW float64
	ThermalPressure string
	Processes       []ProcessSample
}

// BatteryInfo is the parsed subset of `ioreg -rc AppleSmartBattery -a` output
// that `blf power` cares about.
type BatteryInfo struct {
	CurrentCapacity         int
	AppleRawCurrentCapacity int64
	AppleRawMaxCapacity     int64
	Amperage                int64
	Voltage                 int64
	IsCharging              bool
	ExternalConnected       bool
	CycleCount              int
	FullyCharged            bool
}

type powermetricsPlist struct {
	Processor       powermetricsProcessor `plist:"processor"`
	ThermalPressure string                `plist:"thermal_pressure"`
	Tasks           []powermetricsTask    `plist:"tasks"`
}

type powermetricsProcessor struct {
	CPUPower      float64 `plist:"cpu_power"`
	GPUPower      float64 `plist:"gpu_power"`
	CombinedPower float64 `plist:"combined_power"`
}

type powermetricsTask struct {
	PID              int     `plist:"pid"`
	Name             string  `plist:"name"`
	EnergyImpactPerS float64 `plist:"energy_impact_per_s"`
	CPUTimeMsPerS    float64 `plist:"cputime_ms_per_s"`
}

// ParsePowermetrics parses the XML plist produced by:
//
//	sudo -n powermetrics -n 1 -i 1000 --samplers cpu_power,gpu_power,tasks,thermal --show-process-energy -f plist
func ParsePowermetrics(data []byte) (PowermetricsResult, error) {
	var raw powermetricsPlist
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return PowermetricsResult{}, err
	}

	processes := make([]ProcessSample, len(raw.Tasks))
	for i, task := range raw.Tasks {
		processes[i] = ProcessSample{
			PID:              task.PID,
			Name:             task.Name,
			EnergyImpactPerS: task.EnergyImpactPerS,
			CPUMsPerS:        task.CPUTimeMsPerS,
		}
	}

	return PowermetricsResult{
		CPUPowerMW:      raw.Processor.CPUPower,
		GPUPowerMW:      raw.Processor.GPUPower,
		CombinedPowerMW: raw.Processor.CombinedPower,
		ThermalPressure: raw.ThermalPressure,
		Processes:       processes,
	}, nil
}

type ioregEntry struct {
	CurrentCapacity         int   `plist:"CurrentCapacity"`
	AppleRawCurrentCapacity int64 `plist:"AppleRawCurrentCapacity"`
	AppleRawMaxCapacity     int64 `plist:"AppleRawMaxCapacity"`
	Amperage                int64 `plist:"Amperage"`
	Voltage                 int64 `plist:"Voltage"`
	IsCharging              bool  `plist:"IsCharging"`
	ExternalConnected       bool  `plist:"ExternalConnected"`
	CycleCount              int   `plist:"CycleCount"`
	FullyCharged            bool  `plist:"FullyCharged"`
}

// ParseIoreg parses the XML plist produced by `ioreg -rc AppleSmartBattery -a`
// (an array containing a single battery dict).
func ParseIoreg(data []byte) (BatteryInfo, error) {
	var entries []ioregEntry
	if _, err := plist.Unmarshal(data, &entries); err != nil {
		return BatteryInfo{}, err
	}
	if len(entries) == 0 {
		return BatteryInfo{}, nil
	}

	e := entries[0]
	return BatteryInfo{
		CurrentCapacity:         e.CurrentCapacity,
		AppleRawCurrentCapacity: e.AppleRawCurrentCapacity,
		AppleRawMaxCapacity:     e.AppleRawMaxCapacity,
		Amperage:                e.Amperage,
		Voltage:                 e.Voltage,
		IsCharging:              e.IsCharging,
		ExternalConnected:       e.ExternalConnected,
		CycleCount:              e.CycleCount,
		FullyCharged:            e.FullyCharged,
	}, nil
}

// BuildSample combines a parsed powermetrics result and battery info into one
// tick's Sample, keeping only the top topProcessCount processes by
// EnergyImpactPerS (descending).
func BuildSample(now time.Time, pm PowermetricsResult, batt BatteryInfo) Sample {
	return Sample{
		SchemaVersion: CurrentSchemaVersion,
		Ts:            now,

		CPUPowerMW:      pm.CPUPowerMW,
		GPUPowerMW:      pm.GPUPowerMW,
		CombinedPowerMW: pm.CombinedPowerMW,
		ThermalPressure: pm.ThermalPressure,

		BatteryPct:           batt.CurrentCapacity,
		BatteryRawCurrentMAh: batt.AppleRawCurrentCapacity,
		BatteryRawMaxMAh:     batt.AppleRawMaxCapacity,
		BatteryAmperageMA:    batt.Amperage,
		BatteryVoltageMV:     batt.Voltage,
		BatteryCharging:      batt.IsCharging,
		BatteryACConnected:   batt.ExternalConnected,
		BatteryCycleCount:    batt.CycleCount,
		BatteryFullyCharged:  batt.FullyCharged,

		Processes: topProcesses(pm.Processes, topProcessCount),
	}
}

func topProcesses(all []ProcessSample, n int) []ProcessSample {
	sorted := make([]ProcessSample, len(all))
	copy(sorted, all)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EnergyImpactPerS > sorted[j].EnergyImpactPerS
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// EncodeLine encodes a Sample as one JSONL line, including the trailing newline.
func EncodeLine(s Sample) ([]byte, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeLine decodes one JSONL line (with or without trailing newline) back
// into a Sample.
func DecodeLine(line []byte) (Sample, error) {
	var s Sample
	err := json.Unmarshal(line, &s)
	return s, err
}
