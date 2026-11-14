package which

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WilberC/tui-kit/chip"
	"github.com/WilberC/tui-kit/clipboard"
	"github.com/WilberC/tui-kit/zone"

	"github.com/WilberC/herdr-utils/internal/config"
)

// The ids the guide records its clickable elements under. A row and its chip name
// the flat entry they belong to, so a click needs no second guess; the two header
// names are the views a click can open, and the hint bar records its own. Only a
// row, a chip and the paging chip are clickable: everything the mouse reaches is
// also a key.
const (
	rowZonePrefix  = "row:"
	chipZonePrefix = "chip:"
	viewKeysZone   = "view:keys"
	viewListZone   = "view:list"
)

// doubleClickWindow is how close two clicks on the same element must be to count
// as a double click, which runs what enter runs. A slower pair is two clicks:
// selecting, twice.
const doubleClickWindow = 400 * time.Millisecond

// copyConfirmFor is how long the detail line keeps the copy confirmation: long
// enough to read, short enough that it never describes the next selection.
const copyConfirmFor = 1500 * time.Millisecond

// rowZone is the zone id of one entry's row: the whole row, so a click anywhere
// on it selects the entry.
func rowZone(index int) string { return rowZonePrefix + strconv.Itoa(index) }

// chipZone is the zone id of one entry's chip slab: clicking the token selects
// the same entry as clicking its row.
func chipZone(index int) string { return chipZonePrefix + strconv.Itoa(index) }

// lastClick is the click the guide acted on last, so a second one in the same
// place can be recognised as the run.
type lastClick struct {
	id string
	at time.Time
}

// copyExpired clears the copy confirmation once its moment has passed.
type copyExpired struct{ value string }

// Model is the guide's Bubbletea model: the built guide plus the state a
// keystroke changes. It never talks to herdr itself; Invoke and Persist are the
// callbacks Build carried.
type Model struct {
	guide  Guide
	mode   string
	query  string
	cursor int
	footer string
	quit   bool
	width  int
	height int

	// bar is the hint bar and the page it shows. It resets itself when the hints
	// change, so the guide never sets the page by hand.
	bar chip.HintBar

	// zones is where the last render drew each clickable element, in pane cells.
	// A click is answered from this map, never from a second measurement.
	zones *zone.Map

	// hover is the flat index of the entry under the pointer, -1 when the pointer
	// is over nothing clickable. The selection wins over it where they meet.
	hover int

	// click is the last click, for the double-click window; now is the clock that
	// judges it, so a test drives time instead of sleeping.
	click lastClick
	now   func() time.Time

	// out is where the copy key writes its OSC 52 sequence, and confirm is how
	// long the detail line keeps the confirmation. copied is the value being
	// confirmed, so a newer message is never cleared by an older timer.
	out     io.Writer
	confirm time.Duration
	copied  string
}

var _ tea.Model = (*Model)(nil)

// New returns the model for a built guide.
func New(guide Guide) *Model {
	return &Model{
		guide:   guide,
		mode:    config.NormalizeView(guide.View),
		zones:   zone.New(),
		hover:   -1,
		now:     time.Now,
		confirm: copyConfirmFor,
	}
}

// Mode is the view the guide draws: config.ViewKeys or config.ViewList.
func (m *Model) Mode() string { return m.mode }

// Init starts the screen; the guide reads everything before it opens.
func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(message)
	case tea.MouseMsg:
		return m, m.handleMouse(message)
	case copyExpired:
		if m.copied == message.value {
			m.copied = ""
			m.footer = ""
		}
	}
	return m, nil
}

