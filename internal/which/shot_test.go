package which

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/WilberC/herdr-utils/internal/config"
)

// The screenshot harness: it renders a realistic guide at three terminal sizes
// plus a filtered and a LIST state, and writes the ANSI to a directory, so the
// look can be inspected (freeze) and diffed before and after a redesign. It runs
// only when HU_WHICH_SHOT_DIR names a directory, so go test ./... skips it.
//
//	HU_WHICH_SHOT_DIR=/tmp/hu-which-shots HU_WHICH_SHOT_PREFIX=before \
//	  go test ./internal/which -run TestShotHarness -count=1
func TestShotHarness(t *testing.T) {
	dir := os.Getenv("HU_WHICH_SHOT_DIR")
	if dir == "" {
		t.Skip("set HU_WHICH_SHOT_DIR to write the screenshot ANSI files")
	}
	prefix := os.Getenv("HU_WHICH_SHOT_PREFIX")
	if prefix == "" {
		prefix = "shot"
	}
	before := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(before) })

	cases := []struct {
		name   string
		width  int
		height int
		setup  func(*Model)
	}{
		{name: "180", width: 180, height: 50},
		{name: "140", width: 140, height: 40},
		{name: "80", width: 80, height: 24},
		{name: "60", width: 60, height: 20},
		{name: "filtered", width: 180, height: 50, setup: func(m *Model) {
			m.Update(typeKey("prefix+p"))
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}},
		{name: "list-140", width: 140, height: 40, setup: func(m *Model) {
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}},
		{name: "list-80", width: 80, height: 24, setup: func(m *Model) {
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}},
		{name: "list", width: 180, height: 50, setup: func(m *Model) {
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}},
		{name: "nomatch", width: 140, height: 40, setup: func(m *Model) {
			m.Update(typeKey("no-such-binding"))
		}},
		{name: "empty", width: 140, height: 40, setup: func(m *Model) {
			m.guide = Guide{ConfigPath: "/home/u/.config/herdr/plugins/config/herdr-utils/config.json"}
			m.cursor = 0
		}},
	}

	for _, shot := range cases {
		m := New(shotGuide())
		m.Update(tea.WindowSizeMsg{Width: shot.width, Height: shot.height})
		if shot.setup != nil {
			shot.setup(m)
		}
		path := filepath.Join(dir, fmt.Sprintf("%s-%s.ansi", prefix, shot.name))
		if err := os.WriteFile(path, []byte(m.View()), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// shotGuide is realistic sample data: a full HS column, a small GENTLE and
// CUSTOM, and discovered actions, which declare no key of their own.
func shotGuide() Guide {
	hs := []Entry{
		{Key: "prefix+k", Description: "Command bar", Kind: KindPluginAction, Command: "herdr-bar.open", ActionID: "herdr-bar.open"},
		{Key: "prefix+p", Description: "Plugin manager", Kind: KindPluginAction, Command: "herdr.plugin-manager.open", ActionID: "herdr.plugin-manager.open"},
		{Key: "prefix+i", Description: "Keybinding guide", Kind: KindPluginAction, Command: "herdr-utils.which", ActionID: "herdr-utils.which"},
		{Key: "prefix+,", Description: "Plugin settings", Kind: KindPluginAction, Command: "herdr-utils.settings", ActionID: "herdr-utils.settings"},
		{Key: "prefix+.", Description: "Edit the herdr config", Kind: KindPopup, Command: "exec $EDITOR ~/.config/herdr/config.toml"},
		{Key: "prefix+g", Description: "Git status in a popup", Kind: KindPopup, Command: "exec lazygit"},
		{Key: "prefix+r", Description: "Restart the plugin runtime", Kind: KindShell, Command: "herdr plugin restart"},
		{Key: "prefix+v", Description: "Split the pane vertically", Kind: KindShell, Command: "herdr pane split --vertical"},
		{Key: "prefix+s", Description: "Split the pane horizontally", Kind: KindShell, Command: "herdr pane split --horizontal"},
		{Key: "prefix+z", Description: "Zoom the current pane", Kind: KindShell, Command: "herdr pane zoom"},
		{Key: "prefix+n", Description: "New workspace", Kind: KindShell, Command: "herdr workspace new"},
		{Key: "prefix+o", Description: "Open the current file in the editor", Kind: KindPopup, Command: "exec fish -c 'nvim .'"},
		{Key: "prefix+d", Description: "Dashboard", Kind: KindPopup, Command: "exec herdr-suite dashboard"},
		{Key: "prefix+e", Description: "File explorer", Kind: KindPopup, Command: "exec yazi"},
		{Key: "prefix+l", Description: "Plugin logs", Kind: KindShell, Command: "herdr plugin log herdr-utils --follow"},
		{Key: "prefix+m", Description: "Resource monitor", Kind: KindPopup, Command: "exec btop"},
		{Key: "prefix+t", Description: "Terminal presets", Kind: KindShell, Command: "herdr-utils.presets"},
		{Key: "prefix+b", Description: "Background jobs", Kind: KindShell, Command: "herdr job list"},
		{Key: "prefix+?", Description: "Every binding at once", Kind: KindPopup, Command: "exec hu which"},
		{Key: "ctrl+shift+u", Description: "Open the limits pane", Kind: KindPopup, Command: "exec fish -c limits"},
	}
	gentle := []Entry{
		{Key: "ctrl+g", Description: "Lazygit", Kind: KindManual, Command: "fish -c lazygit"},
		{Key: "ctrl+f", Description: "Fuzzy find files", Kind: KindManual, Command: "fish -c fzf-files"},
		{Key: "ctrl+n", Description: "New note in the vault", Kind: KindManual, Command: "fish -c note-new"},
		{Key: "alt+g", Description: "Git graph", Kind: KindManual, Command: "fish -c git-graph"},
	}
	custom := []Entry{
		{Key: "prefix+y", Description: "YouTube music", Kind: KindManual, Command: "fish -c ytmusic"},
	}
	discovered := []Entry{
		{Key: "enter", Description: "Open the bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open", Plugin: "herdr-bar"},
		{Key: "enter", Description: "Open the plugin manager", Kind: KindAction, Command: "herdr plugin action invoke herdr-suite.plugins", ActionID: "herdr-suite.plugins", Plugin: "herdr-suite"},
		{Key: "enter", Description: "Restore the last session", Kind: KindAction, Command: "herdr plugin action invoke herdr-suite.restore", ActionID: "herdr-suite.restore", Plugin: "herdr-suite"},
		{Key: "enter", Description: "Toggle the status line", Kind: KindAction, Command: "herdr plugin action invoke herdr-statusline.toggle", ActionID: "herdr-statusline.toggle", Plugin: "herdr-statusline"},
		{Key: "enter", Description: "Run the health check", Kind: KindAction, Command: "herdr plugin action invoke herdr-utils.health", ActionID: "herdr-utils.health", Plugin: "herdr-utils"},
		{Key: "enter", Description: "Capture the current pane", Kind: KindAction, Command: "herdr plugin action invoke herdr-shots.capture", ActionID: "herdr-shots.capture", Plugin: "herdr-shots"},
	}

	return Guide{
		ConfigPath: "/home/u/.config/herdr/plugins/config/herdr-utils/config.json",
		View:       config.ViewKeys,
		Sources: []Source{
			group{name: NameHS, glyph: glyphHS, color: colorHS, toggle: "which.sources.hs", entries: hs},
			group{name: NameGentle, glyph: glyphGentle, color: colorGentle, toggle: "which.sources.gentle", entries: gentle},
			group{name: NameCustom, glyph: glyphCustom, color: colorCustom, toggle: "which.sources.custom", entries: custom},
			group{name: NameDiscovered, glyph: glyphDiscovered, color: colorDiscovered, toggle: "which.sources.discovered", entries: discovered},
		},
	}
}
