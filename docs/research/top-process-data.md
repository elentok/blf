# How `blf top` reads process data on macOS

Research for ticket `blf:top/01`. Machine for all measurements: Apple Silicon (arm64), 12 cores, 24 GB,
macOS 26 (Darwin 27.2.0), about 720-820 processes, run as a normal user (uid 501, no sudo).
Go 1.27.4 locally; `go.mod` says `go 1.25.0`.

Sources, with short names used below:

- **SDK**: headers in `/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/include`
  (`libproc.h`, `sys/proc_info.h`, `sys/resource.h`, `mach/*.h`).
- **XNU**: <https://github.com/apple-oss-distributions/xnu> (`main` branch, fetched 2026-11-26):
  `bsd/kern/proc_info.c`, `bsd/sys/proc_info_private.h`, `bsd/kern/kern_resource.c`,
  `osfmk/kern/bsd_kern.c`, `bsd/kern/kern_sysctl.c`, `libsyscall/wrappers/libproc/libproc.c`.
- **Probes**: code in `docs/research/top-process-data-probes/` (Go + purego, Go + cgo, plain C).
  Output quoted below was produced by them on this Mac.

## Summary

| Need | Use | Needs sudo for others' processes? |
| --- | --- | --- |
| Process list, pid, ppid, uid, short name, start time | `sysctl kern.proc.all` (pure Go, `x/sys/unix`) or `proc_listpids` + `PROC_PIDT_SHORTBSDINFO` | No. Works for every user. |
| CPU time | `proc_pid_rusage(RUSAGE_INFO_V0)` -> `ri_user_time + ri_system_time`, Mach ticks, convert with `mach_timebase_info` | Yes (EPERM) |
| Memory | `ri_phys_footprint` from the same call. Show footprint, not RSS. | Yes (EPERM) |
| Executable path | `proc_pidpath` | No. Works for every user. |
| Full argv | `sysctl KERN_PROCARGS2` | Yes (EINVAL) |
| App membership | `responsibility_get_pid_responsible_for_pid` (private; own processes only), `PROC_PIDCOALITIONINFO` (all users), `proc_pidpath` (all users) | Responsible pid: yes (returns no pid for other uids). Coalition and path: no. |
| System CPU / memory / swap / pressure | `host_statistics`, `host_statistics64`, `sysctl` | No |
| Go binding | `github.com/ebitengine/purego` against libSystem, `CGO_ENABLED=0` | - |

One full read of every process: about 0.7 ms (CPU + memory + parent + uid), 4-5 ms with path, argv,
responsible pid and coalition id for every process. Details in section 7.

## 1. CPU %

**Source of CPU time.** Use `proc_pid_rusage`. It returns `ri_user_time` and `ri_system_time` (cumulative CPU
time of the task). XNU fills them in `fill_task_rusage` from `task_power_info` (`osfmk/kern/bsd_kern.c`:
`ri->ri_user_time = powerinfo.total_user; ri->ri_system_time = powerinfo.total_system;`), and
`gather_rusage_info` calls it for every flavor including `RUSAGE_INFO_V0` (`bsd/kern/kern_resource.c`).
`proc_pidinfo(PROC_PIDTASKINFO)` returns `pti_total_user/pti_total_system` with the same values (probe:
both gave `46966438` ticks over the same 2 s window). Prefer `proc_pid_rusage`, because the same call also
gives `ri_phys_footprint` (section 2) and `ri_proc_start_abstime`.

`sysctl KERN_PROC` is not a CPU source. `kinfo_proc.p_pctcpu` and `p_estcpu` were `0` for launchd, Safari
and a root daemon in the probe (`./gp kproc`). Do not use it.

**Units.** `ri_user_time` and `ri_system_time` are Mach absolute time ticks, not nanoseconds.
`mach_timebase_info` is required on Apple Silicon. On this Mac it returns `numer=125, denom=3`
(one tick = 41.67 ns). Proof: a `sh` busy loop was sampled over 2.0 s.

```
timebase [125 3]
rusage ticks delta 46966438 -> raw/ns 2.35%  converted 97.78%
ps: 98.1
```

