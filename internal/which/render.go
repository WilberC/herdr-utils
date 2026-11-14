package which

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/WilberC/herdr-utils/internal/config"
)

// The frame's geometry. frameRows counts every row the frame spends outside the
// body: both borders, the header, the rule under it, the filter row, the rule
// that opens the columns, the rule that closes them, the detail line and the
// hints line.
const (
	frameRows     = 9
	defaultWidth  = 100
	minWidth      = 30
	minBodyHeight = 3
	maxChipWidth  = 14
	minDescWidth  = 8
)

// The frame's pieces, the tree guides and the typographic details. The group
// glyphs come from one solid family, so the four groups read as a set.
const (
	borderTopLeft     = "╭"
	borderTopRight    = "╮"
	borderBottomLeft  = "╰"
	borderBottomRight = "╯"
	borderHorizontal  = "─"
	borderVertical    = "│"
	borderLeftTee     = "├"
	borderRightTee    = "┤"
	borderTopTee      = "┬"
	borderBottomTee   = "┴"

	treeMiddle = "├─"
	treeLast   = "└─"

	unsetKey     = "–"
	edgeMark     = "▌"
	ellipsis     = "…"
	separator    = " · "
	chipGutter   = "  "
	filterGlyph  = "❱"
	cursorBlock  = "▏"
	continued    = " (cont.)"
	productName  = "herdr-utils which"
	guideSubname = "keybinding guide"
	filterPrompt = "Filter keys"
	bindingLabel = "BINDING"
	entryLabel   = "ENTRY"
	sourceLabel  = "SOURCE"
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

// View draws the guide: a framed popup with the header, the rule under it, the
// filter row, the rule that opens the columns, the grouped columns, the rule
// that closes them, and the two footer lines. It reads state only; every byte it
// needs was built before the screen opened.
func (m *Model) View() string {
	width := m.width
	if width <= 0 {
		width = defaultWidth
	}
	if width < minWidth {
		width = minWidth
	}
	innerWidth := width - 2
	textWidth := width - 4

	notes := make([]string, 0, len(m.guide.Notes))
	for _, note := range m.guide.Notes {
		notes = append(notes, styleMuted.Render(truncate(note, textWidth)))
	}

	bodyHeight := 0
	if m.height > 0 {
		bodyHeight = m.height - frameRows - len(notes)
		if bodyHeight < minBodyHeight {
			bodyHeight = minBodyHeight
		}
	}

	widths := m.layoutWidths(innerWidth)
	body := m.body(widths, bodyHeight)

	var view strings.Builder
	view.WriteString(topBorder(width))
	view.WriteString("\n")
	view.WriteString(panelLine(m.headerLine(textWidth, body), textWidth))
	view.WriteString("\n")
	view.WriteString(plainRule(innerWidth))
	view.WriteString("\n")
	view.WriteString(panelLine(m.filterLine(textWidth), textWidth))
	view.WriteString("\n")
	view.WriteString(columnRule(widths, borderTopTee))
	view.WriteString("\n")
	for _, line := range body.lines {
		view.WriteString(line)
		view.WriteString("\n")
	}
	view.WriteString(columnRule(widths, borderBottomTee))
	view.WriteString("\n")
	view.WriteString(panelLine(m.detailLine(textWidth), textWidth))
	view.WriteString("\n")
	view.WriteString(panelLine(m.hintsLine(textWidth), textWidth))
	view.WriteString("\n")
	for _, note := range notes {
		view.WriteString(panelLine(note, textWidth))
		view.WriteString("\n")
	}
	view.WriteString(bottomBorder(width))
	view.WriteString("\n")
	return view.String()
}

// topBorder draws the top edge with the product name set into it.
func topBorder(width int) string {
	budget := width - 5
	title := productName
	if lipgloss.Width(title) > budget {
		title = truncate(title, max(0, budget))
	}
	fill := max(0, budget-lipgloss.Width(title))
	return frame(borderTopLeft+borderHorizontal) + " " +
		styleProduct.Render(title) + " " +
		frame(strings.Repeat(borderHorizontal, fill)+borderTopRight)
}

// bottomBorder closes the frame.
func bottomBorder(width int) string {
	return frame(borderBottomLeft + strings.Repeat(borderHorizontal, width-2) + borderBottomRight)
}

// plainRule is the rule under the header: no joints, because the columns have
// not started yet.
func plainRule(innerWidth int) string {
	return frame(borderLeftTee + strings.Repeat(borderHorizontal, innerWidth) + borderRightTee)
}

// columnRule opens and closes the columns, with a joint above every column rule
// so the vertical rules grow out of the frame instead of floating in it.
func columnRule(widths []int, junction string) string {
	var rule strings.Builder
	for index, width := range widths {
		rule.WriteString(strings.Repeat(borderHorizontal, width+2))
		if index < len(widths)-1 {
			rule.WriteString(junction)
		}
	}
	return frame(borderLeftTee) + frame(rule.String()) + frame(borderRightTee)
}

// panelLine wraps one row of content in the frame's vertical edges, keeping one
// cell of padding on each side so the text never touches the border.
func panelLine(content string, textWidth int) string {
	return frame(borderVertical) + " " + padStyled(content, textWidth) + " " + frame(borderVertical)
}

// headerLine carries the guide's own name, the sources it read, the current view
// and how much of it is shown.
func (m *Model) headerLine(textWidth int, body bodyView) string {
	left := styleMuted.Render(guideSubname) + guide(" │ ") + styleMuted.Render(m.sourceCount())
	right := m.viewLabel(config.ViewKeys) + guide(separator) +
		m.viewLabel(config.ViewList) + guide(separator) +
		styleMuted.Render(fmt.Sprintf("%d / %d shown", m.shown(), m.total()))
	if body.pages > 1 {
		right += guide(separator) + styleMuted.Render(fmt.Sprintf("page %d / %d", body.page, body.pages))
	}
	return joinSides(left, right, textWidth)
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
		return lipgloss.NewStyle().Bold(true).Foreground(roleTeal).Render(name)
	}
	return styleMuted.Render(name)
}

