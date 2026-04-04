package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const green = "\x1b[32m"
const reset = "\x1b[0m"

type npmScript struct {
	Name    string
	Command string
}

func runNPMScripts(d deps) error {
	exists, err := d.fileExists("package.json")
	if err != nil {
		return fmt.Errorf("check package.json: %w", err)
	}
	if !exists {
		return fmt.Errorf("No package.json file")
	}

	content, err := d.readFile("package.json")
	if err != nil {
		return fmt.Errorf("read package.json: %w", err)
	}

	scripts, err := parseNPMScripts(content)
	if err != nil {
		return err
	}

	width := 0
	for _, script := range scripts {
		if len(script.Name) > width {
			width = len(script.Name)
		}
	}

	for _, script := range scripts {
		prettyName := green + padRight(script.Name, width) + reset
		fmt.Fprintf(d.stdout, "%s  - %s\n", prettyName, script.Command)
	}

	return nil
}

func parseNPMScripts(content []byte) ([]npmScript, error) {
	var pkg struct {
		Scripts json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	if len(pkg.Scripts) == 0 || bytes.Equal(bytes.TrimSpace(pkg.Scripts), []byte("null")) {
		return nil, fmt.Errorf("package.json file has no scripts")
	}

	dec := json.NewDecoder(bytes.NewReader(pkg.Scripts))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse package.json scripts: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("parse package.json scripts: expected object")
	}

	var scripts []npmScript
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parse package.json scripts: %w", err)
		}
		name, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("parse package.json scripts: invalid script name")
		}

		var command string
		if err := dec.Decode(&command); err != nil {
			return nil, fmt.Errorf("parse package.json scripts: %w", err)
		}
		scripts = append(scripts, npmScript{Name: name, Command: command})
	}

	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("parse package.json scripts: %w", err)
	}

	return scripts, nil
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
