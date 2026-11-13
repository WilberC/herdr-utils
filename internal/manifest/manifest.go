// Package manifest reads TOML documents for the fields herdr-utils reports at
// runtime: herdr-plugin.toml for the plugin version and the herdr version floor,
// and the [[keys.command]] tables of the user's herdr config.toml.
//
// herdr and the user read those files with a TOML parser, and this package does
// not try to replace it. Parse and Tables are structural readers: they collect
// the quoted string values they need and ignore everything else, including the
// tables, numbers and multi-line arrays the documents hold. That keeps the
// binary free of a TOML dependency, and the repository's tests keep validating
// the plugin manifest with python's tomllib, so a manifest this reader accepts
// but herdr rejects still fails the suite.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileName is the name herdr requires of a plugin manifest.
const FileName = "herdr-plugin.toml"

// Manifest is the part of the manifest the binary reads.
type Manifest struct {
	ID              string
	Name            string
	Version         string
	MinHerdrVersion string
}

// required names the keys a manifest must declare for the binary to report it.
var required = []string{"id", "version", "min_herdr_version"}

// Parse reads the fields the binary reports from manifest bytes. A key declared
// twice, a value that is not a quoted string, or a missing required key is an
// error; anything else in the file is left to herdr.
func Parse(data []byte) (Manifest, error) {
	values := make(map[string]string, 4)
	section := ""

	for index, raw := range strings.Split(string(data), "\n") {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !reported(key) {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return Manifest{}, fmt.Errorf("line %d: %s must be a quoted string", index+1, key)
		}
		if _, again := values[key]; again {
			return Manifest{}, fmt.Errorf("line %d: %s is declared twice", index+1, key)
		}
		values[key] = value[1 : len(value)-1]
	}

	for _, key := range required {
		if values[key] == "" {
			return Manifest{}, fmt.Errorf("%s declares no %s", FileName, key)
		}
	}
	return Manifest{
		ID:              values["id"],
		Name:            values["name"],
		Version:         values["version"],
		MinHerdrVersion: values["min_herdr_version"],
	}, nil
}

// reported names the keys Parse collects: the ones the binary prints and the
// display name the docs show.
func reported(key string) bool {
	switch key {
	case "id", "name", "version", "min_herdr_version":
		return true
	}
	return false
}

// Table is one entry of an array of tables, as Tables collects it: the quoted
// string values it declares, by key. A value that is not a quoted string is
// absent, so a caller reads a missing key exactly like an unreadable one.
type Table struct {
	Values map[string]string
}

// Tables reads every [[<name>]] entry of a TOML document and collects the quoted
// string values each entry declares. Everything else is skipped rather than
// refused: another table's keys, numbers, booleans, inline tables, arrays that
// span lines, and lines that are not key/value pairs, so a hand-edited config
// with one broken line still yields every binding it can read.
func Tables(data []byte, name string) []Table {
	lines := strings.Split(string(data), "\n")
	header := normalizeHeader("[[" + name + "]]")

	tables := make([]Table, 0, 4)
	current := -1

	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(stripComment(lines[index]))
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]"):
			if normalizeHeader(trimmed) == header {
				tables = append(tables, Table{Values: map[string]string{}})
				current = len(tables) - 1
			} else {
				current = -1
			}
			continue
		case strings.HasPrefix(trimmed, "["):
			current = -1
			continue
		}
		if current < 0 {
			continue
		}

		key, rest, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value, consumed, ok := readValue(strings.TrimSpace(rest), lines, index+1)
		index += consumed
		if !ok {
			continue
		}
		tables[current].Values[key] = value
	}
	return tables
}

// normalizeHeader drops the spaces a TOML header may carry, so [[keys.command]]
// and [[ keys.command ]] are the same table.
func normalizeHeader(header string) string {
	return strings.ReplaceAll(header, " ", "")
}