// filterLine shows the live query with a block cursor, and the legend that says
// what the three kinds of key look like.
func (m *Model) filterLine(textWidth int) string {
	left := styleAccent.Render(filterGlyph + " ")
	if strings.TrimSpace(m.query) == "" {
		left += styleMuted.Render(filterPrompt + ellipsis)
	} else {
		left += styleText.Render(m.query) + styleAccent.Render(cursorBlock)
	}
	return joinSides(left, m.legend(), textWidth)
}

// legend explains the chips: a prefix binding, a direct one, and an action that
// declares no key of its own. Items are separated by two plain spaces, so no
// separator ever touches a chip's background.
func (m *Model) legend() string {
	items := []string{
		chipPair(styleChip, "x", "after prefix"),
		chipPair(styleChip, "ctrl+x", "direct"),
		styleMuted.Render(unsetKey) + " " + styleMuted.Render("unset"),
	}
	return strings.Join(items, chipGutter)
}

// chipPair is one chip and the dim label it explains: the slab covers the key
// plus one cell of padding on each side, and the label follows after exactly one
// plain space.
func chipPair(slab lipgloss.Style, key, label string) string {
	return slab.Render(" "+key+" ") + " " + styleMuted.Render(label)
}

// detailLine describes the selected entry: what it does, the binding that runs
// it, the source it came from and its kind. The binding is the only green on the
// screen. A status message sits at the right edge and never adds a row.
func (m *Model) detailLine(textWidth int) string {
	right := ""
	if m.footer != "" {
		right = styleMessage.Render(truncate(m.footer, textWidth))
	}

	row, ok := m.highlightVisible()
	if !ok {
		return joinSides(styleMuted.Render("nothing selected"+separator+"type to filter"), right, textWidth)
	}
	entry := row.entry
	key := entry.Key
	if key == "" {
		key = unsetKey
	}

	segments := []struct {
		text  string
		style lipgloss.Style
	}{
		{entry.Description, styleAccentBold},
		{key, styleGreen},
		{m.guide.Sources[row.source].Name(), styleMuted},
		{strings.ReplaceAll(entry.Kind, "_", " "), styleMuted},
	}

	var line strings.Builder
	remaining := textWidth
	for _, segment := range segments {
		if segment.text == "" {
			continue
		}
		gap := ""
		if line.Len() > 0 {
			gap = separator
		}
		needed := lipgloss.Width(gap) + lipgloss.Width(segment.text)
		if needed > remaining {
			if line.Len() > 0 {
				break
			}
			segment.text = truncate(segment.text, remaining)
			needed = lipgloss.Width(segment.text)
			if needed == 0 {
				break
			}
		}
		line.WriteString(guide(gap))
		line.WriteString(segment.style.Render(segment.text))
		remaining -= needed
	}
	if right != "" && lipgloss.Width(line.String())+lipgloss.Width(right)+1 <= textWidth {
		return joinSides(line.String(), right, textWidth)
	}
	return line.String()
}

