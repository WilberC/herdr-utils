// Package tools is the gate behind the container's single [[startup]] hook. It
// decides what the plugin does at herdr server start and records the one hint S2
// allows while every tool is off.
package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/WilberC/herdr-utils/internal/config"
)

// Plan is what the startup hook does for a config.
type Plan string

const (
	// PlanNothing leaves the machine untouched and prints nothing.
	PlanNothing Plan = "nothing"
	// PlanHint prints the one-line reminder once per server socket.
	PlanHint Plan = "hint"
)

// Enabled names the tools the user turned on, in catalog order.
func Enabled(document config.Config) []string {
	names := make([]string, 0, len(config.Tools()))
	for _, name := range config.Tools() {
		if document.ToolEnabled(name) {
			names = append(names, name)
		}
	}
	return names
}

// Decide applies the gate: with every tool off the startup hook mentions the
// container once, and with any tool on there is nothing to start, so it stays
// silent.
func Decide(document config.Config) Plan {
	if len(Enabled(document)) == 0 {
		return PlanHint
	}
	return PlanNothing
}

const hintFileName = "startup-hint.json"

func stateDir(getenv func(string) string) (string, error) {
	if dir := getenv("HERDR_PLUGIN_STATE_DIR"); dir != "" {
		return dir, nil
	}
	base := getenv("XDG_STATE_HOME")
	if base == "" {
		home := getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("cannot locate the plugin state directory: HERDR_PLUGIN_STATE_DIR, XDG_STATE_HOME and HOME are all unset")
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "herdr", "plugins", "herdr-utils"), nil
}

// HintPath is the file recording which server sockets already saw the hint.
func HintPath(getenv func(string) string) (string, error) {
	dir, err := stateDir(getenv)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hintFileName), nil
}

// hintState keys a hinted server by its socket path, so a new server hints once
// and a restarted session stays quiet.
type hintState struct {
	Hinted map[string]string `json:"hinted"`
}

const defaultSocketKey = "default"

// TakeHint reports whether this is the first start recorded for the current
// server socket, and records it.
func TakeHint(getenv func(string) string, now time.Time) (bool, error) {
	path, err := HintPath(getenv)
	if err != nil {
		return false, err
	}
	key := getenv("HERDR_SOCKET_PATH")
	if key == "" {
		key = defaultSocketKey
	}

	state := hintState{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &state); err != nil {
			state = hintState{}
		}
	case os.IsNotExist(err):
	default:
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if state.Hinted == nil {
		state.Hinted = map[string]string{}
	}
	if _, seen := state.Hinted[key]; seen {
		return false, nil
	}
	state.Hinted[key] = now.UTC().Format(time.RFC3339)

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create the plugin state directory: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
