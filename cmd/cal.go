package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

var (
	calMonthTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	calMutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	calTodayStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
)

type calMonth struct {
	start time.Time
	weeks []calWeek
}

type calWeek struct {
	number int
	days   []*time.Time
}

func runCal(args []string, d deps) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: blf cal [date]")
	}

	now := time.Now
	if d.now != nil {
		now = d.now
	}

	date := now()
	if len(args) == 1 {
		parsed, err := parseCalDate(args[0], date.Location())
		if err != nil {
			return err
		}
		date = parsed
	}

	fmt.Fprintln(d.stdout)
	month := createCalMonth(date)
	color := shouldColorCalOutput(d.stdout)
	for _, m := range []calMonth{addCalMonths(month, -1), month, addCalMonths(month, 1)} {
		printCalMonth(d.stdout, m, dateOnly(now()), color)
	}
	return nil
}

func parseCalDate(s string, loc *time.Location) (time.Time, error) {
	layouts := []string{"2006-01-02", "2006-01", "01/02/2006", "1/2/2006"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid calendar date %q", s)
}

func createCalMonth(date time.Time) calMonth {
	start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	return calMonth{start: start, weeks: buildCalWeeks(start)}
}

func addCalMonths(month calMonth, delta int) calMonth {
	return createCalMonth(month.start.AddDate(0, delta, 0))
}

func buildCalWeeks(start time.Time) []calWeek {
	week := calWeek{number: usWeekNumber(start.AddDate(0, 0, 2))}
	date := startOfCalWeek(start)
	weeks := make([]calWeek, 0, 6)

	for date.Before(start) {
		week.days = append(week.days, nil)
		date = date.AddDate(0, 0, 1)
	}

	for date.Month() == start.Month() {
		d := date
		week.days = append(week.days, &d)

		if date.Weekday() == time.Saturday {
			weeks = append(weeks, week)
			week = calWeek{number: week.number + 1}
		}

		date = date.AddDate(0, 0, 1)
	}

	if len(week.days) > 0 {
		weeks = append(weeks, week)
	}

	return weeks
}

func printCalMonth(w io.Writer, month calMonth, today time.Time, color bool) {
	fmt.Fprint(w, renderCalMonth(month, today, color))
}

func renderCalMonth(month calMonth, today time.Time, color bool) string {
	var out strings.Builder
	title := centerCalText(month.start.Format("Jan 2006"), 27)
	if color {
		title = calMonthTitleStyle.Render(title)
	}
	fmt.Fprintln(&out, title)
	fmt.Fprintln(&out, "Wk Sun Mon Tue Wed Thu Fri Sat")
	for _, week := range month.weeks {
		fmt.Fprintln(&out, renderCalWeek(week, today, color))
	}
	fmt.Fprintln(&out)
	return out.String()
}

func renderCalWeek(week calWeek, today time.Time, color bool) string {
	parts := []string{styleCalWeekNumber(strconv.Itoa(week.number), color)}
	for _, day := range week.days {
		if day == nil {
			parts = append(parts, "   ")
			continue
		}
		parts = append(parts, formatCalDay(*day, today, color))
	}
	return strings.Join(parts, " ")
}

func styleCalWeekNumber(text string, color bool) string {
	padded := fmt.Sprintf("%2s", text)
	if !color {
		return padded
	}
	return calMutedStyle.Render(padded)
}

func formatCalDay(day time.Time, today time.Time, color bool) string {
	text := fmt.Sprintf("%3d", day.Day())
	if sameCalDay(day, today) {
		if color {
			return calTodayStyle.Render(text)
		}
		return text
	}
	if day.Weekday() == time.Friday || day.Weekday() == time.Saturday {
		if color {
			return calMutedStyle.Render(text)
		}
		return text
	}
	return text
}

func startOfCalWeek(date time.Time) time.Time {
	return date.AddDate(0, 0, -int(date.Weekday()))
}

func usWeekNumber(date time.Time) int {
	jan1 := time.Date(date.Year(), time.January, 1, 0, 0, 0, 0, date.Location())
	week1Start := startOfCalWeek(jan1)
	startUTC := time.Date(week1Start.Year(), week1Start.Month(), week1Start.Day(), 0, 0, 0, 0, time.UTC)
	dateUTC := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	return int(dateUTC.Sub(startUTC)/(24*time.Hour))/7 + 1
}

func centerCalText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	left := (width - len(text)) / 2
	right := width - len(text) - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}

func sameCalDay(a, b time.Time) bool {
	aa := dateOnly(a)
	bb := dateOnly(b)
	return aa.Equal(bb)
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func shouldColorCalOutput(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
