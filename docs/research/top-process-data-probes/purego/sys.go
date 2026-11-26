package main

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

type vmstat struct {
	Free, Active, Inactive, Wire                                           uint32
	ZeroFill, React, Pageins, Pageouts, Faults, Cow, Lookups, Hits, Purges uint64
	Purgeable, Speculative                                                 uint32
	Decomp, Comp, Swapins, Swapouts                                        uint64
	Compressor, Throttled, External, Internal                              uint32
	Uncompressed                                                           uint64
	Rest                                                                   [64]uint64
}

func sysTotals() {
	lib, _ := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	var hostSelf func() uint32
	var hostStats64 func(host uint32, flavor int32, out unsafe.Pointer, cnt unsafe.Pointer) int32
	var hostStats func(host uint32, flavor int32, out unsafe.Pointer, cnt unsafe.Pointer) int32
	var procInfo func(host uint32, flavor int32, ncpu unsafe.Pointer, info unsafe.Pointer, cnt unsafe.Pointer) int32
	purego.RegisterLibFunc(&hostSelf, lib, "mach_host_self")
	purego.RegisterLibFunc(&hostStats64, lib, "host_statistics64")
	purego.RegisterLibFunc(&hostStats, lib, "host_statistics")
	purego.RegisterLibFunc(&procInfo, lib, "host_processor_info")
	h := hostSelf()
	t0 := time.Now()
	var vm vmstat
	cnt := uint32(38) // HOST_VM_INFO64_REV0_COUNT-ish; kernel accepts >= rev0
	r := hostStats64(h, 4, unsafe.Pointer(&vm), unsafe.Pointer(&cnt))
	fmt.Println("host_statistics64 rc", r, "count back", cnt, "took", time.Since(t0))
	pg := uint64(unix.Getpagesize())
	total, _ := unix.SysctlUint64("hw.memsize")
	app := (uint64(vm.Internal) - uint64(vm.Purgeable)) * pg
	wired := uint64(vm.Wire) * pg
	comp := uint64(vm.Compressor) * pg
	cached := (uint64(vm.External) + uint64(vm.Purgeable)) * pg
	fmt.Printf("total %.2fG app %.2fG wired %.2fG compressed %.2fG => used %.2fG ; cached files %.2fG free %.2fG\n", f(total), f(app), f(wired), f(comp), f(app+wired+comp), f(cached), f(uint64(vm.Free)*pg))
	sw, _ := unix.SysctlRaw("vm.swapusage")
	fmt.Printf("swapusage raw len %d total=%.0fM used=%.0fM\n", len(sw), float64(*(*uint64)(unsafe.Pointer(&sw[0])))/1048576, float64(*(*uint64)(unsafe.Pointer(&sw[16])))/1048576)
	lvl, _ := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	pct, _ := unix.SysctlUint32("kern.memorystatus_level")
	fmt.Println("pressure level", lvl, "memorystatus_level(% free)", pct)
	// CPU: two samples
	read := func() [4]uint32 {
		var l [4]uint32
		c := uint32(4)
		hostStats(h, 3, unsafe.Pointer(&l[0]), unsafe.Pointer(&c))
		return l
	}
	a := read()
	time.Sleep(time.Second)
	b := read()
	var d [4]float64
	var sum float64
	for i := range d {
		d[i] = float64(b[i] - a[i])
		sum += d[i]
	}
	fmt.Printf("total CPU user %.1f%% sys %.1f%% idle %.1f%% (ticks/s %.0f)\n", d[0]/sum*100, d[1]/sum*100, d[2]/sum*100, sum)
	var n uint32
	var inf uintptr
	var ic uint32
	t1 := time.Now()
	r = procInfo(h, 2, unsafe.Pointer(&n), unsafe.Pointer(&inf), unsafe.Pointer(&ic))
	fmt.Println("host_processor_info rc", r, "cpus", n, "took", time.Since(t1))
}
func f(b uint64) float64 { return float64(b) / (1 << 30) }
