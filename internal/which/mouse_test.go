package which

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WilberC/tui-kit/chip"
	"github.com/WilberC/tui-kit/clipboard"
	"github.com/WilberC/tui-kit/text"
	"github.com/WilberC/tui-kit/theme"
	"github.com/WilberC/tui-kit/zone"

	"github.com/WilberC/herdr-utils/internal/config"
)

// clock is the test's time source: a double click is judged by the moment the
// model saw, which a test moves by hand so it never sleeps.
type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

// mouse builds the message Bubbletea sends: a press, a motion or a wheel tick at
// one pane cell.
func mouse(action tea.MouseAction, button tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseMsg{Action: action, Button: button, X: x, Y: y}
}

func pressAt(m *Model, x, y int) tea.Cmd {
	return press(m, mouse(tea.MouseActionPress, tea.MouseButtonLeft, x, y))
}

func hoverAt(m *Model, x, y int) tea.Cmd {
	return press(m, mouse(tea.MouseActionMotion, tea.MouseButtonNone, x, y))
}

func wheel(m *Model, button tea.MouseButton) tea.Cmd {
	return press(m, mouse(tea.MouseActionPress, button, 0, 0))
}

// zoneIndex reads an entry zone id back into the flat index it names, so a test
// can say which entry a cell belongs to without knowing the layout.
func zoneIndex(t *testing.T, id string) int {
	t.Helper()
	for _, prefix := range []string{rowZonePrefix, chipZonePrefix} {
		if rest, found := strings.CutPrefix(id, prefix); found {
			index := 0
			for _, digit := range rest {
				if digit < '0' || digit > '9' {
					t.Fatalf("zone id %q does not end in an index", id)
				}
				index = index*10 + int(digit-'0')
			}
			return index
		}
	}
	t.Fatalf("zone id %q is not an entry", id)
	return -1
}

// centre is the cell a test clicks: the middle of a zone a previous render
// recorded.
func centre(rect zone.Rect) (int, int) {
	return rect.X + rect.W/2, rect.Y + rect.H/2
}

// TestAClickSelectsTheEntryUnderThePointer pins S8's first rule: a click on a row
// or on its chip selects that entry and nothing else — in both views, and in a
// centred frame at a wide and a narrow pane, so the translation from pane cells
// to the entry is what is judged.
func TestAClickSelectsTheEntryUnderThePointer(t *testing.T) {
	for _, view := range []string{config.ViewKeys, config.ViewList} {
		for _, window := range []struct{ width, height int }{{180, 50}, {80, 24}} {
			m := size(New(shotGuide()), window.width, window.height)
			if view == config.ViewList {
				press(m, tea.KeyMsg{Type: tea.KeyCtrlT})
			}
			m.View()

			checked := 0
			for _, record := range m.zones.Zones() {
				if !strings.HasPrefix(record.ID, rowZonePrefix) {
					continue
				}
				index := zoneIndex(t, record.ID)
				m.cursor = m.shown() - 1
				if m.cursor == index {
					m.cursor = 0
				}

				x, y := centre(record.Rect)
				pressAt(m, x, y)

				if m.cursor != index {
					t.Fatalf("%s at %dx%d: a click on %s selected entry %d, want %d",
						view, window.width, window.height, record.ID, m.cursor, index)
				}
				checked++
			}
			if checked == 0 {
				t.Fatalf("%s at %dx%d: the render recorded no row to click", view, window.width, window.height)
			}

			// The chip slab of a row is the same entry: a click on it selects too.
			m.View()
			zone, ok := m.zones.Find(chipZone(1))
			if !ok {
				t.Fatalf("%s at %dx%d: the render recorded no chip zone for entry 1", view, window.width, window.height)
			}
			m.cursor = m.shown() - 1
			x, y := centre(zone)
			pressAt(m, x, y)
			if m.cursor != 1 {
				t.Fatalf("%s at %dx%d: a click on the chip slab selected entry %d, want 1",
					view, window.width, window.height, m.cursor)
			}
		}
	}
}

