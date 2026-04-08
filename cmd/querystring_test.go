package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestRunQueryStringPrettyPrintsOrderedParams(t *testing.T) {
	out := &strings.Builder{}
	err := runQueryString([]string{"https://example.com/path?a=1&a=2&b=hello+world"}, deps{stdout: out})
	if err != nil {
		t.Fatalf("runQueryString returned error: %v", err)
	}

	want := "- a: 1\n- a: 2\n- b: hello world\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestRunQueryStringPrintsAllValuesForKey(t *testing.T) {
	out := &strings.Builder{}
	err := runQueryString([]string{"a=1&a=2&a=", "a"}, deps{stdout: out})
	if err != nil {
		t.Fatalf("runQueryString returned error: %v", err)
	}

	want := "[ \"1\", \"2\", \"\" ]\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestRunQueryStringPrintsEmptyArrayForMissingKey(t *testing.T) {
	out := &strings.Builder{}
	err := runQueryString([]string{"a=1", "missing"}, deps{stdout: out})
	if err != nil {
		t.Fatalf("runQueryString returned error: %v", err)
	}

	if out.String() != "[]\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunQueryStringPrintsSingleValueForKey(t *testing.T) {
	out := &strings.Builder{}
	err := runQueryString([]string{"a=1&b=hello+world", "b"}, deps{stdout: out})
	if err != nil {
		t.Fatalf("runQueryString returned error: %v", err)
	}

	if out.String() != "hello world\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunQueryStringReadsStdinForDash(t *testing.T) {
	out := &strings.Builder{}
	err := runQueryString([]string{"-", "b"}, deps{
		stdout: out,
		stdin:  strings.NewReader("a=1&b=from-stdin"),
	})
	if err != nil {
		t.Fatalf("runQueryString returned error: %v", err)
	}

	if out.String() != "from-stdin\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestParseQueryStringMatchesURLSearchParamsCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []queryParam
	}{
		{
			name:  "leading question mark",
			input: "?a=1&b=2",
			want:  []queryParam{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}},
		},
		{
			name:  "no query in full URL",
			input: "https://example.com/path",
			want:  nil,
		},
		{
			name:  "invalid percent escapes are preserved",
			input: "a=%ZZ&b=valid%20value&c=bad%2G+plus",
			want:  []queryParam{{Key: "a", Value: "%ZZ"}, {Key: "b", Value: "valid value"}, {Key: "c", Value: "bad%2G plus"}},
		},
		{
			name:  "semicolon remains part of value",
			input: "a=1;b=2",
			want:  []queryParam{{Key: "a", Value: "1;b=2"}},
		},
		{
			name:  "empty segments ignored and flag is empty",
			input: "a=1&&b=2&flag",
			want:  []queryParam{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}, {Key: "flag", Value: ""}},
		},
		{
			name:  "split only on first equals",
			input: "a=b=c&=value&encoded%20key=v+1",
			want:  []queryParam{{Key: "a", Value: "b=c"}, {Key: "", Value: "value"}, {Key: "encoded key", Value: "v 1"}},
		},
		{
			name:  "full URL with fragment",
			input: "https://example.com/path?a=1&b=2#frag",
			want:  []queryParam{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQueryString(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseQueryString(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunQueryStringUsage(t *testing.T) {
	err := runQueryString(nil, deps{stdout: &strings.Builder{}, stdin: strings.NewReader("")})
	if err == nil || err.Error() != "usage: blf querystring <querystring|-> [key]" {
		t.Fatalf("error = %v", err)
	}
}
