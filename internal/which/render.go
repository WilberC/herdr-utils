package which

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/WilberC/tui-kit/chip"
	"github.com/WilberC/tui-kit/frame"
	"github.com/WilberC/tui-kit/list"
	"github.com/WilberC/tui-kit/text"
	"github.com/WilberC/tui-kit/theme"
	"github.com/WilberC/tui-kit/zone"

	"github.com/WilberC/herdr-utils/internal/config"
)

const (
	maxChipWidth = 14
	minDescWidth = 8

	// The pane the guide lays itself out in before Bubbletea reports the real one.
	// A program sends its size first, so this is only the answer for a view drawn
	// outside one.
	defaultPaneWidth  = 100
	defaultPaneHeight = 40

	// entryIndent is how many cells list.Entry spends before the row's content: the
	// two cells of indent, the two of the tree guide and the one after it. The
	// guide clips a description against it instead of letting the row cut the text.
	entryIndent = 5

	// hintRow is the line of the pinned footer the hint bar occupies: the closing
	// column rule, the detail line, then the bar. A zone lands on the line it was
	// drawn on.
	hintRow = 2

	// tableGap is how many cells list.Entry spends before a LIST row's description
	// column: the tree guide (entryIndent), the chip cell and the space after it,
	// and the two cells and source column that close the row.
	tableGap = entryIndent + 6
)

// The guide's own words: how the frame names itself, what the groups and the
// views are called, the glyph that opens the filter. Every other glyph, and the
// whole typography, comes from the kit.
const (
	filterGlyph  = "❱"
	continued    = " (cont.)"
	productName  = "herdr-utils which"
	guideSubname = "keybinding guide"
	filterPrompt = "Filter keys"
	bindingLabel = "BINDING"
	entryLabel   = "ENTRY"
	sourceLabel  = "SOURCE"

	// separator joins two facts with one space on each side. A separator that is
	// its own fact is theme.Dot(); inside one quiet sentence the guide keeps it
	// part of that sentence's role.
	separator = " · "
)

// rowKind tells the column renderer what one body row is.
type rowKind int

const (
	rowGroup rowKind = iota
	rowContinued
	rowEntry
	rowEmpty
	rowSpacer
)

// bodyRow is one line of the body flow, before it is cut into columns.
type bodyRow struct {
	kind   rowKind
	source int
	entry  Entry
	index  int  // position in the flat visible list, -1 when the row is not an entry
	last   bool // the last entry of its group
	text   string
	height int // a group header's own height, spacer included
}

// pane is the size the guide draws in: the terminal's, or a readable default
// before Bubbletea reports one.
func (m *Model) pane() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = defaultPaneWidth
	}
	if height <= 0 {
		height = defaultPaneHeight
	}
	return width, height
}

// View draws the guide inside the kit's surface: the header, the rule under it,
// the filter row and the rule that opens the columns sit on top; the rule that
// closes them, the detail line, the hint bar and the notes are pinned to the
// bottom; the body takes the rows between them and always opens and closes with
// one blank row. The frame is the pane clamped to frame.Recommended and centred,
// so it fills the popup it was opened in instead of hugging its content. It reads
// state only; every byte it needs was built before the screen opened. The surface
// itself is the kit's; the guide owns what it puts in it, and records where it put
// every clickable element so a click is answered from cells, never from a second
// measurement.
func (m *Model) View() string {
	paneWidth, paneHeight := m.pane()
	panel := frame.Frame{
		Title: theme.GoldBold.Render(productName),
		Width: frame.Recommended.ClampWidth(paneWidth),
	}
	textWidth := panel.ContentWidth()
	widths := m.layoutWidths(panel.Width - 2)
	hints := m.hintSet()
	m.zones = zone.New()

	surface := frame.Surface{Frame: panel, Size: frame.Recommended}
	if m.mode != config.ViewList {
		surface.Columns = widths
	}

	notes := make([]string, 0, len(m.guide.Notes))
	for _, note := range m.guide.Notes {
		notes = append(notes, theme.Muted.Render(text.Clip(note, textWidth)))
	}

	// The header names the page, so it is the one top row that depends on the body:
	// the surface needs the count of every other row to answer BodyRows first, and
	// the header's row is filled in below.
	top := []string{
		"",
		panel.Rule(),
		surface.Row(paneWidth, m.filterLine(textWidth)),
		frame.ColumnRule(widths, frame.TopTee),
	}
	bottom := []string{
		frame.ColumnRule(widths, frame.BottomTee),
		surface.Row(paneWidth, m.detailLine(textWidth)),
		"", // the hint bar, drawn below once the surface knows the row it sits on
	}
	bottom = append(bottom, surface.Rows(paneWidth, notes...)...)
	surface.Top, surface.Bottom = top, bottom

	// The hint bar records each of its chips and the paging chip, so a click on the
	// paging chip is the paging key. It is drawn into its own pinned row.
	canvas := m.canvas(surface, paneHeight)
	bottom[hintRow] = surface.Row(paneWidth, m.hintsLine(textWidth, surface.BottomY(paneHeight, hintRow), hints))

	body := m.body(surface, canvas, paneWidth, widths)
	top[0] = surface.Row(paneWidth, m.headerLine(textWidth, body))
	return surface.PopupZones(paneWidth, paneHeight, body.lines, m.zones) + "\n"
}