// TestASingleClickNeverRunsTheSelection pins the safety rule: one click only
// selects. A second click on the same entry inside the double-click window is
// what runs it — the same action as enter — and a slower second click starts a
// new single click instead.
func TestASingleClickNeverRunsTheSelection(t *testing.T) {
	base := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	t.Run("a discovered action runs on the double click", func(t *testing.T) {
		var invoked []string
		guide := testGuide(group{name: NameDiscovered, entries: []Entry{
			{Key: "enter", Description: "Open the bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open"},
			{Key: "enter", Description: "Restore the session", Kind: KindAction, Command: "herdr plugin action invoke herdr-suite.restore", ActionID: "herdr-suite.restore"},
		}})
		guide.Invoke = func(_ context.Context, actionID string) error {
			invoked = append(invoked, actionID)
			return nil
		}

		m := size(New(guide), 100, 30)
		now := &clock{at: base}
		m.now = now.now
		m.View()

		first, ok := m.zones.Find(rowZone(1))
		if !ok {
			t.Fatalf("the render recorded no zone for entry 1")
		}
		x, y := centre(first)

		pressAt(m, x, y)
		if len(invoked) != 0 {
			t.Fatalf("a single click ran %v, want it to only select", invoked)
		}
		if m.cursor != 1 || m.quit {
			t.Fatalf("cursor = %d (quit %v), want the click to select entry 1 and stay", m.cursor, m.quit)
		}

		now.at = base.Add(300 * time.Millisecond)
		cmd := pressAt(m, x, y)

		if len(invoked) != 1 || invoked[0] != "herdr-suite.restore" {
			t.Fatalf("invoked = %v, want the second entry's action on the double click", invoked)
		}
		if !m.quit || !isQuit(cmd) {
			t.Fatalf("quit = %v (cmd %v), want the guide to close like enter does", m.quit, cmd)
		}
	})

	t.Run("a bound key shows its command on the double click", func(t *testing.T) {
		m := size(New(testGuide(group{name: NameHS, entries: hsEntries()})), 100, 30)
		now := &clock{at: base}
		m.now = now.now
		m.View()

		first, ok := m.zones.Find(rowZone(0))
		if !ok {
			t.Fatal("the render recorded no zone for the first entry")
		}
		x, y := centre(first)

		pressAt(m, x, y)
		if m.footer != "" {
			t.Fatalf("footer = %q, want a single click to say nothing", m.footer)
		}

		now.at = base.Add(200 * time.Millisecond)
		pressAt(m, x, y)

		if !strings.Contains(m.footer, "herdr-bar.open") {
			t.Fatalf("footer = %q, want the command the double click asked for", m.footer)
		}
		if m.quit {
			t.Fatalf("the guide closed on a bound key, want the footer confirmation")
		}
	})

	t.Run("a slow second click is a first click", func(t *testing.T) {
		var invoked []string
		guide := testGuide(group{name: NameDiscovered, entries: []Entry{
			{Key: "enter", Description: "Open the bar", Kind: KindAction, ActionID: "herdr-bar.open"},
		}})
		guide.Invoke = func(_ context.Context, actionID string) error {
			invoked = append(invoked, actionID)
			return nil
		}

		m := size(New(guide), 100, 30)
		now := &clock{at: base}
		m.now = now.now
		m.View()
		zone, _ := m.zones.Find(rowZone(0))
		x, y := centre(zone)

		pressAt(m, x, y)
		now.at = base.Add(401 * time.Millisecond)
		pressAt(m, x, y)

		if len(invoked) != 0 {
			t.Fatalf("invoked = %v, want two clicks %v apart to not run anything", invoked, 401*time.Millisecond)
		}
	})
}

