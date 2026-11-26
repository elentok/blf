package main

import "fmt"

func sum() {
	rs, _ := readAll(false)
	var f, r uint64
	for _, x := range rs {
		f += x.foot
		r += x.rss
	}
	fmt.Printf("readable procs %d: sum footprint %.2fG sum rss %.2fG\n", len(rs), float64(f)/(1<<30), float64(r)/(1<<30))
}
