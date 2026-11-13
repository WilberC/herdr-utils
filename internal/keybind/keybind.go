// Package keybind keeps the herdr-utils.which action bound in the user's herdr
// config. herdr's plugin manifest cannot declare a keybinding, so the plugin
// provisions its own: a fenced [[keys.command]] block in config.toml, written
// at startup and maintained by the "hu keys" command.
//
// The block is fenced so the plugin owns exactly its lines: ensure replaces
// anything between the markers (a user edit inside the fence is theirs to keep
// — ensure never rewrites it while the command still matches), and remove
// strips the fence and nothing else. A binding the user wrote outside the
// fence is never touched; when it disagrees with the fenced key, ensure reports
// it and changes nothing.
package keybind

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The binding ensure writes, as the manifest and docs document it. prefix+w is
// herdr's default workspace picker, so the guide ships on prefix+i.
const (
	Key         = "prefix+i"
	commandName = "herdr-utils.which"
	description = "Keybinding guide"

	BeginMarker = "# --- herdr-utils: which (hu which) — begin"
	EndMarker   = "# --- herdr-utils: which (hu which) — end"
)

// Block is the exact fenced text ensure writes.
func Block() string {
	return strings.Join([]string{
		BeginMarker,
		"[[keys.command]]",
		fmt.Sprintf("key = %q", Key),
		`type = "plugin_action"`,
		fmt.Sprintf("command = %q", commandName),
		fmt.Sprintf("description = %q", description),
		EndMarker,
	}, "\n")
}

// Path resolves the herdr config file: HERDR_CONFIG_PATH when herdr supplies
// it, otherwise ~/.config/herdr/config.toml.
func Path(getenv func(string) string) string {
	if path := getenv("HERDR_CONFIG_PATH"); path != "" {
		return path
	}
	return filepath.Join(getenv("HOME"), ".config", "herdr", "config.toml")
}

// fenced extracts the text between the markers and reports whether both are
// present.
func fenced(text string) (string, bool) {
	start := strings.Index(text, BeginMarker)
	if start < 0 {
		return "", false
	}
	offset := start + len(BeginMarker)
	end := strings.Index(text[offset:], EndMarker)
	if end < 0 {
		return "", false
	}
	return text[offset : offset+end], true
}

// Parse reports the state of the which binding in a config document:
// the fenced key when the fence carries one, an outside key when a
// non-fenced [[keys.command]] binds the action, and whether the block
// already matches what this build ships.
type State struct {
	FencedKey  string
	OutsideKey string
	Current    bool
}

func Parse(text string) State {
	state := State{}
	if inner, ok := fenced(text); ok {
		state.FencedKey = blockKey(inner)
		start := strings.Index(text, BeginMarker)
		end := strings.Index(text[start:], EndMarker) + start + len(EndMarker)
		text = text[:start] + text[end:]
	}
	for _, block := range strings.Split(text, "[[keys.command]]")[1:] {
		if entryKey(block) == commandName {
			if key := blockKey(block); key != "" {
				state.OutsideKey = key
			}
		}
	}
	if state.FencedKey == Key {
		state.Current = true
	}
	return state
}

// tomlValue reads the value of a `name = "value"` line.
func tomlValue(line string) string {
	_, value, found := strings.Cut(line, "=")
	if !found {
		return ""
	}
	return strings.Trim(strings.TrimSpace(value), `"`)
}

// entryKey reads the command = value of one [[keys.command]] block body.
func entryKey(block string) string {
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "command") {
			return tomlValue(line)
		}
	}
	return ""
}

// blockKey reads the key = value of one [[keys.command]] block body.
func blockKey(block string) string {
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "key") {
			return tomlValue(line)
		}
	}
	return ""
}

// Ensure adds or repairs the fenced binding block in the given document and
// returns the new document and what happened. It never touches an outside
// binding: when one exists with a different key, the result carries its
// advice and the document comes back unchanged.
type Result struct {
	Changed bool
	Wrote   string
	Reason  string
	Manual  string
}

func Ensure(text string) (string, Result) {
	state := Parse(text)
	block := Block()

	if state.Current {
		return text, Result{Reason: "current"}
	}
	if state.OutsideKey != "" {
		if state.OutsideKey == Key {
			return text, Result{Reason: "current"}
		}
		return text, Result{
			Reason: "outside",
			Manual: fmt.Sprintf(
				"a keybinding outside the herdr-utils fence binds %s on %s; move it or delete it, then run hu keys install again",
				commandName, state.OutsideKey),
		}
	}

	if _, ok := fenced(text); ok {
		return replaceFence(text, block), Result{Changed: true, Wrote: block, Reason: "updated"}
	}
	return appendBlock(text, block), Result{Changed: true, Wrote: block, Reason: "added"}
}

func replaceFence(text, block string) string {
	start := strings.Index(text, BeginMarker)
	end := strings.Index(text[start:], EndMarker) + start + len(EndMarker)
	return text[:start] + block + text[end:]
}

func appendBlock(text, block string) string {
	if !strings.HasSuffix(text, "\n") && text != "" {
		text += "\n"
	}
	if text != "" {
		text += "\n"
	}
	return text + block + "\n"
}

// Remove strips the fenced block and nothing else. A binding outside the fence
// stays; the result says so.
func Remove(text string) (string, Result) {
	inner, ok := fenced(text)
	if !ok {
		return text, Result{Reason: "absent"}
	}
	start := strings.Index(text, BeginMarker)
	end := strings.Index(text[start:], EndMarker) + start + len(EndMarker)
	trimmed := text[:start] + text[end:]
	trimmed = strings.TrimLeft(trimmed, "\n")
	trimmed = strings.TrimSuffix(trimmed, "\n\n")
	if trimmed != "" && !strings.HasSuffix(trimmed, "\n") {
		trimmed += "\n"
	}
	result := Result{Changed: true, Reason: "removed"}
	if key := blockKey(inner); key != "" && blockKey(inner) != "" {
		if state := Parse(text); state.OutsideKey != "" {
			result.Manual = fmt.Sprintf("a keybinding outside the fence still binds %s on %s", commandName, state.OutsideKey)
		}
	}
	return trimmed, result
}

// EnsureFile applies Ensure to the config file Path(getenv) names. A missing
// file is left missing: a machine without a herdr config has no herdr running
// this plugin either.
func EnsureFile(getenv func(string) string) (Result, error) {
	path := Path(getenv)
	text, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Result{Reason: "no-config"}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	updated, result := Ensure(string(text))
	if !result.Changed {
		return result, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return Result{}, fmt.Errorf("cannot write %s: %w", path, err)
	}
	result.Wrote = path
	return result, nil
}

// RemoveFile applies Remove to the config file Path(getenv) names.
func RemoveFile(getenv func(string) string) (Result, error) {
	path := Path(getenv)
	text, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Result{Reason: "no-config"}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	updated, result := Remove(string(text))
	if !result.Changed {
		return result, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return Result{}, fmt.Errorf("cannot write %s: %w", path, err)
	}
	result.Wrote = path
	return result, nil
}
