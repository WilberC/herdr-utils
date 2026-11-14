// Package which builds and draws the keybinding guide: the sources the user
// enabled, their entries, and the Bubbletea screen that renders them. It never
// opens a herdr socket; the caller supplies the herdrmock client.
package which

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/manifest"
)

// The source headers, in the order the guide draws them.
const (
	NameHS         = "HS"
	NameGentle     = "GENTLE"
	NameCustom     = "CUSTOM"
	NameDiscovered = "DISCOVERED"
)

// The kinds an entry declares. The first three are herdr's own binding types;
// manual entries and discovered actions name themselves.
const (
	KindPluginAction = "plugin_action"
	KindShell        = "shell"
	KindPopup        = "popup"
	KindManual       = "manual"
	KindAction       = "action"
)

// Entry is one binding the guide shows: the key that runs it, what it does, what
// kind it is, and the command the guide reports for it. ActionID is the
// fully-qualified herdr action id ("<plugin>.<action>") when the entry names one.
type Entry struct {
	Key         string
	Description string
	Kind        string
	Command     string
	ActionID    string
	Plugin      string
}

// Invokable reports whether Enter runs the entry through herdr. Only the live
// actions of installed plugins are invoked; a bound key or a manual entry is
// shown, never executed.
func (e Entry) Invokable() bool { return e.Kind == KindAction && e.ActionID != "" }

// Unset reports whether the entry declares no key of its own: a live plugin
// action is invoked rather than pressed, so the guide draws it with an empty
// chip and a muted description.
func (e Entry) Unset() bool { return e.Key == "" || e.Kind == KindAction }

// Source is one group the guide renders as a column: its header name, the glyph
// that marks it, its colour, and the entries it declares.
type Source interface {
	Name() string
	Glyph() string
	Color() lipgloss.Color
	Entries() []Entry
}

// ProblemSource is a Source that is enabled but could not read its bindings, or
// has none yet. The guide renders its problem instead of a silently empty
// column.
type ProblemSource interface {
	Source
	Problem() string
}

type group struct {
	name    string
	glyph   string
	color   lipgloss.Color
	toggle  string
	entries []Entry
	problem string
}

func (g group) Name() string          { return g.name }
func (g group) Glyph() string         { return g.glyph }
func (g group) Color() lipgloss.Color { return g.color }
func (g group) Entries() []Entry      { return g.entries }
func (g group) Problem() string       { return g.problem }

// The glyph each source's header carries. They come from one solid family, so
// the four groups read as a set; theme.go owns their colour.
const (
	glyphHS         = "◆"
	glyphGentle     = "●"
	glyphCustom     = "■"
	glyphDiscovered = "▲"
)

// Options carries everything Build needs. Actions is the herdrmock client that
// reads and runs plugin actions; Invoke and Persist are the callbacks the screen
// uses to run a discovered action and to save the view.
type Options struct {
	Config     config.Config
	ConfigPath string
	Getenv     func(string) string
	Actions    herdrmock.ActionClient
	Invoke     func(context.Context, string) error
	Persist    func(string) error
}

// Guide is the built screen: the enabled sources in catalog order, the notes the
// footer shows, and the callbacks the model uses.
type Guide struct {
	Sources    []Source
	Notes      []string
	ConfigPath string
	HerdrPath  string
	View       string
	Invoke     func(context.Context, string) error
	Persist    func(string) error
}

// Build resolves every enabled source. A source that fails carries its own
// problem, so one unreachable surface never hides the rest; only the config
// itself can fail before this point.
func Build(ctx context.Context, options Options) Guide {
	getenv := options.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	herdrPath, err := HerdrConfigPath(getenv)
	if err != nil {
		herdrPath = ""
	}
	guide := Guide{
		ConfigPath: options.ConfigPath,
		HerdrPath:  herdrPath,
		View:       config.NormalizeView(options.Config.Which.View),
		Invoke:     options.Invoke,
		Persist:    options.Persist,
	}

	var actions []herdrmock.Action
	var actionsErr error
	if options.Config.SourceEnabled(config.SourceHS) || options.Config.SourceEnabled(config.SourceDiscovered) {
		actions, actionsErr = readActions(ctx, options.Actions)
	}

	if options.Config.SourceEnabled(config.SourceHS) {
		guide.Sources = append(guide.Sources, buildHS(herdrPath, actions))
	}
	if options.Config.SourceEnabled(config.SourceGentle) {
		guide.Sources = append(guide.Sources, buildManual(NameGentle, glyphGentle, colorGentle, "which.sources.gentle", "which.gentle", options.Config.Which.Gentle))
		guide.Notes = append(guide.Notes, "GENTLE auto-discovery is not implemented yet: the source reads the which.gentle entries you maintain.")
	}
	if options.Config.SourceEnabled(config.SourceCustom) {
		guide.Sources = append(guide.Sources, buildManual(NameCustom, glyphCustom, colorCustom, "which.sources.custom", "which.custom", options.Config.Which.Custom))
	}
	if options.Config.SourceEnabled(config.SourceDiscovered) {
		guide.Sources = append(guide.Sources, buildDiscovered(actions, actionsErr))
	}
	return guide
}

