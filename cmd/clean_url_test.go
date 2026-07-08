package cmd

import (
	"bytes"
	"testing"
)

func TestRunCleanURL(t *testing.T) {
	t.Run("prints cleaned url for argument", func(t *testing.T) {
		var out bytes.Buffer
		d := deps{stdout: &out}

		if err := runCleanURL("https://example.com/page?utm_source=x&id=1", false, d); err != nil {
			t.Fatalf("runCleanURL: %v", err)
		}

		want := "https://example.com/page?id=1\n"
		if out.String() != want {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	})
}