Without the conversion the busy loop reads 2.35% instead of about 98%. On Intel the ratio is 1/1, so the
conversion is a no-op there. Always apply it: `ns = ticks * numer / denom`. The ratio is constant for the
life of the boot, so read it once at startup.

**Formula.** Keep the previous sample per `(pid, start time)`.
`cpu% = (Δ(user+system) in ns) / (Δ wall-clock in ns) * 100`. This is "percent of one core", so a
process can exceed 100% (same convention as `ps` and `top`). Use a monotonic clock (`time.Since`) for the wall
delta, not the nominal refresh interval, because the refresh jitters. The first sample of a process has no
delta; show `0` or `-`.

Processes that die between samples drop out. A pid that is reused has a different start time
(`ri_proc_start_abstime`), so key the previous-sample map on `(pid, start)`.

## 2. Memory

**Recommended number: `ri_phys_footprint`** from `proc_pid_rusage`. It is the number `top(1)` documents as
the process's memory ("mem: Physical memory footprint of the process", `man top`, `-stats` keyword list).
Proof that `top`'s MEM is this field:

```
pid 95417 (Safari)  ri_phys_footprint=217728872 B = 207.6 MiB   top -pid 95417 -stats mem  -> 208M
pid 734  (Chrome)   ri_phys_footprint=142477232 B = 135.9 MiB   top -pid 734  -stats mem  -> 136M
```

**Activity Monitor.** I could not read Activity Monitor's source or inspect its UI from a script, so
"Activity Monitor's Memory column equals `phys_footprint`" is **not proven here**. The evidence is indirect: Apple's
own `top` uses the footprint and `footprint(1)` is Apple's tool for the same ledger. Treat the match as
very likely, not certain.

**Difference from RSS.** `ri_resident_size` (and `pti_resident_size`, and `ps -o rss`) counts only pages
currently in RAM. XNU reads it from the `phys_mem` ledger, and the footprint from the `phys_footprint`
ledger (`fill_task_rusage`: `ri->ri_phys_footprint = get_task_phys_footprint(task); ledger_get_balance(...
task_ledgers.phys_mem, &ri->ri_resident_size)`). The footprint ledger also counts dirty compressed and swapped
memory and drops clean file-backed pages that are shared or reclaimable. The effect in the data:

```
Safari   footprint 207.6 MB   RSS 29.5 MB    (most of its memory is compressed/swapped)
Chrome   footprint 135.9 MB   RSS 99.3 MB
```

RSS under-reports a process whose pages were compressed and over-reports one with a large mapped file cache.
For "what is eating my memory", the footprint is the number that matches what the system would get back if
the process quit. Show the footprint as the main column. RSS can stay a secondary detail column.

The sum of footprints is not the system "used" figure. Across the 477 readable processes the footprints
sum to 14.58 GB, while app + wired + compressed from `vm_statistics64` was 13.49 GB. Shared memory and
unreadable processes explain the gap. Do not present the sum as a total. Use the system total (section 8).

**Which flavor.** `RUSAGE_INFO_V0` is enough (macOS 10.9+, SDK `sys/resource.h`: `struct rusage_info_v0`
has user, system, wired, resident, `ri_phys_footprint` and `ri_proc_start_abstime`). The probe shows V0 and V6
return identical footprint, resident and start (`./gp v0`). Later flavors add fields, and XNU's `switch`
falls through from V6 down to V0, so asking for V0 returns exactly the V0 fields and stays valid on
older and newer macOS.

## 3. Owner, parent, name, argv, start time

