package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// TestDefaultsAreEveryToolAndSourceOff pins S2: the container ships no tool on,
// and no which source on either, so installing it changes nothing until the
// user opts in.
func TestDefaultsAreEveryToolAndSourceOff(t *testing.T) {
	if got, want := strings.Join(Tools(), ","), "which,plugins,title"; got != want {
		t.Fatalf("Tools() = %q, want %q", got, want)
	}
	if got, want := strings.Join(Sources(), ","), "hs,gentle,custom,discovered"; got != want {
		t.Fatalf("Sources() = %q, want %q", got, want)
	}

	config := Default()
	for _, tool := range Tools() {
		if config.ToolEnabled(tool) {
			t.Fatalf("ToolEnabled(%q) = true, want every tool off by default", tool)
		}
	}
	for _, source := range Sources() {
		if config.SourceEnabled(source) {
			t.Fatalf("SourceEnabled(%q) = true, want every source off by default", source)
		}
	}
	if config.ToolEnabled("future") {
		t.Fatal("ToolEnabled(\"future\") = true, want an unknown tool off")
	}
	if config.SourceEnabled("future") {
		t.Fatal("SourceEnabled(\"future\") = true, want an unknown source off")
	}
}

func TestConfigPathFallsBackFromPluginConfigDirToXDGToHome(t *testing.T) {
	root := t.TempDir()
	override := filepath.Join(root, "plugin-config")

	path, err := Path(envFrom(map[string]string{"HERDR_PLUGIN_CONFIG_DIR": override, "XDG_CONFIG_HOME": filepath.Join(root, "xdg")}))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join(override, "config.json"); path != want {
		t.Fatalf("Path = %q, want %q", path, want)
	}

	path, err = Path(envFrom(map[string]string{"XDG_CONFIG_HOME": filepath.Join(root, "xdg")}))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(root, "xdg", "herdr", "plugins", "config", "herdr-utils", "config.json")
	if path != want {
		t.Fatalf("Path = %q, want %q", path, want)
	}

	path, err = Path(envFrom(map[string]string{"HOME": filepath.Join(root, "home")}))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want = filepath.Join(root, "home", ".config", "herdr", "plugins", "config", "herdr-utils", "config.json")
	if path != want {
		t.Fatalf("Path = %q, want %q", path, want)
	}

	if _, err := Path(envFrom(map[string]string{})); err == nil {
		t.Fatal("Path with no environment = nil error, want the refusal naming the variables")
	}
}

func TestLoadMissingConfigIsEveryToolOff(t *testing.T) {
	root := t.TempDir()
	getenv := envFrom(map[string]string{"XDG_CONFIG_HOME": root})

	document, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load a missing config: %v", err)
	}
	if document.Present {
		t.Fatal("Present = true, want false for a missing config")
	}
	for _, tool := range Tools() {
		if document.Config.ToolEnabled(tool) {
			t.Fatalf("ToolEnabled(%q) = true, want a missing config to mean every tool off", tool)
		}
	}
	for _, source := range Sources() {
		if document.Config.SourceEnabled(source) {
			t.Fatalf("SourceEnabled(%q) = true, want a missing config to mean every source off", source)
		}
	}
}

func TestLoadReadsOnlyTheKnownToggles(t *testing.T) {
	root := t.TempDir()
	path := writeConfig(t, root, `{"tools": {"which": true, "plugins": false}, "which": {"sources": {"hs": true}}}`)

	document, err := Load(envFrom(map[string]string{"XDG_CONFIG_HOME": root}))
	if err != nil {
		t.Fatalf("Load %s: %v", path, err)
	}
	if !document.Present {
		t.Fatal("Present = false, want true")
	}
	if !document.Config.ToolEnabled(ToolWhich) {
		t.Fatal("ToolEnabled(\"which\") = false, want the declared true")
	}
	if document.Config.ToolEnabled(ToolPlugins) {
		t.Fatal("ToolEnabled(\"plugins\") = true, want the declared false")
	}
	if !document.Config.SourceEnabled(SourceHS) {
		t.Fatal("SourceEnabled(\"hs\") = false, want the declared true")
	}
	for _, source := range []string{SourceGentle, SourceCustom, SourceDiscovered} {
		if document.Config.SourceEnabled(source) {
			t.Fatalf("SourceEnabled(%q) = true, want a missing source off", source)
		}
	}
}

