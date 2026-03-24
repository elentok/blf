package tmuxlinks

import (
	"net/url"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https?://[^\s<>"]+`)

func ExtractURLs(text string) []string {
	matches := urlPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	result := make([]string, 0, len(matches))

	for _, m := range matches {
		clean := trimTrailingPunctuation(m)
		if clean == "" {
			continue
		}
		u, err := url.Parse(clean)
		if err != nil {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		if u.Host == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func trimTrailingPunctuation(s string) string {
	return strings.TrimRight(s, ").,;:]}")
}