// hintsLine names the keys the guide answers to, as chips. A hint that does not
// fit is left out rather than wrapped.
func (m *Model) hintsLine(textWidth int) string {
	enter := "show command"
	if entry, ok := m.highlight(); ok && entry.Invokable() {
		enter = "run action"
	}
	items := []struct{ key, label string }{
		{"enter", enter},
		{"↑↓", "move"},
		{"type", "filter"},
		{"ctrl+t", "view"},
		{"esc", "close"},
		{"ctrl+u", "clear"},
	}

	var line strings.Builder
	used := 0
	for _, item := range items {
		piece := chipPair(styleHint, item.key, item.label)
		gap := ""
		if used > 0 {
			gap = chipGutter
		}
		if used+lipgloss.Width(gap)+lipgloss.Width(piece) > textWidth {
			continue
		}
		line.WriteString(gap)
		line.WriteString(piece)
		used += lipgloss.Width(gap) + lipgloss.Width(piece)
	}
	return line.String()
}

// layoutWidths is how the body is split: one column for the states that hold a
// single centred block or the table, the earned columns otherwise.
func (m *Model) layoutWidths(innerWidth int) []int {
	if m.mode == config.ViewList || len(m.guide.Sources) == 0 || m.filteredToNothing() {
		return []int{innerWidth - 2}
	}
	return columnWidths(innerWidth, columnCount(innerWidth, len(m.guide.Sources)))
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

// body draws the columns, the table, the empty state or the zero-match state.
func (m *Model) body(widths []int, height int) bodyView {
	if m.mode == config.ViewList {
		return m.listBody(widths[0], height)
	}
	blocks := make([][]string, len(widths))
	switch {
	case len(m.guide.Sources) == 0:
		blocks[0] = centered(m.emptyStateLines(widths[0]), widths[0], height)
	case m.filteredToNothing():
		blocks[0] = centered(m.noMatchLines(widths[0]), widths[0], height)
	default:
		return m.columns(widths, height)
	}
	lines := 0
	for _, block := range blocks {
		lines = max(lines, len(block))
	}
	return bodyView{lines: assemble(blocks, widths, lines), page: 1, pages: 1}
}

// columns lays the groups out: the smallest column budget that holds the whole
// flow wins, so a short guide still spreads over the columns the terminal
// earned instead of stacking in the first one. A group that does not fit in the
// space left moves to the next column whole; only a group taller than a column
// is continued there under its own "(cont.)" header.
func (m *Model) columns(widths []int, height int) bodyView {
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

	// The frame hugs its content; a full terminal pages the flow instead, and the
	// selection decides which page is on screen. One row of every column is the
	// blank line that separates the opening rule from the first group.
	lines, page, pages := budget+bodyGap, 1, 1
	if height > 0 && lines > height {
		pageHeight := max(1, height-bodyGap)
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
		blocks[index] = m.columnLines(column, widths[index], len(column))
	}
	return bodyView{lines: assemble(gapColumns(blocks, widths, columns), widths, lines), page: page, pages: pages}
}

// bodyGap is the single blank row that separates the rule opening the columns
// from the first group title, so the gap above a group is always one row.
const bodyGap = 1

// gapColumns puts that blank row on top of every column, so the columns start on
// the same line as the group they carry.
func gapColumns(blocks [][]string, widths []int, columns int) [][]string {
	out := make([][]string, columns)
	for index := range out {
		out[index] = append([]string{strings.Repeat(" ", widths[index])}, blocks[index]...)
	}
	return out
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

// assemble joins the column blocks row by row, drawing the rule between them so
// it reaches both horizontal rules.
func assemble(blocks [][]string, widths []int, height int) []string {
	lines := height
	if lines <= 0 {
		for _, block := range blocks {
			lines = max(lines, len(block))
		}
	}
	out := make([]string, 0, lines)
	for line := 0; line < lines; line++ {
		var row strings.Builder
		row.WriteString(frame(borderVertical))
		for index, width := range widths {
			content := ""
			if line < len(blocks[index]) {
				content = blocks[index][line]
			}
			row.WriteString(" ")
			row.WriteString(padStyled(content, width))
			row.WriteString(" ")
			if index < len(widths)-1 {
				row.WriteString(frame(borderVertical))
			}
		}
		row.WriteString(frame(borderVertical))
		out = append(out, row.String())
	}
	return out
}

// columnLines renders one column: the chips share one width so the descriptions
// start on the same cell in every row, and the block is padded to the frame.
func (m *Model) columnLines(rows []bodyRow, width, height int) []string {
	chipWidth := lipgloss.Width(unsetKey)
	for _, row := range rows {
		if row.kind == rowEntry {
			chipWidth = max(chipWidth, lipgloss.Width(m.keyLabel(row.entry)))
		}
	}
	chipWidth = min(chipWidth, maxChipWidth)
	chipWidth = max(chipWidth, min(lipgloss.Width(unsetKey), width-4-minDescWidth))

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, m.rowLine(row, width, chipWidth))
	}
	for height > 0 && len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return lines
}

