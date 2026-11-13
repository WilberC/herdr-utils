package plugins

import (
	"fmt"
	"strings"
)

// The markers the generated README section lives between. A render replaces
// everything from the start marker through the end marker.
const (
	StartMarker = "<!-- generated:recommended start -->"
	EndMarker   = "<!-- generated:recommended end -->"
)

// Render returns the generated section, markers included, for a catalog.
func Render(catalog Catalog) string {
	var section strings.Builder
	section.WriteString(StartMarker + "\n")
	section.WriteString("Install the core entries with `hu plugins sync`; add `--optional` to include the optional ones.\n\n")
	section.WriteString("| Plugin | Why | Kind |\n")
	section.WriteString("| --- | --- | --- |\n")
	for _, entry := range catalog.Entries {
		kind := "core"
		if entry.Optional {
			kind = "optional"
		}
		fmt.Fprintf(&section, "| [`%s`](https://github.com/%s) | %s | %s |\n",
			entry.Repo, entry.Repo, escapeCell(entry.Reason), kind)
	}
	section.WriteString(EndMarker)
	return section.String()
}

// escapeCell keeps a reason from breaking the table it is rendered into.
func escapeCell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}

// ReplaceSection writes generated between the markers of document, leaving
// everything outside them untouched. Missing or duplicated markers are an error:
// the section can be rendered into a document once its markers exist, and a
// second marker pair would make the target ambiguous.
func ReplaceSection(document, generated string) (string, error) {
	start := strings.Index(document, StartMarker)
	end := strings.Index(document, EndMarker)

	if strings.Count(document, StartMarker) > 1 {
		return "", fmt.Errorf("%s is declared twice", StartMarker)
	}
	if strings.Count(document, EndMarker) > 1 {
		return "", fmt.Errorf("%s is declared twice", EndMarker)
	}
	if start < 0 {
		return "", fmt.Errorf("no %s marker", StartMarker)
	}
	if end < 0 {
		return "", fmt.Errorf("no %s marker", EndMarker)
	}
	if end < start {
		return "", fmt.Errorf("the end marker comes before the start marker")
	}

	prefix := document[:start]
	suffix := document[end+len(EndMarker):]
	return prefix + generated + suffix, nil
}