// headerLine carries the guide's own name, the sources it read, the current view
// and how much of it is shown. The two view names are click targets, recorded at
// the cells they land on: a click on LIST opens the table, one on KEYS goes back.
func (m *Model) headerLine(textWidth int, body bodyView) string {
	left := theme.Muted.Render(guideSubname) + theme.Quiet.Render(" │ ") + theme.Muted.Render(m.sourceCount())
	keys, table := m.viewLabel(config.ViewKeys), m.viewLabel(config.ViewList)
	right := keys + theme.Dot() +
		table + theme.Dot() +
		theme.Muted.Render(fmt.Sprintf("%d / %d shown", m.shown(), m.total()))
	if body.pages > 1 {
		right += theme.Dot() + theme.Muted.Render(fmt.Sprintf("page %d / %d", body.page, body.pages))
	}
	// The right side keeps one cell of air before it, and a side that does not fit
	// with that cell is dropped whole instead of touching the text it explains.
	line := text.JoinDropRight(left, " "+right, textWidth)

	// A right side the line could not hold is not drawn, so it records no zone: a
	// click must never land on a label the user cannot see.
	if text.Width(left)+1+text.Width(right) <= textWidth {
		start := textWidth - text.Width(right)
		m.zones.Add(viewKeysZone, start, 0, text.Width(keys), 1)
		m.zones.Add(viewListZone, start+text.Width(keys)+text.Width(theme.Dot()), 0, text.Width(table), 1)
	}
	return line
}

func (m *Model) sourceCount() string {
	if count := len(m.guide.Sources); count != 1 {
		return fmt.Sprintf("%d sources", count)
	}
	return "1 source"
}

// viewLabel names the view, and lights up the one the guide is drawing.
func (m *Model) viewLabel(view string) string {
	name := strings.ToUpper(view)
	if m.mode == view {
		return theme.TealBold.Render(name)
	}
	return theme.Muted.Render(name)
}

// filterLine shows the live query with a block cursor, and the legend that says
// what the three kinds of key look like. The legend gives way before the prompt
// does when the row is too narrow to hold both.
func (m *Model) filterLine(textWidth int) string {
	left := theme.Accent.Render(filterGlyph + " ")
	if strings.TrimSpace(m.query) == "" {
		left += theme.Muted.Render(filterPrompt + text.Ellipsis)
	} else {
		left += theme.Text.Render(m.query) + theme.Accent.Render(text.Cursor)
	}
	return text.JoinDropRight(left, " "+legend(), textWidth)
}

// legend explains the chips: a prefix binding, a direct one, and an action that
// declares no key of its own. Items are separated by two plain spaces, so no
// separator ever touches a chip's background.
func legend() string {
	return chip.Legend(
		chip.Pair(theme.ChipAccent, "x", "after prefix"),
		chip.Pair(theme.ChipAccent, "ctrl+x", "direct"),
		theme.Muted.Render(text.Unset)+" "+theme.Muted.Render("unset"),
	)
}

