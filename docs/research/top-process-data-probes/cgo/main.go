package main

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <sys/resource.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"sort"
	"time"
	"unsafe"
)

func readAll() int {
	n := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	buf := make([]C.int, int(n)/4+64)
	n = C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&buf[0]), C.int(len(buf)*4))
	ok := 0
	for _, p := range buf[:int(n)/4] {
		if p == 0 {
			continue
		}
		var ru C.struct_rusage_info_v6
		if C.proc_pid_rusage(p, C.RUSAGE_INFO_V6, (*C.rusage_info_t)(unsafe.Pointer(&ru))) != 0 {
			continue
		}
		var bi C.struct_proc_bsdinfo
		C.proc_pidinfo(p, C.PROC_PIDTBSDINFO, 0, unsafe.Pointer(&bi), C.int(unsafe.Sizeof(bi)))
		ok++
	}
	return ok
}

func main() {
	var ds []time.Duration
	var ok int
	for i := 0; i < 50; i++ {
		t := time.Now()
		ok = readAll()
		ds = append(ds, time.Since(t))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	fmt.Println("cgo ok", ok, "median", ds[25], "p90", ds[45])
}