// rowLine draws one body row at a fixed width.
func (m *Model) rowLine(row bodyRow, width, chipWidth int) string {
	switch row.kind {
	case rowGroup:
		source := m.guide.Sources[row.source]
		count := countLabel(len(m.visibleEntries(row.source)), len(source.Entries()), strings.TrimSpace(m.query) != "")
		line := styleGold.Render(source.Glyph()) + " " +
			titleAt(source.Name(), source.Color()) + " " + styleMuted.Render(count)
		return padStyled(line, width)
	case rowContinued:
		source := m.guide.Sources[row.source]
		return padStyled(styleGold.Render(source.Glyph())+" "+styleMuted.Render(source.Name()+continued), width)
	case rowEmpty:
		return padStyled(styleMuted.Render(row.text), width)
	case rowSpacer:
		return strings.Repeat(" ", width)
	}

	entry := row.entry
	label := m.keyLabel(entry)
	unset := entry.Unset()
	description := truncate(entry.Description, width-chipWidth-6)
	tree := guide(treeMiddle)
	if row.last {
		tree = guide(treeLast)
	}

	if row.index == m.cursor {
		return m.selectedRow(label, unset, description, width, chipWidth)
	}

	descriptionStyle := styleText
	slab := styleChip
	if unset {
		descriptionStyle = styleMuted
	}
	return padStyled(tree+" "+chipCell(label, chipWidth, slab, unset)+descriptionStyle.Render(description), width)
}

// selectedRow draws the selected entry on the selection tint: the accent edge in
// the tree's first cell, the chip slab one step brighter, the description in bold
// text and any secondary text lifted, so the row keeps every detail it shows when
// it is not selected. The tint runs to the column's last cell.
func (m *Model) selectedRow(label string, unset bool, description string, width, chipWidth int) string {
	var line strings.Builder
	line.WriteString(styleSelectEdge.Render(edgeMark))
	line.WriteString(styleSelectMuted.Render(borderHorizontal))
	line.WriteString(styleSelectText.Render(" "))
	if unset {
		line.WriteString(styleSelectMuted.Render(padPlain(" "+label, chipWidth+3)))
		line.WriteString(styleSelectMuted.Render(description))
	} else {
		line.WriteString(styleSelectChip.Render(" " + padPlain(label, chipWidth) + " "))
		line.WriteString(styleSelectText.Render(" " + description))
	}
	if fill := width - chipWidth - lipgloss.Width(description) - 6; fill > 0 {
		line.WriteString(styleSelectText.Render(strings.Repeat(" ", fill)))
	}
	return line.String()
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
func (m *Model) listBody(width, height int) bodyView {
	type listRow struct {
		entry  Entry
		source string
		index  int
	}
	rows := make([]listRow, 0, m.shown())
	for source := range m.guide.Sources {
		for _, entry := range m.visibleEntries(source) {
			rows = append(rows, listRow{entry: entry, source: m.guide.Sources[source].Name(), index: len(rows)})
		}
	}

	chipWidth, sourceWidth := lipgloss.Width(unsetKey), lipgloss.Width(sourceLabel)
	for _, row := range rows {
		chipWidth = max(chipWidth, lipgloss.Width(m.keyLabel(row.entry)))
		sourceWidth = max(sourceWidth, lipgloss.Width(row.source))
	}
	chipWidth = min(chipWidth, maxChipWidth)
	chipWidth = max(chipWidth, min(lipgloss.Width(unsetKey), width-sourceWidth-6-minDescWidth))

	// The table spans the whole inner width: the chip keeps its column, the entry
	// takes every cell that is left and the source ends on the inner padding edge.
	entryWidth := width - chipWidth - sourceWidth - 6
	if entryWidth < minDescWidth {
		chipWidth = max(lipgloss.Width(unsetKey), width-sourceWidth-6-minDescWidth)
		entryWidth = max(minDescWidth, width-chipWidth-sourceWidth-6)
	}

	lines := make([]string, 0, len(rows)+3)
	lines = append(lines, "")
	lines = append(lines, m.listHeader(chipWidth, entryWidth, sourceWidth))
	lines = append(lines, guide(strings.Repeat(borderHorizontal, width)))

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
			width, chipWidth, entryWidth, sourceWidth))
	}
	return bodyView{lines: assemble([][]string{lines}, []int{width}, len(lines)), page: page, pages: pages}
}