| Field | Source | Notes |
| --- | --- | --- |
| uid, ppid | `sysctl kern.proc.all` (`KinfoProc.Eproc.Ucred.Uid`, `.Eproc.Ppid`) or `proc_pidinfo(PROC_PIDT_SHORTBSDINFO)` | Both work for every user. Probe: short info for root pid 60977 returned `ppid 1 uid 0 comm "peakpowermanage"`. XNU `proc_info.c` lists `PROC_PIDT_SHORTBSDINFO` under "Certain operations don't require privileges" (`NO_CHECK_SAME_USER`). |
| short name | `comm` (16 bytes, truncated: `peakpowermanage` for `peakpowermanagerd`) | Prefer the basename of `proc_pidpath` for display. |
| path | `proc_pidpath` | Works for every user (probe: 1037 of 1039 pids; the 2 failures were `ESRCH`, process gone). XNU: `PROC_PIDPATHINFO` is `NO_CHECK_SAME_USER`. Buffer must be at least `PROC_PIDPATHINFO_SIZE` and at most `PROC_PIDPATHINFO_MAXSIZE` (4096) (`libproc.c`). |
| argv | `sysctl KERN_PROCARGS2` (`mib = {CTL_KERN, 49, pid}`) | Own processes only. Layout: `int32 argc`, exec path (NUL-terminated), NUL padding, `argc` NUL-terminated argv strings, then env strings. XNU strips env vars for `cs_restricted` processes unless the caller owns them, so read only up to `argc` strings and ignore the rest (`kern_sysctl.c`, `sysctl_procargsx`). Probe parsed kitty: `argc 1 argv0 "/Users/david/Applications/kitty.app/Contents/MacOS/kitty"`. `x/sys/unix` has no constant for it; `KERN_PROCARGS2` is `49`. |
| start time | `kern.proc.all` `P_starttime` (all users), `ri_proc_start_abstime` (Mach ticks, own), `pbi_start_tvsec` in `PROC_PIDTBSDINFO` (own) | `kern.proc.all` is the only one that works for other users. |

Argv costs about 3 ms per 800 processes (C probe), the most expensive field. Read it once when a process
first appears (key on `(pid, start time)`) and cache it. Same for path and coalition id.

`PROC_PIDTBSDINFO` (flavor 3) gives a 32-byte `pbi_name` plus start time but is own-processes-only
(EPERM otherwise).

## 4. App membership

Three signals exist. None is documented as public API except the executable path.

1. **Executable path** (`proc_pidpath`): find the outermost `*.app` segment. Public, simple, works for all
   users. It misses helpers that do not live inside the app bundle. In the probe, WebKit's
   `com.apple.WebKit.WebContent` processes (7 of them, 1.46 GB) live under the system WebKit framework, not
   under `Safari.app`, so a path grouping lists them as a separate "(no .app)" group.
2. **Responsible process** (`responsibility_get_pid_responsible_for_pid(pid)`): the launch-services notion
   of "which app is responsible for this process". Declared in no SDK header. It is exported by libSystem:
   `dlsym` resolves it (`./r` in the C probe). XNU stores the value as `p_responsible_pid`
   (`bsd/kern/kern_proc.c`, `proc_set_responsible_pid`). Returned a pid for every own process, but
   not for other users: `./gp resp` returned a pid for 470 of 470 own processes and for 0 of 228 processes of
   other uids. Result: Safari (15 processes, 1423 MB),
   ChatGPT.app (18), Spotify.app (7), kitty.app (8) grouped correctly, including Safari's WebContent
   helpers. For a terminal child started by a daemon the responsible pid can be a non-app daemon (`gx`,
   n=41); handle "responsible process has no `.app`" by falling back to the process name.
   Risk: private symbol, can change in any release. Mitigation: resolve with `dlsym` at startup and fall
   back to path grouping if the symbol is missing.
3. **Coalitions** (`proc_pidinfo(pid, PROC_PIDCOALITIONINFO = 20, ...)`): returns `coalition_id[2]`
   (resource coalition, jetsam coalition; `bsd/sys/proc_info_private.h`). XNU marks it
   `NO_CHECK_SAME_USER`, and it worked for every pid. Processes in one app share a resource coalition id
   (Chrome main plus all helpers: `848/849`; Safari plus XPC services: `787/1803`). It is in the private
   header only (`proc_info_private.h`, not in the SDK), so the flavor number is an undocumented constant.
   The coalition does not carry a name or app path. Use it to bucket processes, then name the bucket
   from the member whose pid is the coalition leader or whose path is an `.app`.

