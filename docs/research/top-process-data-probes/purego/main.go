package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

type rusageV6 struct {
	UUID                    [16]byte
	UserTime, SystemTime    uint64
	PkgIdle, IntrWk, Pagein uint64
	WiredSize, ResidentSize uint64
	PhysFootprint           uint64
	ProcStartAbs, ProcExit  uint64
	ChildUser, ChildSys     uint64
	ChildPkg, ChildIntr     uint64
	ChildPageins, ChildElap uint64
	DiskRead, DiskWritten   uint64
	QoS                     [7]uint64
	BilledSys, ServicedSys  uint64
	LogicalWrites           uint64
	LifetimeMaxFoot         uint64
	Instructions, Cycles    uint64
	BilledEnergy, SvcEnergy uint64
	IntervalMaxFoot         uint64
	Runnable                uint64
	// v6 extras (pcycles .. neural etc.) padded
	Extra [24]uint64
}

type taskInfo struct {
	Virtual, Resident, TotalUser, TotalSystem, ThrUser, ThrSystem                           uint64
	Policy, Faults, Pageins, Cow, MsgS, MsgR, SysMach, SysUnix, Csw, Threads, Running, Prio int32
}

type bsdInfo struct {
	Flags, Status, Xstatus, Pid, Ppid, Uid, Gid, Ruid, Rgid, Svuid, Svgid, Rfu1 uint32
	Comm                                                                        [16]byte
	Name                                                                        [32]byte
	Nfiles, Pgid, Pjobc, Tdev, Tpgid                                            uint32
	Nice                                                                        int32
	StartSec, StartUsec                                                         uint64
}

var (
	listpids    func(typ, info uint32, buf unsafe.Pointer, sz int32) int32
	pidinfo     func(pid, flavor int32, arg uint64, buf unsafe.Pointer, sz int32) int32
	pidrusage   func(pid, flavor int32, buf unsafe.Pointer) int32
	pidpath     func(pid int32, buf unsafe.Pointer, sz uint32) int32
	responsible func(pid int32) int32
	sysctlC     func(name unsafe.Pointer, namelen uint32, oldp unsafe.Pointer, oldlenp unsafe.Pointer, newp unsafe.Pointer, newlen uintptr) int32
	timebase    func(p unsafe.Pointer) int32
)

func init() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&listpids, lib, "proc_listpids")
	purego.RegisterLibFunc(&pidinfo, lib, "proc_pidinfo")
	purego.RegisterLibFunc(&pidrusage, lib, "proc_pid_rusage")
	purego.RegisterLibFunc(&pidpath, lib, "proc_pidpath")
	purego.RegisterLibFunc(&responsible, lib, "responsibility_get_pid_responsible_for_pid")
	purego.RegisterLibFunc(&sysctlC, lib, "sysctl")
	purego.RegisterLibFunc(&timebase, lib, "mach_timebase_info")
}

func pids() []int32 {
	n := listpids(1, 0, nil, 0)
	buf := make([]int32, n/4+64)
	n = listpids(1, 0, unsafe.Pointer(&buf[0]), int32(len(buf)*4))
	return buf[:n/4]
}

var argbuf = make([]byte, 1<<20)

func procargs2(pid int32) (int32, error) {
	mib := [3]int32{unix.CTL_KERN, 49, pid}
	sz := uintptr(len(argbuf))
	r := sysctlC(unsafe.Pointer(&mib[0]), 3, unsafe.Pointer(&argbuf[0]), unsafe.Pointer(&sz), nil, 0)
	if r != 0 {
		return int32(sz), syscall.EINVAL
	}
	return int32(sz), nil
}

type rec struct {
	pid, ppid, resp int32
	uid             uint32
	foot, rss       uint64
	cpuTicks        uint64
	path            string
	coal            uint64
	name            string
	startSec        uint64
}

func readAll(deep bool) (out []rec, errs map[string]int) {
	errs = map[string]int{}
	pathb := make([]byte, 4096)
	for _, p := range pids() {
		if p == 0 {
			continue
		}
		var r rec
		r.pid = p
		var ru rusageV6
		if pidrusage(p, 6, unsafe.Pointer(&ru)) != 0 {
			errs["rusage"]++
			continue
		}
		r.foot, r.rss, r.cpuTicks = ru.PhysFootprint, ru.ResidentSize, ru.UserTime+ru.SystemTime
		var bi bsdInfo
		if pidinfo(p, 3, 0, unsafe.Pointer(&bi), int32(unsafe.Sizeof(bi))) > 0 {
			r.ppid, r.uid, r.startSec = int32(bi.Ppid), bi.Uid, bi.StartSec
			r.name = string(bi.Name[:clen(bi.Name[:])])
		} else {
			errs["bsdinfo"]++
		}
		if deep {
			if n := pidpath(p, unsafe.Pointer(&pathb[0]), 4096); n > 0 {
				r.path = string(pathb[:n])
			} else {
				errs["pidpath"]++
			}
			if _, err := procargs2(p); err != nil {
				errs["procargs2"]++
			}
			r.resp = responsible(p)
			var co [8]uint64
			if pidinfo(p, 20, 0, unsafe.Pointer(&co[0]), 64) > 0 {
				r.coal = co[0]
			}
		}
		out = append(out, r)
	}
	return
}