func readActions(ctx context.Context, client herdrmock.ActionClient) ([]herdrmock.Action, error) {
	if client == nil {
		return nil, errors.New("no herdr client")
	}
	return client.PluginActions(ctx, "")
}

// HerdrConfigPath resolves the herdr config the HS source reads:
// $HERDR_CONFIG_PATH when the user points at one, otherwise herdr/config.toml
// under $XDG_CONFIG_HOME, otherwise under $HOME/.config.
func HerdrConfigPath(getenv func(string) string) (string, error) {
	if path := getenv("HERDR_CONFIG_PATH"); path != "" {
		return path, nil
	}
	base := getenv("XDG_CONFIG_HOME")
	if base == "" {
		home := getenv("HOME")
		if home == "" {
			return "", errors.New("cannot locate the herdr config: HERDR_CONFIG_PATH, XDG_CONFIG_HOME and HOME are all unset")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "herdr", "config.toml"), nil
}

// buildHS reads the [[keys.command]] tables of the herdr config. A plugin_action
// binding resolves its action title when herdr reports one; without a title the
// command it declares is what the row shows.
func buildHS(path string, actions []herdrmock.Action) Source {
	source := group{name: NameHS, glyph: glyphHS, color: colorHS, toggle: "which.sources.hs"}
	if path == "" {
		source.problem = "cannot locate the herdr config: HERDR_CONFIG_PATH, XDG_CONFIG_HOME and HOME are all unset"
		return source
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		source.problem = fmt.Sprintf("no herdr config at %s; nothing is bound there yet", path)
		return source
	default:
		source.problem = fmt.Sprintf("cannot read %s: %v", path, err)
		return source
	}

	titles := actionTitles(actions)
	for _, table := range manifest.Tables(data, "keys.command") {
		key := strings.TrimSpace(table.Values["key"])
		if key == "" {
			continue
		}
		kind := strings.TrimSpace(table.Values["type"])
		if kind == "" {
			kind = KindShell
		}
		command := strings.TrimSpace(table.Values["command"])
		entry := Entry{Key: key, Kind: kind, Command: command}
		if kind == KindPluginAction {
			entry.ActionID = command
		}
		entry.Description = firstNonEmpty(
			strings.TrimSpace(table.Values["description"]),
			titles[entry.ActionID],
			command,
			key,
		)
		source.entries = append(source.entries, entry)
	}
	if len(source.entries) == 0 {
		source.problem = fmt.Sprintf("%s declares no [[keys.command]] entries", path)
	}
	return source
}

func actionTitles(actions []herdrmock.Action) map[string]string {
	titles := make(map[string]string, len(actions))
	for _, action := range actions {
		if action.Title == "" {
			continue
		}
		titles[action.Plugin+"."+action.ID] = action.Title
	}
	return titles
}

func buildManual(name, glyph string, color lipgloss.Color, toggle, configKey string, entries []config.Entry) Source {
	source := group{name: name, glyph: glyph, color: color, toggle: toggle}
	for _, entry := range entries {
		source.entries = append(source.entries, Entry{
			Key:         entry.Key,
			Description: firstNonEmpty(entry.Description, entry.Command, entry.Key),
			Kind:        KindManual,
			Command:     entry.Command,
		})
	}
	if len(source.entries) == 0 {
		source.problem = fmt.Sprintf("%s is empty; add {\"key\": \"…\", \"description\": \"…\", \"command\": \"…\"} entries to it", configKey)
	}
	return source
}

// buildDiscovered maps every action of every installed plugin to one entry keyed
// enter: the guide delegates to "herdr plugin action invoke <plugin>.<action>".
func buildDiscovered(actions []herdrmock.Action, err error) Source {
	source := group{name: NameDiscovered, glyph: glyphDiscovered, color: colorDiscovered, toggle: "which.sources.discovered"}
	if err != nil {
		source.problem = fmt.Sprintf("herdr did not answer plugin list: %v", err)
		return source
	}

	for _, action := range actions {
		actionID := action.Plugin + "." + action.ID
		source.entries = append(source.entries, Entry{
			Key:         "enter",
			Description: firstNonEmpty(action.Title, action.Description, action.ID),
			Kind:        KindAction,
			Command:     "herdr plugin action invoke " + actionID,
			ActionID:    actionID,
			Plugin:      action.Plugin,
		})
	}
	sort.SliceStable(source.entries, func(left, right int) bool {
		a, b := source.entries[left], source.entries[right]
		switch {
		case a.Plugin != b.Plugin:
			return a.Plugin < b.Plugin
		case a.Description != b.Description:
			return a.Description < b.Description
		}
		return a.ActionID < b.ActionID
	})
	if len(source.entries) == 0 {
		source.problem = "no installed plugin declares an action"
	}
	return source
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