**Observation that matters for the design.** In the probe a Safari helper (`pid 91295`,
`com.apple.SafariPlatformSupport.Helper`) and Chrome's helpers share coalition `848/849` and responsible pid
`734` (`/Applications/Google Chrome.app`). A path-based or coalition-based grouping can merge
helpers into a different parent than expected. Do not assume all three signals agree. Recommended: group by
responsible pid for own processes, name the group from the `.app` in the responsible process's path (fallback: its name), and
keep path-grouping as the fallback when the symbol is missing or the process is another user's.

**What Activity Monitor uses.** Not found in any public source; unproven. Activity Monitor is closed source.
I only know the three kernel/libSystem facilities above exist and group the data this way.

## 5. Access limits (no sudo)

Measured as uid 501 (`./gp uids`): of 819 pids, 592 are mine, 227 are not (the benchmark runs saw 226 `EPERM`; processes come and go). Of the 227,
144 are root-owned and 83 belong to other system service uids (for example 88, 200-308). **All** processes of other uids fail, not only root's.

| Call | Other uid, as non-root | Why (XNU) |
| --- | --- | --- |
| `proc_pid_rusage` | returns `-1`, errno `EPERM` | `proc_security_policy` returns `EPERM` when uids differ unless the caller has `PRIV_GLOBAL_PROC_INFO` (`proc_info.c`). |
| `proc_pidinfo` `PROC_PIDTASKINFO`, `PROC_PIDTBSDINFO` | returns `0`, errno `EPERM` | same |
| `sysctl KERN_PROCARGS2` | fails, errno `EINVAL` | `sysctl_procargsx`: `uid != caller uid && suser() fails -> EINVAL` |
| `proc_pidpath` | works | `NO_CHECK_SAME_USER` |
| `proc_pidinfo` `PROC_PIDT_SHORTBSDINFO`, `PROC_PIDCOALITIONINFO` | work | `NO_CHECK_SAME_USER` |
| `responsibility_get_pid_responsible_for_pid` | returns no pid (0 of 228 other-uid processes) | observed; the call is private, reason not in public source |
| `sysctl kern.proc.all` | works (name, uid, ppid, start) | observed: 819 entries incl. root |
| `kill(pid, sig)` | `EPERM` for other uids | not probed (no need to kill anything); standard kill(2) |

Consequences:

- Process-level CPU and memory for other users' processes (and for `kernel_task`, pid 0) need root or the
  `PRIV_GLOBAL_PROC_INFO` entitlement. blf can show them in a list as hidden ("name only") or leave them
  out. For a tool that only kills your own processes, filtering to your uid is right. Per-process numbers for
  the other 227 are unavailable, but their share still shows up in the system total (section 8).
- `/bin/ps` and `/usr/bin/top` are setuid root (`ls -l`: `-rwsr-xr-x /bin/ps`, `-r-sr-xr-x /usr/bin/top`),
  which is why `ps` shows `peakpowermanagerd` with `%CPU` and `RSS` without sudo. blf cannot be setuid
  and must not claim it. Shelling out to `ps` would reveal other users' CPU and RSS, but not the footprint,
  and only as the scheduler's decayed average. One extra possibility is `top -l 2 -stats pid,cpu,mem`, which
  does show root processes' footprint, at about 230 ms per call. It is an option only if the product wants
  the system processes. I do not recommend it as the main path.
- A pid can vanish between the list and the read. Expect `ESRCH` or `EPERM` mixed with real failures, and
  skip the process on any error instead of treating it as fatal.

I could not run anything as root, so the claim that root would be able to read everything is from the XNU
source only, not from a probe.

## 6. Go options

| Option | Cgo? | Cross-compile from Linux (as in `release.yml`) | Footprint? | Speed (800 procs) | Verdict |
| --- | --- | --- | --- | --- | --- |
| cgo to libproc | yes | **breaks** | yes | 0.47 ms | no |
| `x/sys/unix` sysctl only | no | works | no (no CPU or memory) | 0.24 ms | too thin, but good for the process list |
| **purego** + libSystem | no | works | yes | 0.67 ms | **recommended** |
| `gopsutil/v4` | no (uses purego internally) | works | no (RSS only) | 7-9 ms, 17-34 ms with names | no |
| shell out to `ps`/`top` | no | works | no (`ps`) / yes (`top`) | 20 ms (`ps -ax`) / 230 ms (`top -l 1`) | no |

