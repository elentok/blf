package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func respCheck() {
	kp, _ := unix.SysctlKinfoProcSlice("kern.proc.all")
	var mineOK, mineN, otherOK, otherN int
	for _, k := range kp {
		p := k.Proc.P_pid
		if p == 0 {
			continue
		}
		r := responsible(p)
		if k.Eproc.Ucred.Uid == 501 {
			mineN++
			if r > 0 {
				mineOK++
			}
		} else {
			otherN++
			if r > 0 {
				otherOK++
			}
		}
	}
	fmt.Println("responsible pid ok: mine", mineOK, "/", mineN, "other uids", otherOK, "/", otherN)
	for _, p := range []int32{734, 91295, 95417} {
		b := make([]byte, 4096)
		n := pidpath(p, ptr(b), 4096)
		fmt.Println(p, string(b[:max(n, 0)]))
	}
}

func ptr(b []byte) unsafe.Pointer { return unsafe.Pointer(&b[0]) }
