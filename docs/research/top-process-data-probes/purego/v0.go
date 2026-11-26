package main

import (
	"fmt"
	"os"
	"unsafe"
)

type rusageV0 struct {
	UUID                                                                                [16]byte
	User, System, PkgIdle, Intr, Pageins, Wired, Resident, Footprint, StartAbs, ExitAbs uint64
}

func v0() {
	pid := int32(os.Getpid())
	var a rusageV0
	var b rusageV6
	ra := pidrusage(pid, 0, unsafe.Pointer(&a))
	_ = pidrusage(pid, 6, unsafe.Pointer(&b))
	fmt.Printf("V0 rc %d size %d footprint %d resident %d start %d | V6 footprint %d resident %d start %d\n", ra, unsafe.Sizeof(a), a.Footprint, a.Resident, a.StartAbs, b.PhysFootprint, b.ResidentSize, b.ProcStartAbs)
}