**blf constraints.** `go.mod` has no cgo dependency. `.goreleaser.yml` sets `CGO_ENABLED=0` for linux and
darwin on amd64 and arm64, and `release.yml` runs on `ubuntu-latest` (cross-compiles the darwin binaries).
CI builds on `ubuntu-latest` and `macos-latest` with `go build ./...`.

**cgo would break the release.** Proof: `CGO_ENABLED=0 GOOS=darwin go build` of the cgo probe fails with
`build constraints exclude all Go files` (the cgo file is dropped). Go defaults `CGO_ENABLED` to `0` when
cross-compiling (`GOOS=linux go env CGO_ENABLED` and `GOARCH=amd64 go env CGO_ENABLED` both printed `0` on
this arm64 Mac). Building darwin cgo from an Ubuntu runner would need an Apple SDK and cross C toolchain
(osxcross or zig), or moving the darwin release job to a macOS runner. `blf top` would also lose its
tap-friendly pure-Go release. That is a large cost for a 0.2 ms saving.

**Recommended: purego.** `github.com/ebitengine/purego` v0.11.3 (Apache-2.0) calls C functions in a loaded
dylib without cgo. Probe: `CGO_ENABLED=0 GOOS=darwin GOARCH=amd64|arm64 go build` both succeed. darwin
amd64 and arm64 are its tier 1 targets (`README.md`). `go.mod` says `go 1.25.0`, same as blf.
Pattern (see `probes/purego/main.go`):

```go
lib, _ := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
purego.RegisterLibFunc(&procPidRusage, lib, "proc_pid_rusage")
purego.RegisterLibFunc(&procPidInfo,   lib, "proc_pidinfo")
purego.RegisterLibFunc(&procPidPath,   lib, "proc_pidpath")
```

Register once at startup. Structs are plain `uint64` Go structs matching the C layout. Put the code in a
file with `//go:build darwin` and add a stub for other OSes (Linux CI runs `go build ./...`).

Why not the others:

- `gopsutil/v4` already uses purego internally, which proves the approach works, but its API is wrong for
  this job. `MemoryInfo` returns RSS and VMS only, no footprint. `Times` ignores the return code of
  `ProcPidInfo`, so unreadable processes appear with zero (719 of 719 "ok" while 226 were unreadable). It
  calls `loadProcFuncs()` (a `dlopen`) per call, and `Name`, `Status` and others shell out to `ps`
  (`process/process_darwin.go`). Measured 7-9 ms for list + times + RSS and 17-34 ms with names and ppid.
  It also pulls in 100+ packages (`go list -deps`: 105 for my probe) and a large platform surface blf does
  not use.
- `ps`/`top` text parsing is fragile, slower (20 ms / 230 ms) and gives RSS (`ps`), not footprint.
- `x/sys/unix` has `SysctlKinfoProcSlice("kern.proc.all")`, which is perfect for the list (0.24 ms, 803
  entries incl. other users) and is pure Go. It has no `proc_pid_rusage`.

Recommended split: `x/sys/unix` for `kern.proc.all`, `vm.swapusage`, `hw.memsize`,
`kern.memorystatus_vm_pressure_level`; purego for `proc_pid_rusage`, `proc_pidpath`,
`proc_pidinfo(COALITION)`, `responsibility_get_pid_responsible_for_pid`, `sysctl(KERN_PROCARGS2)`,
`host_statistics`, `host_statistics64`, `mach_timebase_info`. (`unix.SysctlRaw` takes a name string, not a
numeric mib; for `KERN_PROCARGS2` call libc `sysctl` through purego, as the probe does.)

