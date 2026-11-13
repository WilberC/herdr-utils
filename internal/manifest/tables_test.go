package manifest

import (
	"strings"
	"testing"
)

// herdrConfig is shaped like a real ~/.config/herdr/config.toml: tables the
// reader must skip, multi-line arrays, both multi-line string forms, a label
// after a value, a non-string value and a line that is not a key at all.
const herdrConfig = `onboarding = false

[ui]
sidebar_width = 25

[[ui.sidebar.agents]]
rows = [
  ["state_icon", "workspace"],
  [{ token = "$counts", dim = true }],
]

[[keys.command]]
key = "ctrl+shift+u"
type = "popup"
command = '''exec fish -c 'echo limits' '''
description = "Agent Usage: open limits pane"   # a label
width = "70%"

[[keys.command]]
key = "prefix+p"
type = "plugin_action"
command = "ray.plugin-manager.open"
description = "Plugin manager: open"

[[keys.command]]
key = "ctrl+shift+v"
type = "shell"
command = """exec fish -c 'nvim .'"""
this line is not a table entry
count = 3

[other]
key = "ignored"
`

func TestTablesReadsEveryEntryOfTheNamedArrayOfTables(t *testing.T) {
	tables := Tables([]byte(herdrConfig), "keys.command")

	if len(tables) != 3 {
		t.Fatalf("Tables = %d entries, want 3: %#v", len(tables), tables)
	}

	first := tables[0].Values
	if first["key"] != "ctrl+shift+u" || first["type"] != "popup" {
		t.Fatalf("first entry = %#v, want the popup binding", first)
	}
	if first["description"] != "Agent Usage: open limits pane" {
		t.Fatalf("description = %q, want the label before the comment stripped", first["description"])
	}
	if first["width"] != "70%" {
		t.Fatalf("width = %q, want the entry's extra string key collected", first["width"])
	}
	if !strings.Contains(first["command"], "echo limits") || strings.Contains(first["command"], "'''") {
		t.Fatalf("command = %q, want the multi-line literal without its delimiters", first["command"])
	}

	second := tables[1].Values
	if second["type"] != "plugin_action" || second["command"] != "ray.plugin-manager.open" {
		t.Fatalf("second entry = %#v, want the plugin action binding", second)
	}

	third := tables[2].Values
	if !strings.Contains(third["command"], "nvim .") || strings.Contains(third["command"], `"""`) {
		t.Fatalf("command = %q, want the multi-line basic string without its delimiters", third["command"])
	}
	if _, ok := third["count"]; ok {
		t.Fatalf("non-string value collected: %#v, want it skipped", third)
	}
	if _, ok := third["key \"ignored\""]; ok {
		t.Fatalf("a key of another table leaked into the entry: %#v", third)
	}
	if third["key"] != "ctrl+shift+v" {
		t.Fatalf("third entry key = %q, want the binding before the broken line", third["key"])
	}
}

func TestTablesSkipsAnotherTablesKeys(t *testing.T) {
	tables := Tables([]byte(herdrConfig), "keys.command")

	for _, table := range tables {
		if table.Values["key"] == "ignored" {
			t.Fatalf("Tables collected [other] into %#v", table)
		}
	}
}

func TestTablesIsEmptyWithoutTheNamedTable(t *testing.T) {
	if got := Tables([]byte("id = \"herdr-utils\"\n"), "keys.command"); len(got) != 0 {
		t.Fatalf("Tables = %#v, want no entries", got)
	}
}

func TestTablesSkipsAnEntryWithoutAnyStringValue(t *testing.T) {
	body := "[[keys.command]]\nwidth = 70\n"

	tables := Tables([]byte(body), "keys.command")

	if len(tables) != 1 {
		t.Fatalf("Tables = %#v, want the entry kept for its position", tables)
	}
	if len(tables[0].Values) != 0 {
		t.Fatalf("Values = %#v, want no non-string value collected", tables[0].Values)
	}
}
