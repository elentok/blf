package cmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
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

func runCleanURL(rawURL string, useClipboard bool, d deps) error {
	if useClipboard {
		clipboardText, err := d.readClipboard()
		if err != nil {
			return fmt.Errorf("read clipboard: %w", err)
		}
		rawURL = clipboardText
	}

	cleaned := cleanURL(rawURL)

	if useClipboard {
		if err := d.copyText(cleaned); err != nil {
			return fmt.Errorf("write clipboard: %w", err)
		}
	}

	fmt.Fprintln(d.stdout, cleaned)
	return nil
}

func cleanURL(rawURL string) string {
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

func newCleanURLCmd(d deps) *cobra.Command {
	var useClipboard bool

	cmd := &cobra.Command{
		Use:   "clean-url [url]",
		Short: "Unwrap redirect wrappers and strip tracking params from a URL",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case useClipboard && len(args) == 1:
				return fmt.Errorf("specify either <url> or --clipboard, not both")
			case !useClipboard && len(args) == 0:
				return fmt.Errorf("specify either <url> or --clipboard")
			}

			rawURL := ""
			if len(args) == 1 {
				rawURL = args[0]
			}
			return runCleanURL(rawURL, useClipboard, d)
		},
	}
	cmd.Flags().BoolVar(&useClipboard, "clipboard", false, "Read the URL from the clipboard and write the cleaned URL back")
	return cmd
}