// handleMouse answers the pointer: a click on a row or its chip selects, the
// same click twice runs what enter runs, motion previews the row under the
// pointer, the wheel moves the selection — which pages the body — and the two
// views and the paging chip are clickable where the header and the bar name them.
func (m *Model) handleMouse(mouse tea.MouseMsg) tea.Cmd {
	switch mouse.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if mouse.Action != tea.MouseActionPress {
			return nil
		}
		if mouse.Button == tea.MouseButtonWheelUp {
			m.move(-1)
		} else {
			m.move(1)
		}
		return nil
	}

	id, hit := m.zones.Hit(mouse.X, mouse.Y)
	if mouse.Action == tea.MouseActionMotion {
		m.hover, _ = m.rowAt(id, hit)
		return nil
	}
	if mouse.Action != tea.MouseActionPress {
		return nil
	}
	if !hit {
		m.hover = -1
		return nil
	}

	switch id {
	case chip.PageZone:
		m.bar.Next(m.hintWidth(), m.hintSet()...)
		return nil
	case viewKeysZone:
		m.setMode(config.ViewKeys)
		return nil
	case viewListZone:
		m.setMode(config.ViewList)
		return nil
	}

	index, isEntry := m.rowAt(id, true)
	if !isEntry {
		return nil
	}
	m.selectRow(index)
	now := m.now()
	run := m.click.id == id && now.Sub(m.click.at) <= doubleClickWindow
	m.click = lastClick{id: id, at: now}
	if !run {
		return nil
	}
	m.click = lastClick{}
	return m.activate()
}

// rowAt reads an entry zone id back into the flat index it names.
func (m *Model) rowAt(id string, hit bool) (int, bool) {
	if !hit {
		return -1, false
	}
	for _, prefix := range []string{rowZonePrefix, chipZonePrefix} {
		rest, found := strings.CutPrefix(id, prefix)
		if !found {
			continue
		}
		index, err := strconv.Atoi(rest)
		if err != nil || index < 0 || index >= m.shown() {
			return -1, false
		}
		return index, true
	}
	return -1, false
}

// selectRow moves the cursor to a row a click landed on. It never runs the entry:
// selecting is reversible, and nothing here is destructive, but the rule stands.
func (m *Model) selectRow(index int) {
	if index < 0 || index >= m.shown() {
		return
	}
	m.cursor = index
}

func (m *Model) handleKey(key tea.KeyMsg) tea.Cmd {
	// tab pages the hint bar. It is consumed before the rune branch, so the
	// paging key never types into the filter.
	if key.String() == chip.PageKey {
		m.bar.Next(m.hintWidth(), m.hintSet()...)
		return nil
	}
	switch key.String() {
	case "ctrl+c", "esc":
		m.quit = true
		return tea.Quit
	case "ctrl+t":
		m.cycleMode()
		return nil
	case "ctrl+y":
		return m.copySelection()
	case "up", "ctrl+p":
		m.move(-1)
		return nil
	case "down", "ctrl+n":
		m.move(1)
		return nil
	case "enter":
		return m.activate()
	case "backspace":
		m.dropRune()
		return nil
	case "ctrl+u":
		m.setQuery("")
		return nil
	}
	if key.Type == tea.KeyRunes {
		m.setQuery(m.query + string(key.Runes))
	}
	return nil
}

// cycleMode flips the KEYS/LIST view.
func (m *Model) cycleMode() {
	if m.mode == config.ViewList {
		m.setMode(config.ViewKeys)
		return
	}
	m.setMode(config.ViewList)
}

// setMode opens one view and records it in the plugin config, so the guide opens
// the way the user left it. A click on the header and ctrl+t land here, so the two
// paths cannot drift.
func (m *Model) setMode(view string) {
	if m.mode == view {
		return
	}
	m.mode = view
	m.hover = -1
	m.footer = "view: " + strings.ToUpper(m.mode)
	if m.guide.Persist == nil {
		return
	}
	if err := m.guide.Persist(m.mode); err != nil {
		m.footer = "could not save the view: " + err.Error()
	}
}

// activate runs a discovered action through herdr and closes; for a bound key or
// a manual entry it reports the command in the footer and runs nothing.
func (m *Model) activate() tea.Cmd {
	entry, ok := m.highlight()
	if !ok {
		return nil
	}
	if entry.Invokable() {
		if m.guide.Invoke == nil {
			m.footer = "cannot run " + entry.ActionID + ": no herdr client here"
			return nil
		}
		if err := m.guide.Invoke(context.Background(), entry.ActionID); err != nil {
			m.footer = "herdr refused " + entry.ActionID + ": " + err.Error()
			return nil
		}
		m.footer = "ran " + entry.ActionID
		m.quit = true
		return tea.Quit
	}
	if entry.Command == "" {
		m.footer = "no command recorded for this entry"
		return nil
	}
	m.footer = "this entry runs: " + entry.Command
	return nil
}