// TestHoverTintsTheRowFaintlyAndTheSelectionWins pins S8's hover rule: the entry
// under the pointer wears the hover tint end to end, with no accent edge, and a
// hover over the selected row leaves the selection alone.
func TestHoverTintsTheRowFaintlyAndTheSelectionWins(t *testing.T) {
	truecolor(t)

	m := size(New(shotGuide()), 140, 40)
	m.View()

	hovered, ok := m.zones.Find(rowZone(1))
	if !ok {
		t.Fatal("the render recorded no zone for entry 1")
	}
	hoverX, hoverY := centre(hovered)
	hoverAt(m, hoverX, hoverY)

	view := m.View()
	row := lineWith(view, "Plugin manager")
	if row == "" {
		t.Fatalf("the hovered entry has no row:\n%s", view)
	}
	if !strings.Contains(row, paintOf(theme.HoverBG)) {
		t.Fatalf("the hovered row is not tinted:\n%q", row)
	}
	if strings.Contains(row, paintOf(theme.SelectionEdge)+text.SelectMark) {
		t.Fatalf("the hovered row carries the selection's accent edge:\n%q", row)
	}

	selected, ok := m.zones.Find(rowZone(0))
	if !ok {
		t.Fatal("the render recorded no zone for the selected entry")
	}
	selectX, selectY := centre(selected)
	hoverAt(m, selectX, selectY)

	view = m.View()
	row = lineWith(view, "Command bar")
	if !strings.Contains(row, paintOf(theme.SelectionEdge)+text.SelectMark) {
		t.Fatalf("hovering the selected row dropped its accent edge:\n%q", row)
	}
	if strings.Contains(row, paintOf(theme.HoverBG)) {
		t.Fatalf("the hover tint competed with the selection:\n%q", row)
	}

	// The LIST reserves the source column on the row's edge, so the pointer's tint
	// must survive it: the air cell after the source is the row's own background, so
	// the tint reaches the row's last cell instead of stopping inside the source.
	list := size(New(shotGuide()), 140, 40)
	press(list, tea.KeyMsg{Type: tea.KeyCtrlT})
	list.View()
	hovered, ok = list.zones.Find(rowZone(1))
	if !ok {
		t.Fatal("the LIST render recorded no zone for entry 1")
	}
	hoverX, hoverY = centre(hovered)
	hoverAt(list, hoverX, hoverY)

	listRow := lineWith(list.View(), "Plugin manager")
	if listRow == "" {
		t.Fatal("the hovered LIST entry has no row")
	}
	after := strings.TrimPrefix(listRow[strings.Index(listRow, "HS")+len("HS"):], "\x1b[0m")
	if !strings.HasPrefix(after, paintOf(theme.HoverBG)) {
		t.Fatalf("the source column's fill is not the hover tint, so the tint stops at the source:\n%q", listRow)
	}
	if tail := strings.Trim(plainText(after), " │"); tail != "" {
		t.Fatalf("the row draws %q after the source column, so the tint does not reach its edge", tail)
	}
}

// TestTheWheelMovesTheSelectionAndPages pins S8's wheel rule: the wheel moves the
// selection one entry at a time in both views, and the body pages when the
// selection crosses a page.
func TestTheWheelMovesTheSelectionAndPages(t *testing.T) {
	for _, view := range []string{config.ViewKeys, config.ViewList} {
		m := size(New(shotGuide()), 80, 24)
		if view == config.ViewList {
			press(m, tea.KeyMsg{Type: tea.KeyCtrlT})
		}
		m.View()

		wheel(m, tea.MouseButtonWheelDown)
		if m.cursor != 1 {
			t.Fatalf("%s: the wheel down moved the selection to %d, want 1", view, m.cursor)
		}
		wheel(m, tea.MouseButtonWheelUp)
		if m.cursor != 0 {
			t.Fatalf("%s: the wheel up moved the selection to %d, want 0", view, m.cursor)
		}

		for range m.shown() {
			wheel(m, tea.MouseButtonWheelDown)
		}
		if want := m.shown() - 1; m.cursor != want {
			t.Fatalf("%s: the wheel stopped at %d, want the last entry %d", view, m.cursor, want)
		}
		page, pages := pageOf(t, plainText(m.View()))
		if pages < 2 {
			t.Fatalf("%s: the body does not page at 80x24, so the wheel was not tested", view)
		}
		if page != pages {
			t.Fatalf("%s: the body shows page %d of %d on the last entry, want the last page", view, page, pages)
		}

		for range m.shown() {
			wheel(m, tea.MouseButtonWheelUp)
		}
		if page, _ := pageOf(t, plainText(m.View())); page != 1 {
			t.Fatalf("%s: the body still shows page %d at the first entry", view, page)
		}
	}
}