// readValue reads one value that starts on the line after rest. It returns the
// value, how many following lines it consumed, and whether the value is a quoted
// string Tables collects. A multi-line string and an array that spans lines are
// consumed whole, so their contents never look like a key.
func readValue(rest string, lines []string, next int) (string, int, bool) {
	for _, delimiter := range []string{"'''", `"""`} {
		if !strings.HasPrefix(rest, delimiter) {
			continue
		}
		body := rest[len(delimiter):]
		if end := strings.Index(body, delimiter); end >= 0 {
			return body[:end], 0, true
		}
		var value strings.Builder
		value.WriteString(body)
		for consumed := 1; next+consumed-1 < len(lines); consumed++ {
			line := lines[next+consumed-1]
			value.WriteString("\n")
			if end := strings.Index(line, delimiter); end >= 0 {
				value.WriteString(line[:end])
				return value.String(), consumed, true
			}
			value.WriteString(line)
		}
		return value.String(), len(lines) - next, true
	}

	if strings.HasPrefix(rest, "\"") {
		if len(rest) < 2 || !strings.HasSuffix(rest, "\"") {
			return "", 0, false
		}
		return rest[1 : len(rest)-1], 0, true
	}

	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "{") {
		depth := bracketDepth(rest)
		consumed := 0
		for depth > 0 && next+consumed < len(lines) {
			depth += bracketDepth(lines[next+consumed])
			consumed++
		}
		return "", consumed, false
	}

	return "", 0, false
}

// bracketDepth counts the brackets a line opens and closes, ignoring the ones
// inside a quoted string.
func bracketDepth(line string) int {
	depth := 0
	quoted := false
	for index := 0; index < len(line); index++ {
		switch {
		case quoted:
			if line[index] == '\\' {
				index++
				continue
			}
			if line[index] == '"' {
				quoted = false
			}
		case line[index] == '"':
			quoted = true
		case line[index] == '[' || line[index] == '{':
			depth++
		case line[index] == ']' || line[index] == '}':
			depth--
		}
	}
	if depth < 0 {
		return 0
	}
	return depth
}

// stripComment removes a trailing '#' comment. A '#' inside a quoted string is
// kept, and a string that does not close on the line keeps the whole line, so a
// multi-line value is never truncated by a '#' it holds.
func stripComment(line string) string {
	for index := 0; index < len(line); index++ {
		switch {
		case strings.HasPrefix(line[index:], "'''"):
			end := strings.Index(line[index+3:], "'''")
			if end < 0 {
				return line
			}
			index += end + 5
		case strings.HasPrefix(line[index:], `"""`):
			end := strings.Index(line[index+3:], `"""`)
			if end < 0 {
				return line
			}
			index += end + 5
		case line[index] == '"':
			end := closingQuote(line, index)
			if end < 0 {
				return line
			}
			index = end
		case line[index] == '#':
			return line[:index]
		}
	}
	return line
}

// closingQuote returns the index of the quote that closes the basic string
// opened at start, or -1 when the string does not close on this line. An escaped
// quote does not close it.
func closingQuote(line string, start int) int {
	for index := start + 1; index < len(line); index++ {
		switch line[index] {
		case '\\':
			index++
		case '"':
			return index
		}
	}
	return -1
}

// Read reads the manifest at path, naming the file in every error it reports.
func Read(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	document, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	return document, nil
}

// Locate returns the manifest an installation ships: the explicit path when one
// was given, otherwise the manifest beside the binary — its own directory first,
// then the plugin root above it, which is where bin/hu sits under
// herdr-plugin.toml — and finally the one in the working directory.
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

// Compare orders two dotted versions, the way herdr versions are written: at
// least two numeric parts, compared part by part, and a missing trailing part
// read as zero so 1.0 and 1.0.0 are the same version. A version that is not
// dotted numbers is an error, so a caller reports an unreadable floor instead of
// comparing against a guess.
func Compare(left, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}

	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		got, want := 0, 0
		if index < len(leftParts) {
			got = leftParts[index]
		}
		if index < len(rightParts) {
			want = rightParts[index]
		}
		switch {
		case got < want:
			return -1, nil
		case got > want:
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(version string) ([]int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("%q is not a dotted version", version)
	}
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return nil, fmt.Errorf("%q is not a dotted version", version)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}
