package which

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

// fixture is an offline guide environment: a herdr config path inside a temporary
// directory, a plugin config path, and the fake herdr client.
type fixture struct {
	root       string
	configPath string
	env        map[string]string
	client     *herdrmock.Fake
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	return &fixture{
		root:       root,
		configPath: filepath.Join(root, "plugin-config", "config.json"),
		env:        map[string]string{"HERDR_CONFIG_PATH": filepath.Join(root, "herdr", "config.toml")},
		client:     &herdrmock.Fake{},
	}
}

func (f *fixture) writeHerdrConfig(t *testing.T, body string) {
	t.Helper()
	path := f.env["HERDR_CONFIG_PATH"]
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the herdr config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write the herdr config: %v", err)
	}
}

func (f *fixture) enabled(sources ...string) config.Config {
	document := config.Default()
	for _, name := range sources {
		document.Which.Sources[name] = true
	}
	return document
}

func (f *fixture) build(t *testing.T, document config.Config) Guide {
	t.Helper()
	return Build(context.Background(), Options{
		Config:     document,
		ConfigPath: f.configPath,
		Getenv:     func(name string) string { return f.env[name] },
		Actions:    f.client,
	})
}

func sourceNamed(t *testing.T, guide Guide, name string) Source {
	t.Helper()
	for _, source := range guide.Sources {
		if source.Name() == name {
			return source
		}
	}
	t.Fatalf("the guide has no %s source: %#v", name, guide.Sources)
	return nil
}

func problemOf(t *testing.T, source Source) string {
	t.Helper()
	problem, ok := source.(ProblemSource)
	if !ok {
		return ""
	}
	return problem.Problem()
}

// herdrBindings is shaped like a real herdr config.toml: every binding kind, a
// description missing where herdr can resolve one, an action herdr cannot
// resolve, and a plugin_action written as herdr writes it.
const herdrBindings = `[[keys.command]]
key = "prefix+p"
type = "plugin_action"
command = "ray.plugin-manager.open"

[[keys.command]]
key = "prefix+."
type = "plugin_action"
command = "herdr-suite.last-tab"
description = "Toggle to last tab"

[[keys.command]]
key = "prefix+k"
type = "plugin_action"
command = "future.action"

[[keys.command]]
key = "ctrl+shift+u"
type = "popup"
command = '''exec fish -c 'echo limits' '''
description = "Agent Usage: open limits pane"

[[keys.command]]
key = "ctrl+shift+v"
command = "nvim ."
`

// TestHSSourceReadsTheBoundKeys pins the HS source: entries come from the
// [[keys.command]] tables of the real herdr config, a plugin_action resolves its
// action title when herdr reports one, and the raw command shows otherwise.
func TestHSSourceReadsTheBoundKeys(t *testing.T) {
	f := newFixture(t)
	f.writeHerdrConfig(t, herdrBindings)
	f.client.Actions = []herdrmock.Action{
		{Plugin: "ray.plugin-manager", ID: "open", Title: "Open plugin manager"},
		{Plugin: "herdr-suite", ID: "last-tab", Title: "Toggle to the previous tab"},
	}

	source := sourceNamed(t, f.build(t, f.enabled(config.SourceHS)), NameHS)
	entries := source.Entries()

	if len(entries) != 5 {
		t.Fatalf("Entries = %d, want every binding: %#v", len(entries), entries)
	}
	if want := (Entry{Key: "prefix+p", Description: "Open plugin manager", Kind: KindPluginAction, Command: "ray.plugin-manager.open", ActionID: "ray.plugin-manager.open"}); entries[0] != want {
		t.Fatalf("first entry = %#v, want %#v", entries[0], want)
	}
	if entries[1].Description != "Toggle to last tab" {
		t.Fatalf("description = %q, want the config's own description to win", entries[1].Description)
	}
	if entries[2].Description != "future.action" {
		t.Fatalf("description = %q, want the raw command when herdr cannot resolve the action", entries[2].Description)
	}
	if entries[3].Kind != KindPopup || entries[3].Description != "Agent Usage: open limits pane" || !strings.Contains(entries[3].Command, "echo limits") {
		t.Fatalf("popup entry = %#v, want the config's description and multi-line command", entries[3])
	}
	if entries[4].Kind != KindShell || entries[4].Description != "nvim ." {
		t.Fatalf("shell entry = %#v, want a missing type read as a shell binding", entries[4])
	}
}