The "correct fix" reason for purego over a quick shell-out: it calls the same kernel interfaces `top` and
Activity Monitor use, gets the right memory field, keeps the single static binary and cross-compiled release,
and avoids a parse-text layer that breaks silently on locale or column changes.

## 7. Cost of one full read

Release builds (`-O2` for C; Go default). Median of 50 reads, ~720-820 listed pids, 477-600 readable by uid:

| What | Impl | Median | p90 |
| --- | --- | --- | --- |
| `proc_listpids` + `proc_pid_rusage` + `PROC_PIDTBSDINFO` for all pids | Go + purego | 0.67-1.1 ms | 0.77-1.4 ms |
| same | Go + cgo | 0.47-0.59 ms | 0.53-0.74 ms |
| plus `proc_pidpath`, `KERN_PROCARGS2`, responsible pid, coalition id for all pids | Go + purego | 4.4-6.9 ms | 5.4-8.2 ms |
| same, in C | C | 7.6-8.0 ms (with timer overhead) | - |
| `sysctl kern.proc.all` only | Go | 0.24 ms | - |
| `host_statistics64`, `host_processor_info` | Go + purego | 13 us, 23 us | - |
| gopsutil list + Times + MemoryInfo | Go | 7-16 ms | - |
| gopsutil + Name + Ppid | Go | 17-34 ms | - |
| `ps -axo pid,ppid,uid,%cpu,rss,comm` | shell | 20 ms | - |
| `top -l 1 -n 0 -stats pid,cpu,mem` | shell | 232 ms | - |

(The ranges come from several runs while Chrome and other apps churned processes.) The per-refresh work in
steady state is the first row. The expensive fields (path, argv, coalition, responsible pid) are static for
a process, so read them once per new `(pid, start time)`. At a 2 s refresh, a 1 ms read is about 0.05% of
one core. Failing calls for other users' processes are cheap (they return immediately with `EPERM`), but
you can also skip them up front by filtering to your uid from `kern.proc.all`.

## 8. System totals

All of these work without sudo (probe: `./gp sys`).

**CPU.** `host_statistics(mach_host_self(), HOST_CPU_LOAD_INFO=3, ...)` returns cumulative
`cpu_ticks[user, system, idle, nice]` (`mach/host_info.h`, `mach/machine.h`: `CPU_STATE_USER=0`, `SYSTEM=1`,
`IDLE=2`, `NICE=3`). Ticks are 100 Hz per core (1194 ticks/s on 12 cores). Take two samples and use
`(Δuser+Δsystem)/Δtotal`. Probe: `user 4.7% sys 2.8% idle 92.5%`. `host_processor_info(PROCESSOR_CPU_LOAD_INFO=2)`
gives the same per core (12 cpus, 23 us). Apple Silicon performance/efficiency core counts:
`sysctl hw.perflevel0.logicalcpu` (2 on this Mac).

Note: these ticks are not Mach absolute time; no `mach_timebase_info` needed here (the ratio of ticks per
second matched `100 * ncpu`).

**Memory.** `host_statistics64(HOST_VM_INFO64=4)` fills `vm_statistics64` (`mach/vm_statistics.h`). The
struct has grown over macOS releases; pass the oldest count (38 `natural_t`s, REV0) and read only the early
fields. Probe results (page size 16384 from `hw.pagesize`):

```
total 24.00G  app 9.39G  wired 2.55G  compressed 1.56G  => used 13.50G   cached files 4.83G  free 5.24G
```

- total: `sysctl hw.memsize` (`unix.SysctlUint64`).
- app memory: `(internal_page_count - purgeable_count) * pagesize`.
- wired: `wire_count * pagesize`. compressed: `compressor_page_count * pagesize`.
- used = app + wired + compressed. cached files = `(external_page_count + purgeable_count) * pagesize`.

I checked these against `vm_stat` output (wired 167049 pages, free 333919, compressor 102345 pages occupied) and
`top -l 1` (`PhysMem: 18G used (2610M wired, 1599M compressor), 5365M unused`). Note `top`'s "used" counts
all non-free pages (18G), including file cache, so it is a different number from app + wired + compressed
(13.5G). That "used = app + wired + compressed" is what Activity Monitor shows is a convention I know but
could not verify in a primary source. If the product shows "used", pick and label one definition.