// TestSaveWritesTheWholeDefaultMatrixThenKeepsUnknownKeys pins the two halves of
// the config contract: a save records every known toggle, so the file documents
// itself, and it never drops a key this version does not know.
func TestSaveWritesTheWholeDefaultMatrixThenKeepsUnknownKeys(t *testing.T) {
	root := t.TempDir()
	getenv := envFrom(map[string]string{"XDG_CONFIG_HOME": root})
	path, err := Path(getenv)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	if err := Save(path, Document{Config: Default()}); err != nil {
		t.Fatalf("Save a fresh document: %v", err)
	}
	document := readRaw(t, path)
	for _, section := range []string{"tools", "which"} {
		if _, ok := document[section]; !ok {
			t.Fatalf("saved config holds no %q section: %v", section, document)
		}
	}
	tools := section(t, document, "tools")
	for _, tool := range Tools() {
		if value, ok := tools[tool]; !ok || value != false {
			t.Fatalf("saved tools[%q] = %v (present %v), want false", tool, value, ok)
		}
	}
	sources := section(t, section(t, document, "which"), "sources")
	for _, source := range Sources() {
		if value, ok := sources[source]; !ok || value != false {
			t.Fatalf("saved which.sources[%q] = %v (present %v), want false", source, value, ok)
		}
	}

	if err := os.WriteFile(path, []byte(`{
  "wire": {"keep": 1},
  "tools": {"which": true, "future": true},
  "which": {"sources": {"hs": true, "future": true}, "extra": "kept"},
  "other": [1, 2]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded.Config.Tools[ToolPlugins] = true
	if err := Save(path, loaded); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rewritten := readRaw(t, path)
	if _, ok := rewritten["wire"]; !ok {
		t.Fatalf("the save dropped the unknown top-level key: %v", rewritten)
	}
	if _, ok := rewritten["other"]; !ok {
		t.Fatalf("the save dropped the unknown top-level key: %v", rewritten)
	}
	tools = section(t, rewritten, "tools")
	if tools["future"] != true {
		t.Fatalf("tools.future = %v, want the unknown tool kept", tools["future"])
	}
	if tools["which"] != true || tools["plugins"] != true {
		t.Fatalf("tools = %v, want both known tools recorded", tools)
	}
	which := section(t, rewritten, "which")
	if which["extra"] != "kept" {
		t.Fatalf("which.extra = %v, want the unknown key kept", which["extra"])
	}
	sources = section(t, which, "sources")
	for key, want := range map[string]any{"hs": true, "future": true, "gentle": false, "custom": false, "discovered": false} {
		if sources[key] != want {
			t.Fatalf("which.sources[%q] = %v, want %v", key, sources[key], want)
		}
	}
}

func TestLoadRejectsAMalformedToggleAndNamesIt(t *testing.T) {
	for name, body := range map[string]string{
		"a tool that is not a boolean":          `{"tools": {"which": "yes"}}`,
		"a tools section that is not an object": `{"tools": true}`,
		"a source that is not a boolean":        `{"which": {"sources": {"hs": 1}}}`,
		"a document that is not JSON":           `not json`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, body)

			_, err := Load(envFrom(map[string]string{"XDG_CONFIG_HOME": root}))
			if err == nil {
				t.Fatal("Load = nil error, want the malformed file refused")
			}
			if !strings.Contains(err.Error(), "config.json") {
				t.Fatalf("error = %q, want the file named", err)
			}
		})
	}
}

func writeConfig(t *testing.T, root, body string) string {
	t.Helper()
	path := filepath.Join(root, "herdr", "plugins", "config", "herdr-utils", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write the config: %v", err)
	}
	return path
}

func readRaw(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, data)
	}
	return document
}

func section(t *testing.T, document map[string]any, name string) map[string]any {
	t.Helper()
	value, ok := document[name]
	if !ok {
		t.Fatalf("document holds no %q section: %v", name, document)
	}
	section, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%q = %T, want an object", name, value)
	}
	return section
}

// TestLoadReadsTheManualEntriesAndTheSavedView pins the two parts of the guide's
// config a user writes by hand: the manual entry arrays and the view the guide
// writes back when it cycles.
func TestLoadReadsTheManualEntriesAndTheSavedView(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `{
  "tools": {"which": true},
  "which": {
    "sources": {"gentle": true, "custom": true},
    "view": "list",
    "gentle": [{"key": "ctrl+g", "description": "Lazygit", "command": "fish -c lazygit"}],
    "custom": [
      {"key": "prefix+w", "description": "Which", "command": "bin/hu which"},
      {"description": "no key, so it cannot be shown"},
      {"key": "  ", "description": "blank key"}
    ]
  }
}`)

	document, err := Load(envFrom(map[string]string{"XDG_CONFIG_HOME": root}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := document.Config.Which.View; got != ViewList {
		t.Fatalf("View = %q, want %q", got, ViewList)
	}
	gentle := document.Config.Which.Gentle
	if len(gentle) != 1 {
		t.Fatalf("Gentle = %#v, want one entry", gentle)
	}
	if want := (Entry{Key: "ctrl+g", Description: "Lazygit", Command: "fish -c lazygit"}); gentle[0] != want {
		t.Fatalf("Gentle[0] = %#v, want %#v", gentle[0], want)
	}
	custom := document.Config.Which.Custom
	if len(custom) != 1 {
		t.Fatalf("Custom = %#v, want the keyless entries skipped", custom)
	}
	if want := (Entry{Key: "prefix+w", Description: "Which", Command: "bin/hu which"}); custom[0] != want {
		t.Fatalf("Custom[0] = %#v, want %#v", custom[0], want)
	}
}

func TestLoadNormalizesAnUnknownViewToKeys(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `{"which": {"view": "grid"}}`)

	document, err := Load(envFrom(map[string]string{"XDG_CONFIG_HOME": root}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := document.Config.Which.View; got != ViewKeys {
		t.Fatalf("View = %q, want the unknown view normalized to %q", got, ViewKeys)
	}

	if got := NormalizeView(""); got != ViewKeys {
		t.Fatalf("NormalizeView(\"\") = %q, want %q", got, ViewKeys)
	}
	if got := NormalizeView("LIST"); got != ViewList {
		t.Fatalf("NormalizeView(\"LIST\") = %q, want %q", got, ViewList)
	}
}

func TestLoadRejectsMalformedManualEntriesAndNamesThem(t *testing.T) {
	for name, body := range map[string]string{
		"a gentle section that is not an array": `{"which": {"gentle": {"key": "a"}}}`,
		"an entry that is not an object":        `{"which": {"custom": ["a"]}}`,
		"a key that is not a string":            `{"which": {"custom": [{"key": 1}]}}`,
		"a view that is not a string":           `{"which": {"view": 2}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, body)

			_, err := Load(envFrom(map[string]string{"XDG_CONFIG_HOME": root}))
			if err == nil {
				t.Fatal("Load = nil error, want the malformed file refused")
			}
			if !strings.Contains(err.Error(), "config.json") {
				t.Fatalf("error = %q, want the file named", err)
			}
		})
	}
}

