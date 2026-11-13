// Package plugins owns the recommended-plugins catalog: parsing plugins.toml,
// rendering the README section it generates, and comparing the catalog with the
// plugins a machine has installed.
package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the catalog file at the repository root.
const FileName = "plugins.toml"

// Entry is one recommended plugin. Repo is "owner/repo"; Optional marks the
// entries sync skips unless --optional is passed; Ref pins the version.
type Entry struct {
	Repo     string
	Reason   string
	Optional bool
	Ref      string
}

// Catalog is every recommended plugin, in file order.
type Catalog struct {
	Entries []Entry
}

// Parse reads the catalog the way the binary needs it: one [[plugin]] table per
// entry, with repo and reason required and optional and ref optional. A malformed
// entry, an unknown key or a duplicate repo is an error, so a typo fails here
// instead of silently shrinking the catalog.
func Parse(data []byte) (Catalog, error) {
	var catalog Catalog
	var current *Entry
	entryIndex := 0
	seen := map[string]bool{}

	finish := func(entry *Entry) error {
		if entry.Repo == "" {
			return fmt.Errorf("%s: entry %d declares no repo", FileName, entryIndex)
		}
		if strings.Count(entry.Repo, "/") != 1 || strings.ContainsAny(entry.Repo, " \t") {
			return fmt.Errorf("%s: entry %d: %q is not an owner/repo pair", FileName, entryIndex, entry.Repo)
		}
		owner, name, _ := strings.Cut(entry.Repo, "/")
		if owner == "" || name == "" {
			return fmt.Errorf("%s: entry %d: %q is not an owner/repo pair", FileName, entryIndex, entry.Repo)
		}
		if entry.Reason == "" {
			return fmt.Errorf("%s: entry %d (%s) declares no reason", FileName, entryIndex, entry.Repo)
		}
		if seen[entry.Repo] {
			return fmt.Errorf("%s: entry %d: %s is declared twice", FileName, entryIndex, entry.Repo)
		}
		seen[entry.Repo] = true
		catalog.Entries = append(catalog.Entries, *entry)
		return nil
	}

	for index, raw := range strings.Split(string(data), "\n") {
		lineNumber := index + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if line != "[[plugin]]" {
				return Catalog{}, fmt.Errorf("%s: line %d: unknown table %q, want [[plugin]]", FileName, lineNumber, line)
			}
			if current != nil {
				if err := finish(current); err != nil {
					return Catalog{}, err
				}
			}
			entryIndex++
			current = &Entry{}
			continue
		}

		if current == nil {
			return Catalog{}, fmt.Errorf("%s: line %d: a key outside a [[plugin]] entry", FileName, lineNumber)
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Catalog{}, fmt.Errorf("%s: line %d: %q is not a key = value pair", FileName, lineNumber, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "repo", "reason", "ref":
			text, err := quoted(value)
			if err != nil {
				return Catalog{}, fmt.Errorf("%s: line %d: %s must be a quoted string", FileName, lineNumber, key)
			}
			switch key {
			case "repo":
				current.Repo = text
			case "reason":
				current.Reason = text
			case "ref":
				current.Ref = text
			}
		case "optional":
			switch value {
			case "true":
				current.Optional = true
			case "false":
				current.Optional = false
			default:
				return Catalog{}, fmt.Errorf("%s: line %d: optional must be true or false", FileName, lineNumber)
			}
		default:
			return Catalog{}, fmt.Errorf("%s: line %d: unknown key %q", FileName, lineNumber, key)
		}
	}

	if current != nil {
		if err := finish(current); err != nil {
			return Catalog{}, err
		}
	}
	return catalog, nil
}

// quoted reads a one-line quoted TOML string.
func quoted(value string) (string, error) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("not a quoted string")
	}
	return value[1 : len(value)-1], nil
}

// stripComment removes a trailing '#' comment, keeping one inside quotes.
func stripComment(line string) string {
	quoted := false
	for index, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '#' && !quoted:
			return line[:index]
		}
	}
	return line
}

// Read reads the catalog at path, naming the file in every error it reports.
func Read(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("read %s: %w", path, err)
	}
	catalog, err := Parse(data)
	if err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

// Locate returns the catalog an installation ships: the explicit path when one
// was given, otherwise the file beside the binary — its directory first, then
// the plugin root above bin/hu — and finally the working directory.
func Locate(explicit, executable, cwd string) (string, error) {
	if explicit != "" {
		if !isFile(explicit) {
			return "", fmt.Errorf("no %s at %s", FileName, explicit)
		}
		return explicit, nil
	}

	candidates := make([]string, 0, 3)
	if executable != "" {
		dir := filepath.Dir(executable)
		candidates = append(candidates, filepath.Join(dir, FileName))
		candidates = append(candidates, filepath.Join(dir, "..", FileName))
	}
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, FileName))
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if isFile(candidate) {
			return candidate, nil
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no %s found", FileName)
	}
	return "", fmt.Errorf("no %s found; looked at %s", FileName, strings.Join(candidates, ", "))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
