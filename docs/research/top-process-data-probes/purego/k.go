package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"time"
)

func kproc() {
	t := time.Now()
	kp, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	d := time.Since(t)
	fmt.Println("kern.proc.all", len(kp), err, d)
	for _, k := range kp {
		if k.Proc.P_pid == 1 || k.Proc.P_pid == 60977 || k.Proc.P_pid == 95417 {
			fmt.Printf("pid %d uid %d pctcpu %d p_cpu %d xrssize %d start %d comm %s\n", k.Proc.P_pid, k.Eproc.Ucred.Uid, k.Proc.P_pctcpu, k.Proc.P_estcpu, k.Eproc.Xrssize, k.Proc.P_starttime.Sec, string(k.Proc.P_comm[:clen(k.Proc.P_comm[:])]))
		}
	}
}
