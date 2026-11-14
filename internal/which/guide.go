package which

import (
	"context"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WilberC/herdr-utils/internal/config"
)

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
}

var _ tea.Model = (*Model)(nil)

// New returns the model for a built guide.
func New(guide Guide) *Model {
	return &Model{guide: guide, mode: config.NormalizeView(guide.View)}
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
	}
	return m, nil
}

func (m *Model) handleKey(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+c", "esc":
		m.quit = true
		return tea.Quit
	case "ctrl+t":
		m.cycleMode()
		return nil
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

// cycleMode flips the KEYS/LIST view and records it in the plugin config, so the
// guide opens the way the user left it.
func (m *Model) cycleMode() {
	if m.mode == config.ViewList {
		m.mode = config.ViewKeys
	} else {
		m.mode = config.ViewList
	}
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
func (m *Model) Run(in io.Reader, out io.Writer) error {
	options := []tea.ProgramOption{tea.WithOutput(out)}
	if in != nil {
		options = append(options, tea.WithInput(in))
	}
	_, err := tea.NewProgram(m, options...).Run()
	return err
}
