package cmd

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var urlSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

type queryParam struct {
	Key   string
	Value string
}

func runQueryString(args []string, d deps) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: blf querystring <querystring|-> [key]")
	}

	input := args[0]
	if input == "-" {
		data, err := io.ReadAll(d.stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		input = string(data)
	}

	params := parseQueryString(input)
	if len(args) == 1 {
		for _, param := range params {
			fmt.Fprintf(d.stdout, "- %s: %s\n", param.Key, param.Value)
		}
		return nil
	}

	fmt.Fprintln(d.stdout, formatQueryValues(params, args[1]))
	return nil
}

func parseQueryString(input string) []queryParam {
	raw := extractRawQuery(input)
	raw = strings.TrimPrefix(raw, "?")
	if raw == "" {
		return nil
	}

	segments := strings.Split(raw, "&")
	params := make([]queryParam, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		key, value, _ := strings.Cut(segment, "=")
		params = append(params, queryParam{
			Key:   queryUnescapeLenient(key),
			Value: queryUnescapeLenient(value),
		})
	}
	return params
}

func extractRawQuery(input string) string {
	if !urlSchemePattern.MatchString(input) {
		return input
	}
	if idx := strings.IndexByte(input, '?'); idx >= 0 {
		raw := input[idx+1:]
		if hashIdx := strings.IndexByte(raw, '#'); hashIdx >= 0 {
			raw = raw[:hashIdx]
		}
		return raw
	}
	return ""
}

func queryUnescapeLenient(s string) string {
	var out strings.Builder
	out.Grow(len(s))

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '+':
			out.WriteByte(' ')
		case '%':
			if i+2 < len(s) {
				hi, okHi := fromHex(s[i+1])
				lo, okLo := fromHex(s[i+2])
				if okHi && okLo {
					out.WriteByte(hi<<4 | lo)
					i += 2
					continue
				}
			}
			out.WriteByte('%')
		default:
			out.WriteByte(s[i])
		}
	}

	return out.String()
}

func fromHex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func formatQueryValues(params []queryParam, key string) string {
	values := make([]string, 0)
	for _, param := range params {
		if param.Key == key {
			values = append(values, param.Value)
		}
	}
	if len(values) == 0 {
		return "[]"
	}
	if len(values) == 1 {
		return values[0]
	}

	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[ " + strings.Join(quoted, ", ") + " ]"
}
