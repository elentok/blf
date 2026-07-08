// Package cleanurl unwraps redirect wrappers (e.g. Google's /url) and strips
// tracking query params from a URL.
package cleanurl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/elentok/blf/internal/platform"
)

const maxUnwrapIterations = 10

var googleURLHostPattern = regexp.MustCompile(`^(www\.)?google\.[a-z.]+$`)

type redirectWrapper struct {
	matchesHost func(host string) bool
	matchesPath func(path string) bool
	params      []string
}

var redirectWrappers = []redirectWrapper{
	{
		matchesHost: googleURLHostPattern.MatchString,
		matchesPath: func(path string) bool { return path == "/url" },
		params:      []string{"url", "q"},
	},
}

var trackingParams = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"utm_term":     {},
	"utm_content":  {},
	"utm_id":       {},
	"utm_name":     {},
	"gclid":        {},
	"fbclid":       {},
	"igshid":       {},
	"mc_eid":       {},
	"mc_cid":       {},
	"ref":          {},
	"ref_src":      {},
	"ref_url":      {},
	"_hsenc":       {},
	"_hsmi":        {},
	"vero_id":      {},
	"yclid":        {},
	"msclkid":      {},
	"twclid":       {},
	"si":           {},
	"spm":          {},
	"src":          {},
}

// readClipboard and copyText are injectable seams for testing RunClipboard.
var (
	readClipboard = platform.ReadClipboardText
	copyText      = platform.CopyText
)

// CleanURL unwraps redirect wrappers and strips tracking params from a URL.
func CleanURL(rawURL string) string {
	current := rawURL
	for range maxUnwrapIterations {
		unwrapped, ok := unwrapRedirect(current)
		if !ok {
			break
		}
		current = unwrapped
	}

	return stripTrackingParams(current)
}

// RunClipboard reads a URL from the clipboard, cleans it, and writes the
// cleaned URL back to the clipboard.
func RunClipboard() error {
	rawURL, err := readClipboard()
	if err != nil {
		return fmt.Errorf("read clipboard: %w", err)
	}

	cleaned := CleanURL(rawURL)

	if err := copyText(cleaned); err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}

	return nil
}

func unwrapRedirect(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", false
	}

	for _, wrapper := range redirectWrappers {
		if !wrapper.matchesHost(parsed.Host) || !wrapper.matchesPath(parsed.Path) {
			continue
		}
		query := parsed.Query()
		for _, param := range wrapper.params {
			if embedded := query.Get(param); embedded != "" {
				return embedded, true
			}
		}
	}

	return "", false
}

func stripTrackingParams(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}

	segments := strings.Split(parsed.RawQuery, "&")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		key, _, _ := strings.Cut(segment, "=")
		decodedKey, err := url.QueryUnescape(key)
		if err == nil {
			key = decodedKey
		}
		if _, tracked := trackingParams[key]; tracked {
			continue
		}
		kept = append(kept, segment)
	}

	parsed.RawQuery = strings.Join(kept, "&")
	return strings.TrimSuffix(parsed.String(), "?")
}
