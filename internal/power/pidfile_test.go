package power

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

func TestFormatParsePidFileRoundTrips(t *testing.T) {
	info := PidInfo{PID: 4242, StartTime: time.Date(2026, 7, 31, 19, 25, 12, 0, time.FixedZone("+03:00", 3*60*60))}

	data := FormatPidFile(info)
	got, err := ParsePidFile(data)
	if err != nil {
		t.Fatalf("ParsePidFile returned error: %v", err)
	}
	if got.PID != info.PID || !got.StartTime.Equal(info.StartTime) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, info)
	}
}

func TestParsePidFileMalformed(t *testing.T) {
	if _, err := ParsePidFile([]byte("not a pidfile")); err == nil {
		t.Fatal("expected error for malformed pidfile, got nil")
	}
}

func TestIsProcessAliveNilError(t *testing.T) {
	alive := IsProcessAlive(1, func(pid int, sig syscall.Signal) error { return nil })
	if !alive {
		t.Error("IsProcessAlive = false, want true for nil error")
	}
}

func TestIsProcessAliveESRCH(t *testing.T) {
	alive := IsProcessAlive(1, func(pid int, sig syscall.Signal) error { return syscall.ESRCH })
	if alive {
		t.Error("IsProcessAlive = true, want false for ESRCH")
	}
}

func TestIsProcessAliveEPERM(t *testing.T) {
	alive := IsProcessAlive(1, func(pid int, sig syscall.Signal) error { return syscall.EPERM })
	if !alive {
		t.Error("IsProcessAlive = false, want true for EPERM (proves pid exists)")
	}
}

func TestIsProcessAliveWrappedESRCH(t *testing.T) {
	alive := IsProcessAlive(1, func(pid int, sig syscall.Signal) error {
		return errors.New("kill: " + syscall.ESRCH.Error())
	})
	if !alive {
		t.Error("IsProcessAlive = false for a non-ESRCH error that merely mentions ESRCH in its message; want true (errors.Is requires exact match)")
	}
}