**Swap.** `sysctl vm.swapusage` returns `struct xsw_usage` (32 bytes: `xsu_total`, `xsu_avail`, `xsu_used`,
`xsu_pagesize`, `xsu_encrypted`) (`sys/sysctl.h`). Probe: `total=4096M used=2910M`. Same as gopsutil's
`SwapMemory` (`mem/mem_darwin.go`).

**Memory pressure.** `sysctl kern.memorystatus_vm_pressure_level` returns an int (read-only,
`bsd/kern/kern_memorystatus_notify.c`). Values match `DISPATCH_MEMORYPRESSURE_*` in the SDK
(`dispatch/source.h`): `1` normal, `2` warn, `4` critical. Probe: `1`. `kern.memorystatus_level` is the percent of
memory considered free (probe: `81`) and is a smoother gauge for a pressure bar. For push updates, use
a `DISPATCH_SOURCE_TYPE_MEMORYPRESSURE` dispatch source; polling the sysctl at the refresh interval is
simpler and free.

## 9. Recommended data approach for blf

1. Add `golang.org/x/sys` and `github.com/ebitengine/purego`. Keep `CGO_ENABLED=0`. Put the darwin reader
   in `//go:build darwin` files and add a stub for other OSes so Linux CI and the release still build.
2. Each refresh:
   - `unix.SysctlKinfoProcSlice("kern.proc.all")` for pid, ppid, uid, start time, comm of every process.
   - For processes with your uid (or, for the first draft, everyone and ignore errors):
     `proc_pid_rusage(pid, RUSAGE_INFO_V0)` for CPU ticks and `ri_phys_footprint`.
   - CPU % from the delta of `(user+system) * numer/denom` against a monotonic wall delta, keyed on
     `(pid, start)`.
3. Once per new `(pid, start)`: `proc_pidpath`, argv via `KERN_PROCARGS2` (own only), responsible pid via
   `responsibility_get_pid_responsible_for_pid`, coalition id via `PROC_PIDCOALITIONINFO` (optional check).
4. Grouping into apps: responsible pid -> `.app` in that process's path, fallback to the process's own path
   and then its name. Resolve the private symbol at startup and degrade to path grouping if missing.
5. System bar: `host_statistics(HOST_CPU_LOAD_INFO)`, `host_statistics64`, `vm.swapusage`,
   `kern.memorystatus_vm_pressure_level`.
6. Show footprint as "Memory". Show other users' processes by name only, or not at all, and show the
   unaccounted CPU (system total minus the sum of readable processes) as "other users / system".

Risks to track: the private `responsibility_*` symbol, the numeric `KERN_PROCARGS2` and
`PROC_PIDCOALITIONINFO` constants (stable in XNU for years but not in public headers), and purego's
`CGO_ENABLED=0` darwin path (tier 1, but a third-party runtime trick: pin the version and keep a test that
the darwin build links and returns this process's own footprint).

## 10. What I could not prove

- That Activity Monitor's "Memory" column is exactly `phys_footprint` (closed source). Indirect support only:
  `top`'s `mem` column documented as footprint and equal to `ri_phys_footprint` in the probe.
- That Activity Monitor's "Memory Used" is app + wired + compressed (convention; no primary source).
- What Activity Monitor uses to group helpers under their app. Not public.
- Behavior as root. All results are from a normal user; the root-can-read claim comes from `proc_security_policy`
  in XNU, not from a probe. No sudo was used.
- `kill` error behavior for other users' processes (not probed on purpose). Standard `kill(2)` gives `EPERM`.
- Behavior on Intel Macs and macOS versions other than this one. The unit conversion and flavors are in
  the SDK headers for both, but only arm64 on this macOS was run. The cross-compile to `darwin/amd64` was
  built, not run.
- Release builds were not run through goreleaser; the cgo cross-compile failure is demonstrated by the
  `CGO_ENABLED=0` exclusion and the `go env` defaults, not by a goreleaser run on Ubuntu.
