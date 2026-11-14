package which

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WilberC/tui-kit/chip"

	"github.com/WilberC/herdr-utils/internal/config"
)

func typeKey(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

func press(m *Model, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func testGuide(sources ...Source) Guide {
	return Guide{ConfigPath: "/home/u/.config/herdr/plugins/config/herdr-utils/config.json", Sources: sources}
}

func hsEntries() []Entry {
	return []Entry{
		{Key: "prefix+k", Description: "Command bar", Kind: KindPluginAction, Command: "herdr-bar.open", ActionID: "herdr-bar.open"},
		{Key: "ctrl+shift+u", Description: "Open limits pane", Kind: KindPopup, Command: "exec fish"},
		{Key: "prefix+p", Description: "Plugin manager", Kind: KindPluginAction, Command: "ray.plugin-manager.open"},
	}
}

// TestFilterMatchesKeysAndDescriptionsAndCounts pins the live filter: it matches
// across the key and the description, ignores case, and both the group header
// and the screen header report how many entries are shown.
func TestFilterMatchesKeysAndDescriptionsAndCounts(t *testing.T) {
	guide := testGuide(
		group{name: NameHS, entries: hsEntries()},
		group{name: NameCustom, entries: []Entry{{Key: "prefix+g", Description: "Git status", Kind: KindManual, Command: "lazygit"}}},
	)

	m := New(guide)
	if m.shown() != 4 || m.total() != 4 {
		t.Fatalf("shown/total = %d/%d, want 4/4", m.shown(), m.total())
	}
	if view := m.View(); !strings.Contains(view, "4 / 4 shown") || !strings.Contains(view, "HS 3") {
		t.Fatalf("unfiltered view does not report the counts:\n%s", view)
	}

	press(m, typeKey("PREFIX"))

	if m.shown() != 3 || m.total() != 4 {
		t.Fatalf("shown/total = %d/%d, want 3/4 after a case-insensitive key match", m.shown(), m.total())
	}
	view := m.View()
	for _, want := range []string{"3 / 4 shown", "HS 2 / 3", "CUSTOM 1 / 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filtered view does not contain %q:\n%s", want, view)
		}
	}

	press(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.shown() != 3 {
		t.Fatalf("shown = %d, want the shorter query to still match", m.shown())
	}
	press(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.shown() != 4 {
		t.Fatalf("shown = %d, want ctrl+u to clear the filter", m.shown())
	}
	press(m, typeKey("limits"))

	if m.shown() != 1 {
		t.Fatalf("shown = %d, want the description match only", m.shown())
	}
	if entry, ok := m.highlight(); !ok || entry.Key != "ctrl+shift+u" {
		t.Fatalf("highlight = %#v (ok %v), want the limits binding", entry, ok)
	}
	if view := m.View(); !strings.Contains(view, "HS 1 / 3") {
		t.Fatalf("view does not report a group with no match:\n%s", view)
	}
}

func TestTheFilterClampsTheCursorBackIntoTheMatches(t *testing.T) {
	m := New(testGuide(group{name: NameHS, entries: hsEntries()}))
	press(m, tea.KeyMsg{Type: tea.KeyDown})
	press(m, tea.KeyMsg{Type: tea.KeyDown})
	if entry, _ := m.highlight(); entry.Key != "prefix+p" {
		t.Fatalf("highlight = %q, want the last entry before filtering", entry.Key)
	}

	press(m, typeKey("command"))

	entry, ok := m.highlight()
	if !ok || entry.Key != "prefix+k" {
		t.Fatalf("highlight = %#v (ok %v), want the cursor clamped to the only match", entry, ok)
	}
}

// TestCtrlTCyclesTheViewAndSavesIt pins the KEYS/LIST toggle: the mode flips,
// the plugin config is told to save it, and each view orders the row the way its
// name promises.
func TestCtrlTCyclesTheViewAndSavesIt(t *testing.T) {
	var saved []string
	guide := testGuide(group{name: NameHS, entries: []Entry{
		{Key: "prefix+k", Description: "Command bar", Kind: KindPluginAction, Command: "herdr-bar.open"},
	}})
	guide.Persist = func(view string) error {
		saved = append(saved, view)
		return nil
	}

	m := New(guide)
	if m.Mode() != config.ViewKeys {
		t.Fatalf("Mode = %q, want %q", m.Mode(), config.ViewKeys)
	}
	keysView := m.View()
	if !strings.Contains(keysView, "KEYS") {
		t.Fatalf("default view is not the KEYS view:\n%s", keysView)
	}
	if strings.Index(keysView, "prefix+k") > strings.Index(keysView, "Command bar") {
		t.Fatalf("KEYS view does not put the key first:\n%s", keysView)
	}

	press(m, tea.KeyMsg{Type: tea.KeyCtrlT})

	if m.Mode() != config.ViewList {
		t.Fatalf("Mode = %q, want %q after ctrl+t", m.Mode(), config.ViewList)
	}
	if !reflect.DeepEqual(saved, []string{config.ViewList}) {
		t.Fatalf("saved = %v, want one save of %q", saved, config.ViewList)
	}
	listView := m.View()
	if !strings.Contains(listView, "LIST") {
		t.Fatalf("view is not the LIST view:\n%s", listView)
	}
	// LIST is the table view: the chip leads, the description follows it, and the
	// source closes the row in its own column.
	if strings.Index(listView, "prefix+k") > strings.Index(listView, "Command bar") {
		t.Fatalf("LIST view does not put the key chip first:\n%s", listView)
	}
	if strings.Index(listView, "Command bar") > strings.LastIndex(listView, "HS") {
		t.Fatalf("LIST view does not close the row with the source:\n%s", listView)
	}

	press(m, tea.KeyMsg{Type: tea.KeyCtrlT})

	if m.Mode() != config.ViewKeys {
		t.Fatalf("Mode = %q, want %q after the second ctrl+t", m.Mode(), config.ViewKeys)
	}
	if !reflect.DeepEqual(saved, []string{config.ViewList, config.ViewKeys}) {
		t.Fatalf("saved = %v, want both views saved", saved)
	}
}

func TestAFailedViewSaveIsReportedInTheFooter(t *testing.T) {
	guide := testGuide(group{name: NameHS, entries: hsEntries()})
	guide.Persist = func(string) error { return errors.New("read-only file") }

	m := New(guide)
	press(m, tea.KeyMsg{Type: tea.KeyCtrlT})

	if !strings.Contains(m.footer, "read-only file") {
		t.Fatalf("footer = %q, want the save failure reported", m.footer)
	}
	if m.Mode() != config.ViewList {
		t.Fatalf("Mode = %q, want the view to switch even when the save fails", m.Mode())
	}
}

func TestEnterOnADiscoveredEntryRunsItAndCloses(t *testing.T) {
	var invoked []string
	guide := testGuide(group{name: NameDiscovered, entries: []Entry{
		{Key: "enter", Description: "Open bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open", Plugin: "herdr-bar"},
	}})
	guide.Invoke = func(ctx context.Context, actionID string) error {
		invoked = append(invoked, actionID)
		return nil
	}

	m := New(guide)
	cmd := press(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !reflect.DeepEqual(invoked, []string{"herdr-bar.open"}) {
		t.Fatalf("invoked = %v, want the highlighted action", invoked)
	}
	if !m.quit || !isQuit(cmd) {
		t.Fatalf("quit = %v (cmd %v), want the guide to close after running the action", m.quit, cmd)
	}
	if !strings.Contains(m.footer, "herdr-bar.open") {
		t.Fatalf("footer = %q, want the action it ran", m.footer)
	}
}

func TestAHerdrRefusalKeepsTheGuideOpen(t *testing.T) {
	guide := testGuide(group{name: NameDiscovered, entries: []Entry{
		{Key: "enter", Description: "Open bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open"},
	}})
	guide.Invoke = func(context.Context, string) error { return errors.New("no server running") }

	m := New(guide)
	cmd := press(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.quit || isQuit(cmd) {
		t.Fatal("the guide closed after a refused action, want it to stay open")
	}
	if !strings.Contains(m.footer, "no server running") {
		t.Fatalf("footer = %q, want herdr's failure", m.footer)
	}
}

// TestEnterOnABoundKeyShowsTheCommandWithoutRunningIt pins the v1 limit: a
// source entry is shown, never executed by hu.
func TestEnterOnABoundKeyShowsTheCommandWithoutRunningIt(t *testing.T) {
	invoked := 0
	guide := testGuide(group{name: NameHS, entries: []Entry{
		{Key: "prefix+k", Description: "Command bar", Kind: KindPluginAction, Command: "herdr-bar.open", ActionID: "herdr-bar.open"},
		{Key: "ctrl+shift+u", Description: "Open limits pane", Kind: KindPopup, Command: "exec fish"},
	}})
	guide.Invoke = func(context.Context, string) error {
		invoked++
		return nil
	}

	m := New(guide)
	cmd := press(m, tea.KeyMsg{Type: tea.KeyEnter})

	if invoked != 0 {
		t.Fatalf("invoked = %d, want HS entries shown and never run", invoked)
	}
	if m.quit || isQuit(cmd) {
		t.Fatal("the guide closed on a bound key, want the footer confirmation instead")
	}
	if !strings.Contains(m.footer, "herdr-bar.open") {
		t.Fatalf("footer = %q, want the command the entry runs", m.footer)
	}

	press(m, tea.KeyMsg{Type: tea.KeyDown})
	press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m.footer, "exec fish") {
		t.Fatalf("footer = %q, want the next entry's command", m.footer)
	}
}

func TestEnterWithoutACommandSaysSo(t *testing.T) {
	m := New(testGuide(group{name: NameCustom, entries: []Entry{{Key: "prefix+t", Description: "TickTick", Kind: KindManual}}}))

	press(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !strings.Contains(m.footer, "no command") {
		t.Fatalf("footer = %q, want an honest empty command note", m.footer)
	}
}

func TestEscapeClosesTheGuide(t *testing.T) {
	m := New(testGuide(group{name: NameHS, entries: hsEntries()}))

	cmd := press(m, tea.KeyMsg{Type: tea.KeyEsc})

	if !m.quit || !isQuit(cmd) {
		t.Fatalf("quit = %v (cmd %v), want esc to close", m.quit, cmd)
	}
}

func TestArrowsMoveAcrossTheVisibleEntries(t *testing.T) {
	m := New(testGuide(
		group{name: NameHS, entries: hsEntries()},
		group{name: NameCustom, entries: []Entry{{Key: "prefix+g", Description: "Git status", Kind: KindManual}}},
	))
	down := tea.KeyMsg{Type: tea.KeyDown}
	up := tea.KeyMsg{Type: tea.KeyUp}

	for _, key := range []string{"prefix+k", "ctrl+shift+u", "prefix+p", "prefix+g"} {
		if entry, ok := m.highlight(); !ok || entry.Key != key {
			t.Fatalf("highlight = %#v (ok %v), want %q", entry, ok, key)
		}
		press(m, down)
	}
	if entry, _ := m.highlight(); entry.Key != "prefix+g" {
		t.Fatalf("highlight = %q, want the cursor to stay on the last row", entry.Key)
	}

	for _, key := range []string{"prefix+p", "ctrl+shift+u", "prefix+k"} {
		press(m, up)
		if entry, ok := m.highlight(); !ok || entry.Key != key {
			t.Fatalf("highlight = %#v (ok %v), want %q", entry, ok, key)
		}
	}
	press(m, up)
	if entry, _ := m.highlight(); entry.Key != "prefix+k" {
		t.Fatalf("highlight = %q, want the cursor to stay on the first row", entry.Key)
	}
}

// TestEverySourceOffIsAGracefulEmptyState pins the guide's answer to the default
// config: a screen that names the switches instead of a blank or a crash.
func TestEverySourceOffIsAGracefulEmptyState(t *testing.T) {
	m := New(Guide{ConfigPath: "/home/u/.config/herdr/plugins/config/herdr-utils/config.json"})

	view := m.View()

	for _, want := range []string{"Every source is off", "which.sources", m.guide.ConfigPath, "hs", "gentle", "custom", "discovered"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty view does not contain %q:\n%s", want, view)
		}
	}
	if m.total() != 0 || m.shown() != 0 {
		t.Fatalf("shown/total = %d/%d, want 0/0", m.shown(), m.total())
	}
}

func TestAnEnabledSourceWithoutEntriesShowsItsProblem(t *testing.T) {
	guide := testGuide(group{name: NameHS, problem: "no herdr config at /home/u/.config/herdr/config.toml"})
	guide.Notes = []string{"GENTLE auto-discovery is pending; the source reads which.gentle."}

	view := New(guide).View()

	for _, want := range []string{"HS 0", "no herdr config at", "auto-discovery"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestTheFooterNamesTheKeysAndTheEnterAction(t *testing.T) {
	// A wide pane, so the whole hint set fits on one line: the seven hints the
	// guide answers to page on anything narrower.
	bound := size(New(testGuide(group{name: NameHS, entries: hsEntries()})), 180, 50)
	view := bound.View()
	for _, want := range []string{"enter  show command", "↑↓", "move", "type", "filter", "ctrl+t", "view", "esc", "close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("footer does not contain %q:\n%s", want, view)
		}
	}

	discovered := size(New(testGuide(group{name: NameDiscovered, entries: []Entry{
		{Key: "enter", Description: "Open bar", Kind: KindAction, Command: "herdr plugin action invoke herdr-bar.open", ActionID: "herdr-bar.open"},
	}})), 180, 50)
	if view := discovered.View(); !strings.Contains(view, "enter  run action") {
		t.Fatalf("footer does not invite running the action:\n%s", view)
	}
}

func TestTheGuideStartsInTheConfiguredView(t *testing.T) {
	guide := testGuide(group{name: NameHS, entries: hsEntries()})
	guide.View = config.ViewList

	if got := New(guide).Mode(); got != config.ViewList {
		t.Fatalf("Mode = %q, want the saved %q", got, config.ViewList)
	}
}

// TestTabPagesTheHintBarAndWraps pins S6 (tui-kit): a hint that does not fit is
// reachable — the bar ends with the paging chip, the paging key moves to the next
// page, and the last page wraps back to the first.
func TestTabPagesTheHintBarAndWraps(t *testing.T) {
	m := size(New(shotGuide()), 80, 24)

	first := m.View()
	if !strings.Contains(plainText(first), chip.PageKey+"  more 1/2") {
		t.Fatalf("a narrow guide does not say the hints are paged:\n%s", first)
	}
	if strings.Contains(plainText(first), "clear") {
		t.Fatalf("the first page already shows the last hint, so the bar did not page:\n%s", first)
	}

	press(m, tea.KeyMsg{Type: tea.KeyTab})
	second := m.View()
	if !strings.Contains(plainText(second), "more 2/2") {
		t.Fatalf("the paging key did not move to the next page:\n%s", second)
	}
	if !strings.Contains(plainText(second), "clear") {
		t.Fatalf("the second page does not show the hints that did not fit:\n%s", second)
	}

	press(m, tea.KeyMsg{Type: tea.KeyTab})
	if wrapped := plainText(m.View()); !strings.Contains(wrapped, "more 1/2") {
		t.Fatalf("the paging key does not wrap from the last page back to the first:\n%s", wrapped)
	}
}

// TestTabDoesNotTypeIntoTheFilter pins the paging key against the filter: it is
// consumed before the rune branch, so it pages instead of narrowing the list.
func TestTabDoesNotTypeIntoTheFilter(t *testing.T) {
	m := size(New(shotGuide()), 80, 24)
	before := m.shown()

	press(m, tea.KeyMsg{Type: tea.KeyTab})
	press(m, tea.KeyMsg{Type: tea.KeyTab})
	press(m, tea.KeyMsg{Type: tea.KeyTab})

	if strings.TrimSpace(m.query) != "" {
		t.Fatalf("query = %q, want tab to leave the filter empty", m.query)
	}
	if m.shown() != before {
		t.Fatalf("shown = %d, want the %d entries the empty filter matched", m.shown(), before)
	}
	if view := m.View(); !strings.Contains(view, filterPrompt) {
		t.Fatalf("the filter row lost its prompt after tab:\n%s", view)
	}
}

// TestTheHintPageFollowsTheHintSet pins the kit's reset rule (tui-kit S10): the
// bar starts over when the hints themselves change — a page kept from a set that
// is gone would show a position the bar no longer has — and keeps its page while
// they stay the same. The guide sets no page by hand.
func TestTheHintPageFollowsTheHintSet(t *testing.T) {
	t.Run("an unchanged hint set keeps the page", func(t *testing.T) {
		m := size(New(shotGuide()), 80, 24)
		press(m, tea.KeyMsg{Type: tea.KeyTab})
		if !strings.Contains(plainText(m.View()), "more 2/2") {
			t.Fatalf("the guide is not on the second page before the change:\n%s", m.View())
		}

		// Two bound entries: what enter does is the same on both, so the bar's
		// hints are the same set and the page it shows stays.
		press(m, tea.KeyMsg{Type: tea.KeyDown})

		if view := plainText(m.View()); !strings.Contains(view, "more 2/2") {
			t.Fatalf("the bar lost its page although its hints did not change:\n%s", view)
		}
	})

	t.Run("a changed hint set starts on page one", func(t *testing.T) {
		m := size(New(shotGuide()), 80, 24)
		press(m, tea.KeyMsg{Type: tea.KeyTab})
		if !strings.Contains(plainText(m.View()), "more 2/2") {
			t.Fatalf("the guide is not on the second page before the change:\n%s", m.View())
		}

		// A live action turns enter into "run action", so the set differs.
		for {
			entry, ok := m.highlight()
			if !ok {
				t.Fatal("the guide has nothing selected")
			}
			if entry.Invokable() {
				break
			}
			press(m, tea.KeyMsg{Type: tea.KeyDown})
		}

		if view := plainText(m.View()); !strings.Contains(view, "more 1/") {
			t.Fatalf("a changed hint set kept the old page:\n%s", view)
		}
	})
}

// TestTabDoesNothingWhenEverythingFits pins the other half of the paging rule: a
// wide footer shows every hint on one line, with no paging chip, and the paging
// key leaves the screen untouched.
func TestTabDoesNothingWhenEverythingFits(t *testing.T) {
	m := size(New(shotGuide()), 180, 50)
	before := m.View()
	if strings.Contains(plainText(before), "more ") {
		t.Fatalf("a footer that fits still shows a paging chip:\n%s", before)
	}

	press(m, tea.KeyMsg{Type: tea.KeyTab})

	if after := m.View(); after != before {
		t.Fatalf("the paging key redrew a footer that fits:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
