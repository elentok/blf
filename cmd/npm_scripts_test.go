package cmd

import (
	"strings"
	"testing"
)

func TestRunNPMScriptsFormatsAlignedOutput(t *testing.T) {
	out := &strings.Builder{}
	err := runNPMScripts(deps{
		stdout:     out,
		fileExists: func(string) (bool, error) { return true, nil },
		readFile: func(string) ([]byte, error) {
			return []byte(`{"scripts":{"dev":"vite","lint:fix":"eslint --fix ."}}`), nil
		},
	})
	if err != nil {
		t.Fatalf("runNPMScripts returned error: %v", err)
	}

	got := out.String()
	want := "\x1b[32mdev     \x1b[0m  - vite\n\x1b[32mlint:fix\x1b[0m  - eslint --fix .\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunNPMScriptsErrorsWhenPackageJSONMissing(t *testing.T) {
	err := runNPMScripts(deps{
		stdout:     &strings.Builder{},
		fileExists: func(string) (bool, error) { return false, nil },
		readFile:   func(string) ([]byte, error) { return nil, nil },
	})
	if err == nil || err.Error() != "No package.json file" {
		t.Fatalf("error = %v", err)
	}
}

func TestRunNPMScriptsErrorsWhenScriptsMissing(t *testing.T) {
	err := runNPMScripts(deps{
		stdout:     &strings.Builder{},
		fileExists: func(string) (bool, error) { return true, nil },
		readFile: func(string) ([]byte, error) {
			return []byte(`{"name":"demo"}`), nil
		},
	})
	if err == nil || err.Error() != "package.json file has no scripts" {
		t.Fatalf("error = %v", err)
	}
}

func TestParseNPMScriptsPreservesOrder(t *testing.T) {
	scripts, err := parseNPMScripts([]byte(`{"scripts":{"b":"two","a":"one","c":"three"}}`))
	if err != nil {
		t.Fatalf("parseNPMScripts returned error: %v", err)
	}

	var names []string
	for _, script := range scripts {
		names = append(names, script.Name)
	}
	if strings.Join(names, ",") != "b,a,c" {
		t.Fatalf("script order = %v", names)
	}
}