func (m *Model) move(delta int) {
	total := m.shown()
	if total == 0 {
		m.cursor = 0
		return
	}
	next := m.cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= total {
		next = total - 1
	}
	m.cursor = next
}

func (m *Model) setQuery(query string) {
	m.hover = -1
	m.query = query
	if total := m.shown(); m.cursor >= total {
		m.cursor = total - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) dropRune() {
	runes := []rune(m.query)
	if len(runes) == 0 {
		return
	}
	m.setQuery(string(runes[:len(runes)-1]))
}

// visible is one row of the flat list the cursor moves through: the source it
// belongs to and the entry it shows.
type visible struct {
	source int
	entry  Entry
}

func (m *Model) visibleEntries(source int) []Entry {
	declared := m.guide.Sources[source].Entries()
	entries := make([]Entry, 0, len(declared))
	for _, entry := range declared {
		if matches(entry, m.query) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func matches(entry Entry, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(entry.Key), query) || strings.Contains(strings.ToLower(entry.Description), query)
}

func (m *Model) matches() []visible {
	matches := make([]visible, 0, m.total())
	for index := range m.guide.Sources {
		for _, entry := range m.visibleEntries(index) {
			matches = append(matches, visible{source: index, entry: entry})
		}
	}
	return matches
}

func (m *Model) shown() int { return len(m.matches()) }

func (m *Model) total() int {
	total := 0
	for _, source := range m.guide.Sources {
		total += len(source.Entries())
	}
	return total
}

func (m *Model) highlight() (Entry, bool) {
	matches := m.matches()
	if m.cursor < 0 || m.cursor >= len(matches) {
		return Entry{}, false
	}
	return matches[m.cursor].entry, true
}

// highlightVisible is the selected entry together with the source it came from,
// which the footer names.
func (m *Model) highlightVisible() (visible, bool) {
	matches := m.matches()
	if m.cursor < 0 || m.cursor >= len(matches) {
		return visible{}, false
	}
	return matches[m.cursor], true
}

// Run shows the guide until the user leaves it.
//
// The mouse is enabled with every motion reported, because that is what a hover
// costs: herdr forwards a motion event to a pane only while the pane asked for DEC
// mode 1003 (any-motion) and drops it otherwise, and Bubbletea's
// WithMouseAllMotion asks for exactly that mode.
func (m *Model) Run(in io.Reader, out io.Writer) error {
	m.out = out
	options := []tea.ProgramOption{tea.WithOutput(out), tea.WithMouseAllMotion()}
	if in != nil {
		options = append(options, tea.WithInput(in))
	}
	_, err := tea.NewProgram(m, options...).Run()
	return err
}

// copySelection copies the one value the cursor is on — the command a binding
// runs, or the action id of a live action — with OSC 52, and confirms it on the
// detail line. The sequence carries no newline, so it goes straight to the
// screen's own writer: a line the program printed would scroll the frame.
func (m *Model) copySelection() tea.Cmd {
	entry, ok := m.highlight()
	if !ok {
		m.footer = "nothing selected to copy"
		return nil
	}
	value := entry.Command
	if entry.Invokable() {
		value = entry.ActionID
	}
	if value == "" {
		m.footer = "nothing to copy for this entry"
		return nil
	}

	m.copied = value
	m.footer = "copied " + value
	out, confirm := m.out, m.confirm
	return tea.Batch(
		func() tea.Msg {
			if out != nil {
				_, _ = io.WriteString(out, clipboard.OSC52(value))
			}
			return nil
		},
		tea.Tick(confirm, func(time.Time) tea.Msg { return copyExpired{value: value} }),
	)
}