// detailLine describes the selected entry: what it does, the binding that runs
// it, the source it came from and its kind. The binding is the only green on the
// screen. A status message sits at the right edge and never adds a row.
func (m *Model) detailLine(textWidth int) string {
	right := ""
	if m.footer != "" {
		right = theme.Gold.Render(text.Clip(m.footer, textWidth))
	}

	row, ok := m.highlightVisible()
	if !ok {
		empty := theme.Muted.Render("nothing selected" + separator + "type to filter")
		if right == "" {
			return empty
		}
		return text.JoinDropRight(empty, " "+right, textWidth)
	}
	entry := row.entry
	key := entry.Key
	if key == "" {
		key = text.Unset
	}

	segments := []struct {
		text  string
		style lipgloss.Style
	}{
		{entry.Description, theme.AccentBold},
		{key, theme.Green},
		{m.guide.Sources[row.source].Name(), theme.Muted},
		{strings.ReplaceAll(entry.Kind, "_", " "), theme.Muted},
	}

	var line strings.Builder
	remaining := textWidth
	for _, segment := range segments {
		if segment.text == "" {
			continue
		}
		// A separator is its own fact, so it carries the guide role. The first
		// segment has no separator, and the role still renders it: the row keeps
		// the shape it has always had.
		gap := ""
		if line.Len() > 0 {
			gap = separator
		}
		needed := text.Width(gap) + text.Width(segment.text)
		if needed > remaining {
			if line.Len() > 0 {
				break
			}
			segment.text = text.Clip(segment.text, remaining)
			needed = text.Width(segment.text)
			if needed == 0 {
				break
			}
		}
		line.WriteString(theme.Quiet.Render(gap))
		line.WriteString(segment.style.Render(segment.text))
		remaining -= needed
	}
	if right == "" {
		return line.String()
	}
	return text.JoinDropRight(line.String(), " "+right, textWidth)
}

// hintSet is the hints the guide answers to, in declaration order — the order is
// the priority the bar pages through. The first one names what enter does for the
// entry under the cursor.
func (m *Model) hintSet() []chip.Hint {
	enter := "show command"
	if entry, ok := m.highlight(); ok && entry.Invokable() {
		enter = "run action"
	}
	return []chip.Hint{
		{Key: "enter", Label: enter},
		{Key: "↑↓", Label: "move"},
		{Key: "type", Label: "filter"},
		{Key: "ctrl+y", Label: "copy"},
		{Key: "ctrl+t", Label: "view"},
		{Key: "esc", Label: "close"},
		{Key: "ctrl+u", Label: "clear"},
	}
}

// hintsLine is the page of the hint bar the guide shows: every hint that fits,
// and the paging chip naming chip.PageKey when some do not. A hint is never
// dropped without it, and every chip is recorded at the row it was drawn on, so
// the paging chip is clickable.
func (m *Model) hintsLine(textWidth, y int, hints []chip.Hint) string {
	return m.bar.RenderZones(m.zones, 0, y, textWidth, hints...)
}

// hintWidth is the room the hint bar is drawn in: the frame's content width at
// this pane, which is the width the bar pages against.
func (m *Model) hintWidth() int {
	paneWidth, _ := m.pane()
	return frame.Frame{Width: frame.Recommended.ClampWidth(paneWidth)}.ContentWidth()
}

// layoutWidths is how the body is split: one column for the states that hold a
// single centred block or the table, the earned columns otherwise.
func (m *Model) layoutWidths(innerWidth int) []int {
	if m.mode == config.ViewList || len(m.guide.Sources) == 0 || m.filteredToNothing() {
		return []int{innerWidth - 2}
	}
	return frame.ColumnWidths(innerWidth, frame.ColumnCount(innerWidth, len(m.guide.Sources)))
}

// filteredToNothing reports a live filter with no match, which the guide answers
// with one calm message instead of a set of empty columns.
func (m *Model) filteredToNothing() bool {
	return m.shown() == 0 && strings.TrimSpace(m.query) != ""
}

// bodyView is the body the guide drew, plus the page it had to show: the header
// names the page when the columns hold more than one.
type bodyView struct {
	lines []string
	page  int
	pages int
}

// bodyCanvas is where the body's rows land: the zone map they record in, the
// frame-content x of each column, the frame-content row of the body's first line
// and how many lines the body has room for. A row registers the cells it was drawn
// on, so a line the surface never gave the body is never a click target.
type bodyCanvas struct {
	zones *zone.Map
	colX  []int
	top   int
	rows  int
}

