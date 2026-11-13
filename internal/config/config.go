// Package config reads and writes the plugin config file, the document that
// carries S2's everything-toggles rule: every tool and every which source is
// false until the user says otherwise.
//
// The file is JSON, at $HERDR_PLUGIN_CONFIG_DIR/config.json when herdr supplies
// it and otherwise under the herdr plugin config directory. Unknown keys are
// kept on save, so a config written by a newer build survives a round trip
// through this one.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The tools this container ships. A tool's name is its key under "tools".
const (
	ToolWhich   = "which"
	ToolPlugins = "plugins"
)

// The sources the which guide can render, in catalog order. A source's name is
// its key under "which.sources".
const (
	SourceHS         = "hs"
	SourceGentle     = "gentle"
	SourceCustom     = "custom"
	SourceDiscovered = "discovered"
)

var (
	toolNames   = []string{ToolWhich, ToolPlugins}
	sourceNames = []string{SourceHS, SourceGentle, SourceCustom, SourceDiscovered}
)

// Tools returns the tool names this build knows, in catalog order.
func Tools() []string { return append([]string(nil), toolNames...) }

// Sources returns the which source names this build knows, in catalog order.
func Sources() []string { return append([]string(nil), sourceNames...) }

// Config is the documented part of the config file.
type Config struct {
	Tools map[string]bool
	Which Which
}

// Which holds the toggles of the keybinding guide.
type Which struct {
	Sources map[string]bool
}

// Default is the config a missing file means: every tool off, every source off.
func Default() Config {
	return Config{Tools: toggles(toolNames), Which: Which{Sources: toggles(sourceNames)}}
}

func toggles(names []string) map[string]bool {
	values := make(map[string]bool, len(names))
	for _, name := range names {
		values[name] = false
	}
	return values
}

// ToolEnabled reports whether the user turned a tool on. An unknown or missing
// name is off.
func (c Config) ToolEnabled(name string) bool { return c.Tools[name] }

// SourceEnabled reports whether the user turned a which source on. An unknown or
// missing name is off.
func (c Config) SourceEnabled(name string) bool { return c.Which.Sources[name] }

// Document is a loaded config file. The raw maps keep keys this version does not
// know, so writing the file back never drops another version's setting.
type Document struct {
	Config  Config
	Present bool
	top     map[string]json.RawMessage
	tools   map[string]json.RawMessage
	which   map[string]json.RawMessage
	sources map[string]json.RawMessage
}

// Path resolves the plugin config file: HERDR_PLUGIN_CONFIG_DIR/config.json when
// herdr supplies it, otherwise
// <XDG_CONFIG_HOME>/herdr/plugins/config/herdr-utils/config.json, otherwise the
// same path under $HOME/.config.
func Path(getenv func(string) string) (string, error) {
	if dir := getenv("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	base := getenv("XDG_CONFIG_HOME")
	if base == "" {
		home := getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("cannot locate the plugin config: HERDR_PLUGIN_CONFIG_DIR, XDG_CONFIG_HOME and HOME are all unset")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "herdr", "plugins", "config", "herdr-utils", "config.json"), nil
}

// Load reads the config. A missing file is not an error: it means every toggle is
// off, which is the documented default. A malformed file is an error the caller
// reports; the startup gate treats it as every tool off.
func Load(getenv func(string) string) (Document, error) {
	path, err := Path(getenv)
	if err != nil {
		return Document{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Document{Config: Default()}, nil
		}
		return Document{}, fmt.Errorf("read %s: %w", path, err)
	}
	return parseConfig(data, path)
}

func parseConfig(data []byte, path string) (Document, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return Document{}, fmt.Errorf("parse %s: %w", path, err)
	}
	document := Document{Present: true, Config: Default(), top: top}

	if raw, ok := top["tools"]; ok {
		if err := json.Unmarshal(raw, &document.tools); err != nil {
			return Document{}, fmt.Errorf("parse %s: tools must be a JSON object: %w", path, err)
		}
		var wire map[string]*bool
		if err := json.Unmarshal(raw, &wire); err != nil {
			return Document{}, fmt.Errorf("parse %s: every entry of tools must be true or false: %w", path, err)
		}
		for _, name := range toolNames {
			if value, ok := wire[name]; ok && value != nil {
				document.Config.Tools[name] = *value
			}
		}
	}

	raw, ok := top["which"]
	if !ok {
		return document, nil
	}
	if err := json.Unmarshal(raw, &document.which); err != nil {
		return Document{}, fmt.Errorf("parse %s: which must be a JSON object: %w", path, err)
	}
	raw, ok = document.which["sources"]
	if !ok {
		return document, nil
	}
	if err := json.Unmarshal(raw, &document.sources); err != nil {
		return Document{}, fmt.Errorf("parse %s: which.sources must be a JSON object: %w", path, err)
	}
	var wire map[string]*bool
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Document{}, fmt.Errorf("parse %s: every entry of which.sources must be true or false: %w", path, err)
	}
	for _, name := range sourceNames {
		if value, ok := wire[name]; ok && value != nil {
			document.Config.Which.Sources[name] = *value
		}
	}
	return document, nil
}

// Save writes the document back, recording every known toggle and merging it into
// the raw JSON it was loaded from.
func Save(path string, document Document) error {
	top := document.top
	if top == nil {
		top = map[string]json.RawMessage{}
	}
	tools := document.tools
	if tools == nil {
		tools = map[string]json.RawMessage{}
	}
	which := document.which
	if which == nil {
		which = map[string]json.RawMessage{}
	}
	sources := document.sources
	if sources == nil {
		sources = map[string]json.RawMessage{}
	}

	for _, name := range toolNames {
		encoded, err := json.Marshal(document.Config.ToolEnabled(name))
		if err != nil {
			return err
		}
		tools[name] = encoded
	}
	for _, name := range sourceNames {
		encoded, err := json.Marshal(document.Config.SourceEnabled(name))
		if err != nil {
			return err
		}
		sources[name] = encoded
	}

	encodedSources, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return fmt.Errorf("encode which.sources: %w", err)
	}
	which["sources"] = encodedSources
	encodedWhich, err := json.MarshalIndent(which, "", "  ")
	if err != nil {
		return fmt.Errorf("encode which: %w", err)
	}
	encodedTools, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tools: %w", err)
	}
	top["tools"] = encodedTools
	top["which"] = encodedWhich

	data, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create the config directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