func clen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

func main() {
	switch os.Args[1] {
	case "bench":
		for _, deep := range []bool{false, true} {
			var ds []time.Duration
			var n int
			var e map[string]int
			for i := 0; i < 50; i++ {
				t := time.Now()
				var rs []rec
				rs, e = readAll(deep)
				ds = append(ds, time.Since(t))
				n = len(rs)
			}
			sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
			fmt.Printf("deep=%v ok=%d errs=%v median=%v p90=%v min=%v\n", deep, n, e, ds[25], ds[45], ds[0])
		}
	case "cpu":
		var tb [2]uint32
		timebase(unsafe.Pointer(&tb[0]))
		fmt.Println("timebase", tb)
		c := exec.Command("sh", "-c", "while :; do :; done")
		c.Start()
		defer c.Process.Kill()
		pid := int32(c.Process.Pid)
		read := func() (uint64, uint64, uint64) {
			var ru rusageV6
			pidrusage(pid, 6, unsafe.Pointer(&ru))
			var ti taskInfo
			pidinfo(pid, 4, 0, unsafe.Pointer(&ti), int32(unsafe.Sizeof(ti)))
			return ru.UserTime + ru.SystemTime, ti.TotalUser + ti.TotalSystem, ru.PhysFootprint
		}
		time.Sleep(500 * time.Millisecond)
		a1, b1, _ := read()
		t1 := time.Now()
		time.Sleep(2 * time.Second)
		a2, b2, _ := read()
		dt := time.Since(t1)
		fmt.Printf("wall %v\n", dt)
		fmt.Printf("rusage ticks delta %d -> raw/ns%% %.2f%%  converted %.2f%%\n", a2-a1, float64(a2-a1)/float64(dt.Nanoseconds())*100, float64(a2-a1)*float64(tb[0])/float64(tb[1])/float64(dt.Nanoseconds())*100)
		fmt.Printf("taskinfo ticks delta %d -> raw %.2f%%  converted %.2f%%\n", b2-b1, float64(b2-b1)/float64(dt.Nanoseconds())*100, float64(b2-b1)*float64(tb[0])/float64(tb[1])/float64(dt.Nanoseconds())*100)
		out, _ := exec.Command("ps", "-o", "%cpu=,time=", "-p", fmt.Sprint(pid)).Output()
		fmt.Printf("ps: %s", out)
	case "mem":
		p := int32(os.Getpid())
		if len(os.Args) > 2 {
			fmt.Sscan(os.Args[2], &p)
		}
		var ru rusageV6
		pidrusage(p, 6, unsafe.Pointer(&ru))
		var ti taskInfo
		pidinfo(p, 4, 0, unsafe.Pointer(&ti), int32(unsafe.Sizeof(ti)))
		fmt.Printf("pid %d footprint=%d resident(rusage)=%d resident(taskinfo)=%d virtual=%d wired=%d\n", p, ru.PhysFootprint, ru.ResidentSize, ti.Resident, ti.Virtual, ru.WiredSize)
	case "sys":
		sysTotals()
	case "apps":
		apps()
	case "short":
		var p int32
		fmt.Sscan(os.Args[2], &p)
		short(p)
	case "v0":
		v0()
	case "sum":
		sum()
	case "resp":
		respCheck()
	case "kproc":
		kproc()
	case "uids":
		rs, _ := readAll(false)
		pl := pids()
		have := map[int32]bool{}
		for _, r := range rs {
			have[r.pid] = true
		}
		kp, _ := unix.SysctlKinfoProcSlice("kern.proc.all")
		byuid := map[uint32][2]int{}
		for _, k := range kp {
			v := byuid[k.Eproc.Ucred.Uid]
			if have[k.Proc.P_pid] {
				v[0]++
			} else {
				v[1]++
			}
			byuid[k.Eproc.Ucred.Uid] = v
		}
		fmt.Println("listpids", len(pl), "kern.proc.all", len(kp), "uid -> [readable, EPERM]", byuid, "my uid", os.Getuid())
		var one []int32
		for _, k := range kp {
			if !have[k.Proc.P_pid] && len(one) < 3 {
				one = append(one, k.Proc.P_pid)
				fmt.Printf("unreadable pid %d comm %s uid %d  ppid %d\n", k.Proc.P_pid, string(k.Proc.P_comm[:clen(k.Proc.P_comm[:])]), k.Eproc.Ucred.Uid, k.Eproc.Ppid)
			}
		}
	}
}