// canvas measures the body against the surface: the same offsets the kit's
// RenderZones translates, so a click lands on the row the user sees.
func (m *Model) canvas(surface frame.Surface, paneHeight int) bodyCanvas {
	colX := surface.ColumnX()
	if len(colX) == 0 {
		colX = []int{0}
	}
	return bodyCanvas{zones: m.zones, colX: colX, top: surface.BodyY(0), rows: surface.BodyRows(paneHeight)}
}

// at is the frame-content cell of one body line in one column, and whether the
// body had room for that line at all.
func (c bodyCanvas) at(column, line int) (int, int, bool) {
	if line < 0 || line >= c.rows || column < 0 || column >= len(c.colX) {
		return 0, 0, false
	}
	return c.colX[column], c.top + line, true
}

// body draws the columns, the table, the empty state or the zero-match state.
func (m *Model) body(surface frame.Surface, canvas bodyCanvas, paneWidth int, widths []int) bodyView {
	panel := surface.At(paneWidth)
	height := canvas.rows
	if m.mode == config.ViewList {
		return m.listBody(panel, widths[0], height, canvas)
	}
	blocks := make([][]string, len(widths))
	switch {
	case len(m.guide.Sources) == 0:
		blocks[0] = centered(m.emptyStateLines(widths[0]), widths[0], height)
	case m.filteredToNothing():
		blocks[0] = centered(m.noMatchLines(widths[0]), widths[0], height)
	default:
		return m.columns(panel, widths, height, canvas)
	}
	lines := 0
	for _, block := range blocks {
		lines = max(lines, len(block))
	}
	return bodyView{lines: frame.Grid(panel, blocks, widths, lines), page: 1, pages: 1}
}

// columns lays the groups out: the smallest column budget that holds the whole
// flow wins, so a short guide still spreads over the columns the terminal
// earned instead of stacking in the first one. A group that does not fit in the
// space left moves to the next column whole; only a group taller than a column
// is continued there under its own "(cont.)" header.
func (m *Model) columns(panel frame.Frame, widths []int, height int, canvas bodyCanvas) bodyView {
	wrap := widths[0]
	for _, width := range widths {
		wrap = min(wrap, width)
	}
	rows := m.rows(wrap)
	columns := len(widths)

	cursorRow := -1
	for index, row := range rows {
		if row.kind == rowEntry && row.index == m.cursor {
			cursorRow = index
			break
		}
	}

	budget := max(1, (len(rows)+columns-1)/columns)
	flowed, fits := flowRows(rows, budget, columns)
	for !fits && budget < len(rows) {
		budget++
		flowed, fits = flowRows(rows, budget, columns)
	}

	// The body pages the flow instead of squeezing it, and the selection decides
	// which page is on screen. The blank row the surface keeps above the body is
	// what separates the opening rule from the first group.
	lines, page, pages := budget, 1, 1
	if height > 0 && lines > height {
		pageHeight := max(1, height)
		pageLines := max(1, columns*pageHeight-(columns-1))
		start := 0
		if cursorRow >= 0 && len(rows) > pageLines {
			start = min((cursorRow/pageLines)*pageLines, len(rows))
		}
		flowed, _ = flowRows(rows[start:min(start+pageLines, len(rows))], pageHeight, columns)
		lines, page, pages = height, start/pageLines+1, (len(rows)+pageLines-1)/pageLines
	}

	blocks := make([][]string, columns)
	for index, column := range flowed {
		blocks[index] = m.columnLines(column, widths[index], len(column), index, canvas)
	}
	return bodyView{lines: frame.Grid(panel, blocks, widths, lines), page: page, pages: pages}
}

// flowRows pours the rows into at most columns columns of budget lines each. It
// reports false when a row had nowhere left to go.
func flowRows(rows []bodyRow, budget, columns int) ([][]bodyRow, bool) {
	flowed := make([][]bodyRow, 0, columns)
	current := make([]bodyRow, 0, budget)
	flush := func() {
		flowed = append(flowed, trimSpacer(current))
		current = make([]bodyRow, 0, budget)
	}
	fits := true
	for _, row := range rows {
		if row.kind == rowSpacer && len(current) == 0 && len(flowed) > 0 {
			continue
		}
		need := 1
		if row.kind == rowEntry && len(current) == 0 {
			need = 2 // the continuation names the group it carries
		}
		if len(current) > 0 && len(current)+need > budget && len(flowed) < columns-1 {
			flush()
			need = 1
			if row.kind == rowEntry {
				need = 2
			}
		}
		if row.kind == rowGroup && len(current) > 0 && row.height <= budget &&
			len(current)+row.height > budget && len(flowed) < columns-1 {
			flush()
		}
		if row.kind == rowSpacer && len(current) == 0 && len(flowed) > 0 {
			continue
		}
		if len(current)+need > budget && len(flowed) == columns-1 {
			fits = false
			continue
		}
		if row.kind == rowEntry && len(current) == 0 {
			current = append(current, bodyRow{kind: rowContinued, source: row.source, index: -1})
		}
		current = append(current, row)
	}
	if len(current) > 0 {
		flush()
	}
	if len(flowed) > columns {
		fits = false
	}
	return flowed, fits
}

