package tmuxlinks

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxMenuItems = 30

const (
	ModeOpen = "open"
	ModeCopy = "copy"
)

func BuildDisplayMenuArgs(mode string, urls []string) ([]string, error) {
	title, err := menuTitle(mode)
	if err != nil {
		return nil, err
	}

	if len(urls) > MaxMenuItems {
		urls = urls[:MaxMenuItems]
	}

	args := []string{"display-menu", "-T", title, "-x", "C", "-y", "C"}
	for _, u := range urls {
		label := truncateLabel(u, 90)
		shellCmd := fmt.Sprintf("blf %s %s", mode, shellQuote(u))
		tmuxCmd := "run-shell " + strconv.Quote(shellCmd)
		args = append(args, label, "", tmuxCmd)
	}

	return args, nil
}

func menuTitle(mode string) (string, error) {
	switch mode {
	case ModeOpen:
		return "Open URL", nil
	case ModeCopy:
		return "Copy URL", nil
	default:
		return "", fmt.Errorf("invalid mode %q", mode)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func truncateLabel(s string, maxRunes int) string {
	if maxRunes < 4 {
		maxRunes = 4
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxRunes-3]) + "..."
}
