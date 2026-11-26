package main

import (
	"bytes"
	"fmt"
	"unsafe"
)

type shortInfo struct {
	Pid, Ppid, Pgid, Status                        uint32
	Comm                                           [16]byte
	Flags, Uid, Gid, Ruid, Rgid, Svuid, Svgid, Rfu uint32
}

func short(pid int32) {
	var s shortInfo
	r := pidinfo(pid, 13, 0, unsafe.Pointer(&s), int32(unsafe.Sizeof(s)))
	fmt.Printf("SHORTBSDINFO pid %d ret %d (size %d) ppid %d uid %d comm %q\n", pid, r, unsafe.Sizeof(s), s.Ppid, s.Uid, s.Comm[:clen(s.Comm[:])])
	var ru rusageV6
	fmt.Println("  rusage rc", pidrusage(pid, 6, unsafe.Pointer(&ru)))
	n, err := procargs2(pid)
	fmt.Println("  procargs2 n", n, "err", err)
	if err == nil {
		b := argbuf[:n]
		argc := *(*int32)(unsafe.Pointer(&b[0]))
		rest := b[4:]
		i := bytes.IndexByte(rest, 0)
		exe := rest[:i]
		for i < len(rest) && rest[i] == 0 {
			i++
		}
		parts := bytes.SplitN(rest[i:], []byte{0}, int(argc)+1)
		fmt.Printf("  argc %d exe %q argv0 %q\n", argc, exe, parts[0])
	}
}