// TestAClickOnThePagingChipIsThePagingKey pins the parity rule: a click on the
// paging chip draws exactly what chip.PageKey draws, so the mouse is never a
// second interface.
func TestAClickOnThePagingChipIsThePagingKey(t *testing.T) {
	clicked := size(New(shotGuide()), 80, 24)
	clicked.View()
	chipZone, ok := clicked.zones.Find(chip.PageZone)
	if !ok {
		t.Fatal("a paged bar recorded no paging chip")
	}
	if !strings.Contains(plainText(clicked.View()), "more 1/") {
		t.Fatalf("the bar is not on its first page:\n%s", clicked.View())
	}

	x, y := centre(chipZone)
	pressAt(clicked, x, y)

	typed := size(New(shotGuide()), 80, 24)
	typed.View()
	press(typed, tea.KeyMsg{Type: tea.KeyTab})

	if got, want := plainText(clicked.View()), plainText(typed.View()); got != want {
		t.Fatalf("the paging chip draws something tab does not:\nclicked:\n%s\ntab:\n%s", got, want)
	}
}

// TestAClickOnTheHeaderSwitchesTheView pins the header's two names: a click on
// LIST opens the table and a click on KEYS returns, exactly what ctrl+t does,
// including the saved view.
func TestAClickOnTheHeaderSwitchesTheView(t *testing.T) {
	var saved []string
	guide := testGuide(group{name: NameHS, entries: hsEntries()})
	guide.Persist = func(view string) error {
		saved = append(saved, view)
		return nil
	}

	m := size(New(guide), 140, 40)
	m.View()

	listZone, ok := m.zones.Find(viewListZone)
	if !ok {
		t.Fatal("the header recorded no zone for LIST")
	}
	x, y := centre(listZone)
	pressAt(m, x, y)

	if m.Mode() != config.ViewList {
		t.Fatalf("Mode = %q, want the LIST view after a click on it", m.Mode())
	}
	if len(saved) != 1 || saved[0] != config.ViewList {
		t.Fatalf("saved = %v, want the clicked view recorded once", saved)
	}

	m.View()
	keysZone, ok := m.zones.Find(viewKeysZone)
	if !ok {
		t.Fatal("the header recorded no zone for KEYS")
	}
	x, y = centre(keysZone)
	pressAt(m, x, y)

	if m.Mode() != config.ViewKeys {
		t.Fatalf("Mode = %q, want the KEYS view after a click on it", m.Mode())
	}
	if len(saved) != 2 || saved[1] != config.ViewKeys {
		t.Fatalf("saved = %v, want both clicks recorded", saved)
	}
}