// listHeader names the table's columns in dim capitals, aligned to the cells
// their values start on.
func (m *Model) listHeader(chipWidth, entryWidth, sourceWidth int) string {
	line := "  " + styleMuted.Render(padPlain(bindingLabel, chipWidth+1)) +
		" " + styleMuted.Render(padPlain(entryLabel, entryWidth)) +
		"  " + styleMuted.Render(padLeft(sourceLabel, sourceWidth))
	return line
}

// listRowLine draws one table row: the chip, the description and the source flush
// right, so the row fills the table's whole width.
func (m *Model) listRowLine(entry Entry, source string, index, width, chipWidth, entryWidth, sourceWidth int) string {
	label := m.keyLabel(entry)
	description := truncate(entry.Description, entryWidth)
	name := padLeft(source, sourceWidth)

	if index == m.cursor {
		var line strings.Builder
		line.WriteString(styleSelectEdge.Render(edgeMark))
		if entry.Unset() {
			line.WriteString(styleSelectMuted.Render(padPlain(" "+label, chipWidth+3)))
			line.WriteString(styleSelectMuted.Render(padPlain(description, entryWidth) + "  " + name))
		} else {
			line.WriteString(styleSelectChip.Render(" " + padPlain(label, chipWidth) + " "))
			line.WriteString(styleSelectText.Render(" " + padPlain(description, entryWidth) + "  "))
			line.WriteString(styleSelectMuted.Render(name))
		}
		return padStyled(line.String(), width)
	}

	slab := styleChip
	descriptionStyle := styleText
	if entry.Unset() {
		descriptionStyle = styleMuted
	}
	return padStyled(" "+chipCell(label, chipWidth, slab, entry.Unset())+
		descriptionStyle.Render(padPlain(description, entryWidth))+"  "+styleMuted.Render(name), width)
}

// chipCell renders the key cell of one row: the slab covers the key plus one cell
// of padding on each side, followed by the one plain space that separates it from
// the description. An entry that declares no key gets no slab at all — a dim dash
// in the cell the key's first letter would use, padded with plain spaces — so a
// group of discovered actions never reads as a wall of empty boxes.
func chipCell(label string, chipWidth int, slab lipgloss.Style, unset bool) string {
	if unset {
		return styleMuted.Render(padPlain(" "+label, chipWidth+3))
	}
	return slab.Render(" "+padPlain(label, chipWidth)+" ") + " "
}

// keyLabel is what the chip shows: the binding, or a dash for an entry that
// declares no key of its own.
func (m *Model) keyLabel(entry Entry) string {
	if entry.Unset() {
		return unsetKey
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
		styleProduct.Render("Every source is off."),
		"",
		styleMuted.Render("Turn one on under which.sources in"),
		styleText.Render(truncate(m.guide.ConfigPath, width)),
		"",
		styleMuted.Render("hs" + separator + "gentle" + separator + "custom" + separator + "discovered"),
		"",
	}
}

// noMatchLines is the calm answer to a filter that matches nothing.
func (m *Model) noMatchLines(width int) []string {
	query := strings.TrimSpace(m.query)
	return []string{
		"",
		styleText.Render(truncate(fmt.Sprintf("No binding matches “%s”", query), width)),
		"",
		styleMuted.Render("ctrl+u clears the filter" + separator + "esc closes the guide"),
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
		line = fitStyled(line, width)
		out = append(out, strings.Repeat(" ", max(0, (width-lipgloss.Width(line))/2))+line)
	}
	return out
}

// columnCount is how many columns the width earns: three when the terminal is
// wide, two in the middle, one otherwise, and never more than the guide holds.
func columnCount(innerWidth, sources int) int {
	count := 1
	switch {
	case innerWidth >= 148:
		count = 3
	case innerWidth >= 98:
		count = 2
	}
	if sources > 0 {
		count = min(count, sources)
	}
	return count
}

// columnWidths splits the inner width into columns with one cell of rule between
// them. Each column spends one cell of padding on each side of its content.
func columnWidths(innerWidth, count int) []int {
	total := innerWidth - 2*count - (count - 1)
	base, rest := total/count, total%count
	widths := make([]int, count)
	for index := range widths {
		widths[index] = base
		if index < rest {
			widths[index]++
		}
	}
	return widths
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