func trimSpacer(rows []bodyRow) []bodyRow {
	if len(rows) > 0 && rows[len(rows)-1].kind == rowSpacer {
		return rows[:len(rows)-1]
	}
	return rows
}

// columnLines renders one column: the chips share one width so the descriptions
// start on the same cell in every row, and the block is padded to the frame.
func (m *Model) columnLines(rows []bodyRow, width, height, column int, canvas bodyCanvas) []string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.kind == rowEntry {
			labels = append(labels, m.keyLabel(row.entry))
		}
	}
	chipWidth := min(chip.Width(append(labels, text.Unset)...), maxChipWidth)
	chipWidth = max(chipWidth, min(text.Width(text.Unset), width-4-minDescWidth))

	lines := make([]string, 0, len(rows))
	for line, row := range rows {
		lines = append(lines, m.rowLine(row, width, chipWidth, column, line, canvas))
	}
	for height > 0 && len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return lines
}

// rowLine draws one body row at a fixed width. An entry is a list.Entry: the kit
// draws the tree guide, the chip and the tint, and records the row and its slab as
// click targets, so the guide never measures a row it drew.
func (m *Model) rowLine(row bodyRow, width, chipWidth, column, line int, canvas bodyCanvas) string {
	switch row.kind {
	case rowGroup:
		source := m.guide.Sources[row.source]
		count := countLabel(len(m.visibleEntries(row.source)), len(source.Entries()), strings.TrimSpace(m.query) != "")
		line := theme.GoldBold.Render(source.Glyph()) + " " +
			titleAt(source.Name(), source.Color()) + " " + theme.Muted.Render(count)
		return text.Fit(line, width)
	case rowContinued:
		source := m.guide.Sources[row.source]
		return text.Fit(theme.GoldBold.Render(source.Glyph())+" "+theme.Muted.Render(source.Name()+continued), width)
	case rowEmpty:
		return text.Fit(theme.Muted.Render(row.text), width)
	case rowSpacer:
		return strings.Repeat(" ", width)
	}

	entry := row.entry
	roles := m.rolesAt(row.index)
	description := m.descriptionRole(entry, roles)
	return m.drawEntry(entryRow{
		entry:       entry,
		roles:       roles,
		index:       row.index,
		last:        row.last,
		cell:        m.cell(entry, roles, chipWidth),
		description: description.Render(text.Clip(entry.Description, width-chipWidth-entryIndent-3)),
		width:       width,
		column:      column,
		line:        line,
	}, canvas)
}

// rolesAt is how one entry paints: the selection, the pointer's faint tint, or
// the panel. The selection wins where the pointer and the cursor meet, and a
// key's chip keeps the guide's own accent colour on whichever surface it sits.
func (m *Model) rolesAt(index int) list.Roles {
	roles := list.RowRolesFor(list.State{Selected: index == m.cursor, Hovered: index >= 0 && index == m.hover})
	if !roles.Selected {
		roles.Chip = theme.ChipAccent
	}
	return roles
}

// descriptionRole is how an entry's description paints: the row's primary text,
// or the row's muted tone for an entry that declares no key of its own, so a bound
// entry beside it stands out. Both follow the surface the row sits on, so the muted
// description is lifted on the hover and on the selection.
func (m *Model) descriptionRole(entry Entry, roles list.Roles) lipgloss.Style {
	if entry.Unset() {
		return roles.Muted
	}
	return roles.Name
}

// cell is an entry's key column: a chip slab, or the dim dash of an entry that
// declares no key of its own.
func (m *Model) cell(entry Entry, roles list.Roles, chipWidth int) string {
	label := m.keyLabel(entry)
	if entry.Unset() {
		return chip.UnsetCell(roles.Muted, label, chipWidth)
	}
	return chip.Cell(roles.Chip, label, chipWidth)
}

