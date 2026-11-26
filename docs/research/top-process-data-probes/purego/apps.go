package main

import (
	"fmt"
	"sort"
	"strings"
)

func outerApp(p string) string {
	i := strings.Index(p, ".app/")
	if i < 0 {
		return ""
	}
	s := p[:i+4]
	if j := strings.LastIndex(s, "/"); j >= 0 {
		return s[j+1:]
	}
	return s
}

func apps() {
	rs, _ := readAll(true)
	byPid := map[int32]rec{}
	for _, r := range rs {
		byPid[r.pid] = r
	}
	type g struct {
		n    int
		foot uint64
		rss  uint64
	}
	byResp, byCoal, byPath := map[string]*g{}, map[uint64]*g{}, map[string]*g{}
	name := func(pid int32) string {
		if r, ok := byPid[pid]; ok {
			if a := outerApp(r.path); a != "" {
				return a
			}
			return r.name
		}
		return fmt.Sprintf("pid%d(unreadable)", pid)
	}
	coalName := map[uint64]string{}
	for _, r := range rs {
		k := name(r.resp)
		if byResp[k] == nil {
			byResp[k] = &g{}
		}
		byResp[k].n++
		byResp[k].foot += r.foot
		byResp[k].rss += r.rss
		if byCoal[r.coal] == nil {
			byCoal[r.coal] = &g{}
		}
		byCoal[r.coal].n++
		byCoal[r.coal].foot += r.foot
		if r.pid == int32(r.resp) || coalName[r.coal] == "" {
			coalName[r.coal] = name(r.pid)
		}
		pk := outerApp(r.path)
		if pk == "" {
			pk = "(no .app) " + r.name
		}
		if byPath[pk] == nil {
			byPath[pk] = &g{}
		}
		byPath[pk].n++
		byPath[pk].foot += r.foot
	}
	top := func(title string, m map[string]*g) {
		type kv struct {
			k string
			v *g
		}
		var l []kv
		for k, v := range m {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v.foot > l[j].v.foot })
		fmt.Println("==", title, "groups:", len(m))
		for _, x := range l[:min(8, len(l))] {
			fmt.Printf("  %-40s n=%-3d footprint=%.0fMB rss=%.0fMB\n", x.k, x.v.n, float64(x.v.foot)/1048576, float64(x.v.rss)/1048576)
		}
	}
	top("by responsible pid", byResp)
	top("by .app in own path", byPath)
	fmt.Println("== coalitions (resource coalition id) groups:", len(byCoal))
	// disagreements: process whose own .app differs from its responsible's app
	cnt := 0
	for _, r := range rs {
		own := outerApp(r.path)
		rp := name(r.resp)
		if own != rp && cnt < 12 {
			cnt++
			fmt.Printf("  differs: pid %d %q own-app=%q responsible=%q (pid %d) coal=%d\n", r.pid, r.name, own, rp, r.resp, r.coal)
		}
	}
}
