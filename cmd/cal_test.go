package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestRunCalPrintsPreviousCurrentAndNextMonth(t *testing.T) {
	out := &strings.Builder{}
	err := runCal([]string{"2026-04-08"}, deps{
		stdout: out,
		now: func() time.Time {
			return time.Date(2026, time.April, 8, 12, 0, 0, 0, time.Local)
		},
	})
	if err != nil {
		t.Fatalf("runCal returned error: %v", err)
	}

	want := `
         Mar 2026          
Wk Sun Mon Tue Wed Thu Fri Sat
10   1   2   3   4   5   6   7
11   8   9  10  11  12  13  14
12  15  16  17  18  19  20  21
13  22  23  24  25  26  27  28
14  29  30  31

         Apr 2026          
Wk Sun Mon Tue Wed Thu Fri Sat
14               1   2   3   4
15   5   6   7   8   9  10  11
16  12  13  14  15  16  17  18
17  19  20  21  22  23  24  25
18  26  27  28  29  30

         May 2026          
Wk Sun Mon Tue Wed Thu Fri Sat
18                       1   2
19   3   4   5   6   7   8   9
20  10  11  12  13  14  15  16
21  17  18  19  20  21  22  23
22  24  25  26  27  28  29  30
23  31

`
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestRunCalDefaultsToToday(t *testing.T) {
	out := &strings.Builder{}
	err := runCal(nil, deps{
		stdout: out,
		now: func() time.Time {
			return time.Date(2026, time.January, 1, 12, 0, 0, 0, time.Local)
		},
	})
	if err != nil {
		t.Fatalf("runCal returned error: %v", err)
	}

	if !strings.Contains(out.String(), "         Jan 2026          ") {
		t.Fatalf("expected Jan 2026 in output: %q", out.String())
	}
}

func TestRunCalErrorsOnInvalidDate(t *testing.T) {
	err := runCal([]string{"invalid"}, deps{stdout: &strings.Builder{}})
	if err == nil || err.Error() != `invalid calendar date "invalid"` {
		t.Fatalf("error = %v", err)
	}
}

func TestRunCalUsage(t *testing.T) {
	err := runCal([]string{"2026-04-08", "extra"}, deps{stdout: &strings.Builder{}})
	if err == nil || err.Error() != "usage: blf cal [date]" {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderCalMonthCanStyleTodayAndWeekends(t *testing.T) {
	month := createCalMonth(time.Date(2026, time.April, 8, 12, 0, 0, 0, time.Local))
	today := time.Date(2026, time.April, 8, 0, 0, 0, 0, time.Local)
	out := renderCalMonth(month, today, true)

	if !strings.Contains(out, calMonthTitleStyle.Render("         Apr 2026          ")) {
		t.Fatalf("expected styled month title: %q", out)
	}
	if !strings.Contains(out, calTodayStyle.Render("  8")) {
		t.Fatalf("expected styled today: %q", out)
	}
	if !strings.Contains(out, calMutedStyle.Render(" 10")) {
		t.Fatalf("expected styled Friday/Saturday: %q", out)
	}
}