// entryRow is one entry as the body draws it: where it stands, the roles it
// paints with, its two columns and the cells it goes on.
type entryRow struct {
	entry       Entry
	roles       list.Roles
	index       int
	last        bool
	cell        string
	description string
	width       int
	column      int
	line        int
}

// drawEntry draws one entry through the kit and records the row and its chip slab
// at the cells they were drawn on, so a click on either selects the entry. An
// entry with no key of its own records its row only: there is no slab to hit.
func (m *Model) drawEntry(row entryRow, canvas bodyCanvas) string {
	x, y, drawn := canvas.at(row.column, row.line)
	painted := list.Entry{
		Roles:       row.roles,
		Last:        row.last,
		Cell:        row.cell,
		Description: row.description,
		Width:       row.width,
		X:           x,
		Y:           y,
	}
	if drawn {
		painted.ID, painted.Zones = rowZone(row.index), canvas.zones
		if !row.entry.Unset() {
			painted.ChipID = chipZone(row.index)
		}
	}
	return painted.Render()
}

// rows flattens the enabled sources into the body flow: one blank row under the
// opening rule, a header per group, one row per entry, one row between groups.
func (m *Model) rows(wrap int) []bodyRow {
	groups := make([][]bodyRow, 0, len(m.guide.Sources))
	flat := 0
	for index := range m.guide.Sources {
		group := []bodyRow{{kind: rowGroup, source: index, index: -1}}
		entries := m.visibleEntries(index)
		if len(entries) == 0 {
			for _, line := range wrapText(m.emptyBlock(m.guide.Sources[index]), wrap) {
				group = append(group, bodyRow{kind: rowEmpty, source: index, index: -1, text: line})
			}
		}
		for position, entry := range entries {
			group = append(group, bodyRow{
				kind:   rowEntry,
				source: index,
				entry:  entry,
				index:  flat,
				last:   position == len(entries)-1,
			})
			flat++
		}
		groups = append(groups, group)
	}

	rows := make([]bodyRow, 0, flat+len(groups)+len(m.guide.Sources))
	for index, group := range groups {
		group[0].height = len(group)
		if index < len(groups)-1 {
			group[0].height++ // the rule between groups is a row too
		}
		rows = append(rows, group...)
		if index < len(groups)-1 {
			rows = append(rows, bodyRow{kind: rowSpacer, index: -1})
		}
	}
	return rows
}

// listBody is the LIST view: one aligned table — chip, entry, source — with the
// same highlight and the same paging as the columns.
func (m *Model) listBody(panel frame.Frame, width, height int, canvas bodyCanvas) bodyView {
	type listRow struct {
		entry  Entry
		source string
		index  int
		last   bool
	}
	rows := make([]listRow, 0, m.shown())
	for source := range m.guide.Sources {
		entries := m.visibleEntries(source)
		for position, entry := range entries {
			rows = append(rows, listRow{
				entry:  entry,
				source: m.guide.Sources[source].Name(),
				index:  len(rows),
				last:   position == len(entries)-1,
			})
		}
	}

	sources := make([]string, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, row.source)
	}
	sourceWidth := chip.Width(append(sources, sourceLabel)...)
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, m.keyLabel(row.entry))
	}
	chipWidth := min(chip.Width(append(labels, text.Unset)...), maxChipWidth)
	chipWidth = max(chipWidth, min(text.Width(text.Unset), width-sourceWidth-tableGap-minDescWidth))

	// The table spans the whole inner width: the chip keeps its column, the entry
	// takes every cell that is left and the source ends on the inner padding edge.
	entryWidth := width - chipWidth - sourceWidth - tableGap
	if entryWidth < minDescWidth {
		chipWidth = max(text.Width(text.Unset), width-sourceWidth-tableGap-minDescWidth)
		entryWidth = max(minDescWidth, width-chipWidth-sourceWidth-tableGap)
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, m.listHeader(chipWidth, entryWidth, sourceWidth))
	lines = append(lines, theme.Quiet.Render(strings.Repeat(frame.Horizontal, width)))

	rowsPerPage := height - len(lines)
	start, page, pages := 0, 1, 1
	if rowsPerPage > 0 && len(rows) > rowsPerPage {
		if m.cursor >= 0 {
			start = min((m.cursor/rowsPerPage)*rowsPerPage, len(rows))
		}
		page, pages = start/rowsPerPage+1, (len(rows)+rowsPerPage-1)/rowsPerPage
	}
	for position := start; position < len(rows); position++ {
		if height > 0 && len(lines) >= height {
			break
		}
		lines = append(lines, m.listRowLine(rows[position].entry, rows[position].source, rows[position].index,
			rows[position].last, len(lines), width, chipWidth, entryWidth, sourceWidth, canvas))
	}
	return bodyView{lines: frame.Grid(panel, [][]string{lines}, []int{width}, len(lines)), page: page, pages: pages}
}

