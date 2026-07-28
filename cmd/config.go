package cmd

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/editfile"
	"github.com/spf13/cobra"
)

func newConfigCmd(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the blf config file",
	}

	cmd.AddCommand(newConfigEditCmd(d))
	cmd.AddCommand(newConfigEditSnippetsCmd(d))
	return cmd
}

func newConfigEditCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the blf config file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEdit(d)
		},
	}
}

const configSchemaPragma = "#:schema ./" + config.ConfigSchemaFilename

func runConfigEdit(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("config edit: get home dir: %w", err)
	}

	path := config.XDGConfigPath(homeDir)
	dir := filepath.Dir(path)

	// Always (re)write the schema files and taplo.toml so taplo/Even Better
	// TOML stay in sync with the current version of blf, even for an
	// already-existing config.toml.
	if err := writeSchemaAndTaploToml(dir, config.ConfigSchemaFilename, config.ConfigSchemaJSON(), d); err != nil {
		return fmt.Errorf("config edit: write schema: %w", err)
	}

	exists, err := d.fileExists(path)
	if err != nil {
		return fmt.Errorf("config edit: check %q: %w", path, err)
	}
	if !exists {
		if err := seedDefaultConfig(path, d); err != nil {
			return fmt.Errorf("config edit: seed %q: %w", path, err)
		}
	} else {
		if err := ensureSchemaPragma(path, configSchemaPragma, d); err != nil {
			return fmt.Errorf("config edit: add schema pragma %q: %w", path, err)
		}
	}

	return editfile.Open(path, editfile.Deps{
		Stdin:     d.stdin,
		Stdout:    d.stdout,
		Stderr:    d.stderr,
		LookupEnv: d.lookupEnv,
	})
}

func newConfigEditSnippetsCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "edit-snippets",
		Short: "Edit the blf snippets file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEditSnippets(d)
		},
	}
}

const snippetsSchemaPragma = "#:schema ./" + config.SnippetsSchemaFilename

const exampleSnippets = snippetsSchemaPragma + `
# Snippets are copied to the clipboard when selected in ` + "`blf launcher`" + `.
#
# [[snippet]]
# name  = "shipping"
# value = """
# David Elentok
# 123 Main St
# Springfield, IL 62704
# """
`

func runConfigEditSnippets(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("config edit-snippets: get home dir: %w", err)
	}

	path := config.XDGSnippetsPath(homeDir)
	dir := filepath.Dir(path)

	// Always (re)write the schema files and taplo.toml so taplo/Even Better
	// TOML stay in sync with the current version of blf, even for an
	// already-existing snippets.toml.
	if err := writeSchemaAndTaploToml(dir, config.SnippetsSchemaFilename, config.SnippetsSchemaJSON(), d); err != nil {
		return fmt.Errorf("config edit-snippets: write schema: %w", err)
	}

	exists, err := d.fileExists(path)
	if err != nil {
		return fmt.Errorf("config edit-snippets: check %q: %w", path, err)
	}
	if !exists {
		if err := seedExampleSnippets(path, d); err != nil {
			return fmt.Errorf("config edit-snippets: seed %q: %w", path, err)
		}
	} else {
		if err := ensureSchemaPragma(path, snippetsSchemaPragma, d); err != nil {
			return fmt.Errorf("config edit-snippets: add schema pragma %q: %w", path, err)
		}
	}

	return editfile.Open(path, editfile.Deps{
		Stdin:     d.stdin,
		Stdout:    d.stdout,
		Stderr:    d.stderr,
		LookupEnv: d.lookupEnv,
	})
}

func seedExampleSnippets(path string, d deps) error {
	if err := d.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return d.writeFile(path, []byte(exampleSnippets), 0o644)
}

func seedDefaultConfig(path string, d deps) error {
	if err := d.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(configSchemaPragma + "\n")
	if err := toml.NewEncoder(&buf).Encode(config.DefaultConfig()); err != nil {
		return fmt.Errorf("encode defaults: %w", err)
	}

	return d.writeFile(path, buf.Bytes(), 0o644)
}

// ensureSchemaPragma prepends pragma to an existing file that predates the
// schema feature, or that had the pragma removed. A file that already starts
// with it is left untouched.
func ensureSchemaPragma(path, pragma string, d deps) error {
	data, err := d.readFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}

	if bytes.HasPrefix(data, []byte(pragma)) {
		return nil
	}

	updated := append([]byte(pragma+"\n"), data...)
	return d.writeFile(path, updated, 0o644)
}

// writeSchemaAndTaploToml (re)writes schemaFilename alongside schemaJSON,
// plus the shared taplo.toml associating both config.toml and snippets.toml
// with their schemas by filename (belt-and-suspenders alongside each file's
// own `#:schema` pragma).
func writeSchemaAndTaploToml(dir, schemaFilename string, schemaJSON []byte, d deps) error {
	if err := d.mkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	schemaPath := filepath.Join(dir, schemaFilename)
	if err := d.writeFile(schemaPath, schemaJSON, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", schemaPath, err)
	}

	taploPath := filepath.Join(dir, config.TaploTomlFilename)
	return d.writeFile(taploPath, config.TaploTomlContent(), 0o644)
}