// TestCtrlYCopiesTheSelectionWithOSC52 pins S9: one key copies the one value the
// cursor is on — the command of a binding, the action id of a live action — with
// OSC 52, the detail line confirms it, and the key never types into the filter.
func TestCtrlYCopiesTheSelectionWithOSC52(t *testing.T) {
	cases := []struct {
		name  string
		entry Entry
		want  string
	}{
		{
			name:  "a binding copies its command",
			entry: Entry{Key: "prefix+k", Description: "Command bar", Kind: KindPluginAction, Command: "herdr-bar.open", ActionID: "herdr-bar.open"},
			want:  "herdr-bar.open",
		},
		{
			name:  "a manual entry copies its command",
			entry: Entry{Key: "ctrl+g", Description: "Lazygit", Kind: KindManual, Command: "fish -c lazygit"},
			want:  "fish -c lazygit",
		},
		{
			name:  "a live action copies its action id",
			entry: Entry{Key: "enter", Description: "Open the bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open"},
			want:  "herdr-bar.open",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			m := size(New(testGuide(group{name: NameHS, entries: []Entry{test.entry}})), 100, 30)
			m.out = &out
			m.confirm = 0
			before := m.shown()

			cmd := press(m, tea.KeyMsg{Type: tea.KeyCtrlY})
			written := messages(cmd)

			if got, want := out.String(), clipboard.OSC52(test.want); got != want {
				t.Fatalf("the copy key wrote %q, want the OSC 52 sequence for %q", got, want)
			}
			if m.query != "" || m.shown() != before {
				t.Fatalf("query = %q (shown %d), want the copy key to leave the filter alone", m.query, m.shown())
			}
			if !strings.Contains(m.footer, "copied") {
				t.Fatalf("footer = %q, want the detail line to confirm the copy", m.footer)
			}
			if view := plainText(m.View()); !strings.Contains(view, "copied") {
				t.Fatalf("the detail line does not show the confirmation:\n%s", view)
			}

			for _, msg := range written {
				if _, ok := msg.(copyExpired); ok {
					press(m, msg)
				}
			}
			if m.footer != "" {
				t.Fatalf("footer = %q, want the confirmation cleared after a moment", m.footer)
			}
		})
	}
}

// TestCtrlYWithNothingToCopySaysSo pins the empty case: an entry with no command
// copies nothing and never clears the user's clipboard.
func TestCtrlYWithNothingToCopySaysSo(t *testing.T) {
	var out bytes.Buffer
	m := size(New(testGuide(group{name: NameCustom, entries: []Entry{{Key: "prefix+t", Description: "TickTick", Kind: KindManual}}})), 100, 30)
	m.out = &out

	press(m, tea.KeyMsg{Type: tea.KeyCtrlY})

	if out.Len() != 0 {
		t.Fatalf("the copy key wrote %q, want nothing to copy", out.String())
	}
	if !strings.Contains(m.footer, "nothing to copy") {
		t.Fatalf("footer = %q, want an honest note", m.footer)
	}
}

// TestTheHintBarNamesTheCopyKey pins the discoverability rule: the key that
// copies is on the bar.
func TestTheHintBarNamesTheCopyKey(t *testing.T) {
	m := size(New(shotGuide()), 180, 50)

	if view := plainText(m.View()); !strings.Contains(view, "ctrl+y  copy") {
		t.Fatalf("the hint bar does not name the copy key:\n%s", view)
	}
}

// lineWith is the rendered line that carries text, with its escapes kept so a
// test can judge the roles it was painted with.
func lineWith(view, text string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return line
		}
	}
	return ""
}

// pageOf reads the page the header names, so a test does not hard-code how many
// pages a window has.
func pageOf(t *testing.T, view string) (int, int) {
	t.Helper()
	index := strings.Index(view, "page ")
	if index < 0 {
		t.Fatalf("the header names no page:\n%s", view)
	}
	rest := view[index+len("page "):]
	page, pages := 0, 0
	if _, err := fmt.Sscanf(rest, "%d / %d", &page, &pages); err != nil || page == 0 || pages == 0 {
		t.Fatalf("the header names no page position: %q", rest)
	}
	return page, pages
}

// messages runs a command and flattens the batch it returns, so a test can see
// every message the update loop would receive.
func messages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		out := make([]tea.Msg, 0, len(batch))
		for _, inner := range batch {
			out = append(out, messages(inner)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}