// TestSaveRecordsTheViewAndKeepsTheManualEntries pins the round trip a ctrl+t in
// the guide depends on: the view is written, and the hand-maintained entry
// arrays survive byte for byte, unknown fields included.
func TestSaveRecordsTheViewAndKeepsTheManualEntries(t *testing.T) {
	root := t.TempDir()
	getenv := envFrom(map[string]string{"XDG_CONFIG_HOME": root})
	writeConfig(t, root, `{
  "tools": {"which": true},
  "which": {
    "sources": {"gentle": true},
    "gentle": [{"key": "ctrl+g", "description": "Lazygit", "command": "fish -c lazygit", "note": "kept"}]
  }
}`)
	document, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	document.Config.Which.View = ViewList

	path, err := Path(getenv)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := Save(path, document); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw := readRaw(t, path)
	which := section(t, raw, "which")
	if which["view"] != ViewList {
		t.Fatalf("which.view = %v, want %q", which["view"], ViewList)
	}
	gentle, ok := which["gentle"].([]any)
	if !ok || len(gentle) != 1 {
		t.Fatalf("which.gentle = %#v, want the hand-written array kept", which["gentle"])
	}
	first, ok := gentle[0].(map[string]any)
	if !ok {
		t.Fatalf("which.gentle[0] = %T, want an object", gentle[0])
	}
	if first["note"] != "kept" || first["command"] != "fish -c lazygit" {
		t.Fatalf("which.gentle[0] = %#v, want the entry kept verbatim", first)
	}
	if tools := section(t, raw, "tools"); tools[ToolWhich] != true {
		t.Fatalf("tools.which = %v, want the enabled tool kept", tools[ToolWhich])
	}
}