// TestHSSourceExplainsAMissingHerdrConfig pins the source's gate: it never
// renders a silently empty column.
func TestHSSourceExplainsAMissingHerdrConfig(t *testing.T) {
	f := newFixture(t)

	source := sourceNamed(t, f.build(t, f.enabled(config.SourceHS)), NameHS)

	if len(source.Entries()) != 0 {
		t.Fatalf("Entries = %#v, want none without a config file", source.Entries())
	}
	problem := problemOf(t, source)
	for _, want := range []string{"no herdr config", f.env["HERDR_CONFIG_PATH"]} {
		if !strings.Contains(problem, want) {
			t.Fatalf("Problem = %q, want it to contain %q", problem, want)
		}
	}
}

func TestHSSourceSkipsMalformedLinesInsteadOfFailing(t *testing.T) {
	f := newFixture(t)
	f.writeHerdrConfig(t, `[[keys.command]]
key = "prefix+a"
description = "Kept"
broken line without an equals sign
count = 3

[[keys.command]]
key =
description = "no key, so nothing to bind"

[[keys.command]]
key = "prefix+b"
description = "
`)

	source := sourceNamed(t, f.build(t, f.enabled(config.SourceHS)), NameHS)
	entries := source.Entries()

	if len(entries) != 2 {
		t.Fatalf("Entries = %#v, want the two readable bindings", entries)
	}
	if entries[0].Key != "prefix+a" || entries[0].Description != "Kept" {
		t.Fatalf("first entry = %#v, want the readable binding", entries[0])
	}
	if entries[1].Key != "prefix+b" {
		t.Fatalf("second entry = %#v, want the binding with an unreadable description kept by its key", entries[1])
	}
}

// TestTheSourceTogglesBuildExactlyTheEnabledSources pins the toggle matrix: with
// every combination, the guide holds exactly the enabled sources, in catalog
// order, and nothing more.
func TestTheSourceTogglesBuildExactlyTheEnabledSources(t *testing.T) {
	f := newFixture(t)
	f.writeHerdrConfig(t, herdrBindings)

	names := map[string]string{
		config.SourceHS:         NameHS,
		config.SourceGentle:     NameGentle,
		config.SourceCustom:     NameCustom,
		config.SourceDiscovered: NameDiscovered,
	}
	order := config.Sources()

	for mask := 0; mask < 1<<len(order); mask++ {
		document := config.Default()
		want := make([]string, 0, len(order))
		for index, name := range order {
			if mask&(1<<index) == 0 {
				continue
			}
			document.Which.Sources[name] = true
			want = append(want, names[name])
		}

		guide := f.build(t, document)
		got := make([]string, 0, len(guide.Sources))
		for _, source := range guide.Sources {
			got = append(got, source.Name())
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mask %04b: sources = %v, want %v", mask, got, want)
		}
	}
}

func TestGentleAndCustomReadTheirManualEntries(t *testing.T) {
	f := newFixture(t)
	document := f.enabled(config.SourceGentle, config.SourceCustom)
	document.Which.Gentle = []config.Entry{{Key: "ctrl+g", Description: "Lazygit", Command: "fish -c lazygit"}}
	document.Which.Custom = []config.Entry{{Key: "prefix+w", Command: "bin/hu which"}, {Key: "prefix+t"}}

	guide := f.build(t, document)

	gentle := sourceNamed(t, guide, NameGentle)
	want := Entry{Key: "ctrl+g", Description: "Lazygit", Kind: KindManual, Command: "fish -c lazygit"}
	if entries := gentle.Entries(); len(entries) != 1 || entries[0] != want {
		t.Fatalf("GENTLE entries = %#v, want %#v", entries, want)
	}
	if problem := problemOf(t, gentle); problem != "" {
		t.Fatalf("GENTLE problem = %q, want none with an entry", problem)
	}

	custom := sourceNamed(t, guide, NameCustom).Entries()
	if len(custom) != 2 {
		t.Fatalf("CUSTOM entries = %#v, want two", custom)
	}
	if custom[0].Description != "bin/hu which" {
		t.Fatalf("CUSTOM[0].Description = %q, want the command when no description is written", custom[0].Description)
	}
	if custom[1].Description != "prefix+t" {
		t.Fatalf("CUSTOM[1].Description = %q, want the key when nothing else is written", custom[1].Description)
	}
}

func TestAnEnabledManualSourcePointsAtItsConfigKey(t *testing.T) {
	f := newFixture(t)

	guide := f.build(t, f.enabled(config.SourceGentle, config.SourceCustom))

	for _, tt := range []struct{ name, key string }{
		{name: NameGentle, key: "which.gentle"},
		{name: NameCustom, key: "which.custom"},
	} {
		source := sourceNamed(t, guide, tt.name)
		if len(source.Entries()) != 0 {
			t.Fatalf("%s entries = %#v, want none", tt.name, source.Entries())
		}
		if problem := problemOf(t, source); !strings.Contains(problem, tt.key) {
			t.Fatalf("%s problem = %q, want it to name %s", tt.name, problem, tt.key)
		}
	}
}

