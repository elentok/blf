package cleanurl

import (
	"errors"
	"testing"
)

func TestCleanURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "google redirect wrapper",
			input: "https://www.google.com/url?sa=t&source=web&rct=j&opi=89978449&url=https://en.wikipedia.org/wiki/Hello&ved=2ahUKEwiggueRy_WUAxV787sIHd97PAEQFnoECCIQAQ&usg=AOvVaw0zbH-a0GaFHdJIHKeFAVgu",
			want:  "https://en.wikipedia.org/wiki/Hello",
		},
		{
			name:  "google redirect wrapper with q param",
			input: "https://www.google.com/url?q=https://en.wikipedia.org/wiki/Hello&source=gmail&ust=123&usg=abc",
			want:  "https://en.wikipedia.org/wiki/Hello",
		},
		{
			name:  "google redirect wrapper with country tld",
			input: "https://www.google.co.uk/url?sa=t&url=https://example.com/page&ved=abc",
			want:  "https://example.com/page",
		},
		{
			name:  "nested google wrappers",
			input: "https://www.google.com/url?sa=t&url=" + escapeURL("https://www.google.com/url?sa=t&url=https://example.com/page&ved=def") + "&ved=abc",
			want:  "https://example.com/page",
		},
		{
			name:  "strips tracking params",
			input: "https://example.com/page?utm_source=newsletter&id=42&fbclid=xyz",
			want:  "https://example.com/page?id=42",
		},
		{
			name:  "unwrap then strip tracking params on the embedded url",
			input: "https://www.google.com/url?sa=t&url=" + escapeURL("https://example.com/page?utm_source=newsletter&id=42") + "&ved=abc",
			want:  "https://example.com/page?id=42",
		},
		{
			name:  "passthrough for plain text",
			input: "not a url",
			want:  "not a url",
		},
		{
			name:  "passthrough for already clean url",
			input: "https://example.com/page?id=42",
			want:  "https://example.com/page?id=42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanURL(tt.input)
			if got != tt.want {
				t.Errorf("CleanURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunClipboard(t *testing.T) {
	t.Run("reads from clipboard, cleans, and writes back", func(t *testing.T) {
		origReadClipboard, origCopyText := readClipboard, copyText
		defer func() { readClipboard, copyText = origReadClipboard, origCopyText }()

		var written string
		readClipboard = func() (string, error) {
			return "https://example.com/page?utm_source=x&id=1", nil
		}
		copyText = func(text string) error {
			written = text
			return nil
		}

		if err := RunClipboard(); err != nil {
			t.Fatalf("RunClipboard: %v", err)
		}

		want := "https://example.com/page?id=1"
		if written != want {
			t.Errorf("clipboard written = %q, want %q", written, want)
		}
	})

	t.Run("returns error when clipboard read fails", func(t *testing.T) {
		origReadClipboard, origCopyText := readClipboard, copyText
		defer func() { readClipboard, copyText = origReadClipboard, origCopyText }()

		readClipboard = func() (string, error) {
			return "", errors.New("boom")
		}
		copyText = func(string) error {
			t.Fatal("copyText should not be called when read fails")
			return nil
		}

		if err := RunClipboard(); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}

func escapeURL(s string) string {
	escaped := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ':', '/', '?', '&', '=':
			escaped = append(escaped, '%', hexDigit(c>>4), hexDigit(c&0xf))
		default:
			escaped = append(escaped, c)
		}
	}
	return string(escaped)
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'A' + (b - 10)
}
