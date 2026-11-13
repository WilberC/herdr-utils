package which

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WilberC/herdr-utils/internal/config"
)

// blockTextWidth wraps the problem an empty source prints, so a long path does
// not destroy the columns beside it.
const blockTextWidth = 46

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	filterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle      = lipgloss.NewStyle().Faint(true)
	messageStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	selectedStyle = lipgloss.NewStyle().Bold(true)
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
	m.footer = "view: " + m.mode
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

func (m *Model) View() string {
	var view strings.Builder
	fmt.Fprintf(&view, "%s  %s\n", titleStyle.Render("herdr-utils which"), dimStyle.Render(fmt.Sprintf("%d / %d shown · %s view", m.shown(), m.total(), strings.ToUpper(m.mode))))
	fmt.Fprintf(&view, "%s\n\n", filterStyle.Render("filter ❯ "+m.query))

	if len(m.guide.Sources) == 0 {
		view.WriteString(m.emptyState())
	} else {
		view.WriteString(m.columns())
	}
	view.WriteString("\n")
	for _, note := range m.guide.Notes {
		fmt.Fprintf(&view, "%s\n", dimStyle.Render(note))
	}
	if m.footer != "" {
		fmt.Fprintf(&view, "%s\n", messageStyle.Render(m.footer))
	}
	fmt.Fprintf(&view, "%s\n", dimStyle.Render(m.hints()))
	return view.String()
}

// emptyState is the answer to the default config: every source off, so the
// screen names the switches instead of drawing a blank guide.
func (m *Model) emptyState() string {
	return strings.Join([]string{
		"Every source is off.",
		"",
		"Enable one under which.sources in " + m.guide.ConfigPath + ":",
		"  hs, gentle, custom, discovered",
	}, "\n")
}

func (m *Model) hints() string {
	enter := "enter show command"
	if entry, ok := m.highlight(); ok && entry.Invokable() {
		enter = "enter run action"
	}
	return enter + " · ↑↓ move · type filter · ctrl+t view · esc close"
}

func (m *Model) columns() string {
	blocks := make([]string, 0, len(m.guide.Sources))
	for index := range m.guide.Sources {
		blocks = append(blocks, m.block(index))
	}
	return joinBlocks(blocks, m.width)
}

func (m *Model) block(index int) string {
	source := m.guide.Sources[index]
	entries := m.visibleEntries(index)

	offset := 0
	for previous := 0; previous < index; previous++ {
		offset += len(m.visibleEntries(previous))
	}

	var block strings.Builder
	fmt.Fprintf(&block, "%s\n", lipgloss.NewStyle().Foreground(source.Color()).Bold(true).Render(
		strings.TrimSpace(source.Glyph()+" "+source.Name())+" "+countLabel(len(entries), len(source.Entries()), m.query != ""),
	))

	if len(entries) == 0 {
		block.WriteString(dimStyle.Render(m.emptyBlock(source)))
		block.WriteString("\n")
		return block.String()
	}

	keyWidth := 0
	for _, entry := range entries {
		if width := len([]rune(entry.Key)); width > keyWidth {
			keyWidth = width
		}
	}
	for row, entry := range entries {
		marker := "  "
		if offset+row == m.cursor {
			marker = "> "
		}
		switch m.mode {
		case config.ViewList:
			fmt.Fprintf(&block, "%s%s  %s\n", marker, entry.Description, dimStyle.Render(entry.Key))
		default:
			fmt.Fprintf(&block, "%s%s  %s\n", marker, selectedStyle.Render(pad(entry.Key, keyWidth)), entry.Description)
		}
	}
	return block.String()
}

func (m *Model) emptyBlock(source Source) string {
	if problem, ok := source.(ProblemSource); ok && problem.Problem() != "" {
		return wrapText(problem.Problem(), blockTextWidth)
	}
	if m.query != "" {
		return "(no match)"
	}
	return "(no entries)"
}

func countLabel(shown, total int, filtered bool) string {
	if shown == total && !filtered {
		return strconv.Itoa(total)
	}
	return fmt.Sprintf("%d / %d", shown, total)
}

func pad(value string, width int) string {
	if missing := width - len([]rune(value)); missing > 0 {
		return value + strings.Repeat(" ", missing)
	}
	return value
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := make([]string, 0, 2)
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	return strings.Join(append(lines, line), "\n")
}

// joinBlocks places the source columns side by side, wrapping into another row
// of columns when the terminal is narrower than the columns together.
func joinBlocks(blocks []string, width int) string {
	const gutter = "  "
	var rows [][]string
	current := make([]string, 0, len(blocks))
	currentWidth := 0

	for _, block := range blocks {
		blockWidth := lipgloss.Width(block)
		extra := blockWidth
		if len(current) > 0 {
			extra += len(gutter)
		}
		if width > 0 && len(current) > 0 && currentWidth+extra > width {
			rows = append(rows, current)
			current = nil
			currentWidth = 0
			extra = blockWidth
		}
		if len(current) > 0 {
			currentWidth += len(gutter)
		}
		current = append(current, block)
		currentWidth += blockWidth
	}
	if len(current) > 0 {
		rows = append(rows, current)
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(lines, "\n")
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