func TestTheGuideStatesThatGentleDiscoveryIsPending(t *testing.T) {
	f := newFixture(t)

	guide := f.build(t, f.enabled(config.SourceGentle))

	notes := strings.Join(guide.Notes, " ")
	for _, want := range []string{"auto-discovery", "which.gentle"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("notes = %q, want them to contain %q", notes, want)
		}
	}
}

// TestDiscoveredSourceMapsEveryInstalledAction pins the DISCOVERED source: one
// entry per action of every installed plugin, keyed enter, invoking the
// fully-qualified action id through herdr.
func TestDiscoveredSourceMapsEveryInstalledAction(t *testing.T) {
	f := newFixture(t)
	f.client.Actions = []herdrmock.Action{
		{Plugin: "herdr-suite", ID: "save", Title: "Save space"},
		{Plugin: "herdr-bar", ID: "stop-titles", Title: "Stop tab watcher"},
		{Plugin: "herdr-bar", ID: "open", Title: "Open bar"},
		{Plugin: "usagebar", ID: "refresh"},
	}

	source := sourceNamed(t, f.build(t, f.enabled(config.SourceDiscovered)), NameDiscovered)

	want := []Entry{
		{Key: "enter", Description: "Open bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open", Plugin: "herdr-bar"},
		{Key: "enter", Description: "Stop tab watcher", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.stop-titles", ActionID: "herdr-bar.stop-titles", Plugin: "herdr-bar"},
		{Key: "enter", Description: "Save space", Kind: KindAction, Command: "herdr plugin action invoke herdr-suite.save", ActionID: "herdr-suite.save", Plugin: "herdr-suite"},
		{Key: "enter", Description: "refresh", Kind: KindAction, Command: "herdr plugin action invoke usagebar.refresh", ActionID: "usagebar.refresh", Plugin: "usagebar"},
	}
	if got := source.Entries(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DISCOVERED entries = %#v, want %#v", got, want)
	}
	for _, entry := range source.Entries() {
		if !entry.Invokable() {
			t.Fatalf("entry %#v is not invokable", entry)
		}
	}
	if problem := problemOf(t, source); problem != "" {
		t.Fatalf("DISCOVERED problem = %q, want none", problem)
	}
}

func TestDiscoveredSourceExplainsAHerdrFailure(t *testing.T) {
	f := newFixture(t)
	f.client.PluginActionsErr = errors.New("no server running")

	source := sourceNamed(t, f.build(t, f.enabled(config.SourceDiscovered)), NameDiscovered)

	if len(source.Entries()) != 0 {
		t.Fatalf("Entries = %#v, want none when herdr fails", source.Entries())
	}
	problem := problemOf(t, source)
	for _, want := range []string{"herdr", "no server running"} {
		if !strings.Contains(problem, want) {
			t.Fatalf("Problem = %q, want it to contain %q", problem, want)
		}
	}
}

func TestHerdrConfigPathHonoursTheEnvironment(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}

	if got, err := HerdrConfigPath(env(map[string]string{"HERDR_CONFIG_PATH": "/tmp/herdr.toml"})); err != nil || got != "/tmp/herdr.toml" {
		t.Fatalf("HerdrConfigPath = %q, %v, want /tmp/herdr.toml", got, err)
	}
	if got, err := HerdrConfigPath(env(map[string]string{"XDG_CONFIG_HOME": "/xdg"})); err != nil || got != "/xdg/herdr/config.toml" {
		t.Fatalf("HerdrConfigPath = %q, %v, want /xdg/herdr/config.toml", got, err)
	}
	if got, err := HerdrConfigPath(env(map[string]string{"HOME": "/home/u"})); err != nil || got != "/home/u/.config/herdr/config.toml" {
		t.Fatalf("HerdrConfigPath = %q, %v, want the ~/.config fallback", got, err)
	}
	if _, err := HerdrConfigPath(env(nil)); err == nil {
		t.Fatal("HerdrConfigPath with no environment = nil error, want the refusal naming the variables")
	}
}

func TestBuildCarriesTheSavedViewAndTheConfigPath(t *testing.T) {
	f := newFixture(t)
	document := f.enabled(config.SourceCustom)
	document.Which.View = config.ViewList

	guide := f.build(t, document)

	if guide.View != config.ViewList {
		t.Fatalf("View = %q, want %q", guide.View, config.ViewList)
	}
	if guide.ConfigPath != f.configPath {
		t.Fatalf("ConfigPath = %q, want %q", guide.ConfigPath, f.configPath)
	}
	if got, want := guide.Sources[0].Name(), NameCustom; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}