// listHeader names the table's columns in dim capitals, aligned to the cells
// their values start on.
func (m *Model) listHeader(chipWidth, entryWidth, sourceWidth int) string {
	return strings.Repeat(" ", entryIndent+1) + theme.Muted.Render(text.Fit(bindingLabel, chipWidth+2)) +
		" " + theme.Muted.Render(text.Fit(entryLabel, entryWidth)) +
		"  " + theme.Muted.Render(text.PadLeft(sourceLabel, sourceWidth))
}

// listRowLine draws one table row: the chip, the description and the source flush
// right, so the row fills the table's whole width. The source rides the
// description column, so list.Entry records the row as one click target.
func (m *Model) listRowLine(entry Entry, source string, index int, last bool, line, width, chipWidth, entryWidth, sourceWidth int, canvas bodyCanvas) string {
	roles := m.rolesAt(index)
	description := m.descriptionRole(entry, roles)
	return m.drawEntry(entryRow{
		entry: entry,
		roles: roles,
		index: index,
		last:  last,
		cell:  m.cell(entry, roles, chipWidth),
		description: roles.Space.Render(" ") +
			description.Render(text.Fit(text.Clip(entry.Description, entryWidth), entryWidth)) +
			roles.Space.Render("  ") + roles.Muted.Render(text.PadLeft(source, sourceWidth)),
		width:  width,
		column: 0,
		line:   line,
	}, canvas)
}

// keyLabel is what the chip shows: the binding, or a dash for an entry that
// declares no key of its own.
func (m *Model) keyLabel(entry Entry) string {
	if entry.Unset() {
		return text.Unset
	}
	return entry.Key
}

// emptyBlock is what a group with no rows says: why it is empty, or that the
// filter hides everything in it.
func (m *Model) emptyBlock(source Source) string {
	if problem, ok := source.(ProblemSource); ok && problem.Problem() != "" {
		return problem.Problem()
	}
	if strings.TrimSpace(m.query) != "" {
		return "(no match)"
	}
	return "(no entries)"
}

// emptyStateLines is the answer to the default config: every source off, so the
// screen names the switches instead of drawing a blank guide.
func (m *Model) emptyStateLines(width int) []string {
	return []string{
		"",
		theme.GoldBold.Render("Every source is off."),
		"",
		theme.Muted.Render("Turn one on under which.sources in"),
		theme.Text.Render(text.Clip(m.guide.ConfigPath, width)),
		"",
		theme.Muted.Render("hs" + separator + "gentle" + separator + "custom" + separator + "discovered"),
		"",
	}
}

// noMatchLines is the calm answer to a filter that matches nothing.
func (m *Model) noMatchLines(width int) []string {
	query := strings.TrimSpace(m.query)
	return []string{
		"",
		theme.Text.Render(text.Clip(fmt.Sprintf("No binding matches “%s”", query), width)),
		"",
		theme.Muted.Render("ctrl+u clears the filter" + separator + "esc closes the guide"),
		"",
	}
}

// centered places a short block in the middle of the body, and never taller
// than the room the terminal leaves for it.
func centered(lines []string, width, height int) []string {
	if height > 0 && len(lines) > height {
		return lines[:height]
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, text.Center(line, width))
	}
	return out
}

// countLabel is the group's count: the plain total, or shown over total while a
// filter is live.
func countLabel(shown, total int, filtered bool) string {
	if shown == total && !filtered {
		return strconv.Itoa(total)
	}
	return fmt.Sprintf("%d / %d", shown, total)
}

// wrapText breaks plain text into lines no wider than width, so a long problem
// stays readable inside its column.
func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	if width <= 0 {
		return words
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
	return append(lines, line)
}
