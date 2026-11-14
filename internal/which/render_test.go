package which

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// truecolor forces the renderer to emit colour, so the tests can assert which
// role a row was drawn with, and restores the profile afterwards.
func truecolor(t *testing.T) {
	t.Helper()
	before := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(before) })
}

// plainText drops the SGR escapes so a test can measure the cells it sees.
func plainText(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] == 0x1b {
			for index < len(text) && text[index] != 'm' {
				index++
			}
			continue
		}
		out.WriteByte(text[index])
	}
	return out.String()
}

// size sends the window size the way Bubbletea does, so the model lays itself
// out for a real terminal.
func size(m *Model, width, height int) *Model {
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m
}

// TestTheFrameFitsEveryWindow pins the hard edge: at a wide, a middle and a
// narrow terminal, in both views and in every state, no line leaves the window
// and the frame never grows past its height.
func TestTheFrameFitsEveryWindow(t *testing.T) {
	states := map[string]func(*Model){
		"keys":          func(*Model) {},
		"list":          func(m *Model) { m.Update(tea.KeyMsg{Type: tea.KeyCtrlT}) },
		"filtered":      func(m *Model) { m.Update(typeKey("prefix+p")) },
		"zero matches":  func(m *Model) { m.Update(typeKey("zzz")) },
		"every source":  func(m *Model) {},
		"no sources":    func(m *Model) { m.guide = Guide{} },
		"problem block": func(m *Model) {},
	}
	for name, setup := range states {
		for _, window := range []struct{ width, height int }{{180, 50}, {140, 40}, {80, 24}, {60, 20}} {
			m := New(shotGuide())
			if window.width == 80 && window.height == 24 {
				m = New(shotGuide())
			}
			setup(m)
			view := size(m, window.width, window.height).View()
			lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

			for index, line := range lines {
				if width := lipgloss.Width(line); width > window.width {
					t.Fatalf("%s at %dx%d: line %d is %d cells wide:\n%s", name, window.width, window.height, index, width, line)
				}
			}
			if len(lines) > window.height {
				t.Fatalf("%s at %dx%d: the frame is %d rows tall", name, window.width, window.height, len(lines))
			}
		}
	}
}

// TestTheFrameIsClosedAndEven pins the geometry the eye notices: every line is
// framed, the rules and the column rules line up on one cell, and a row never
// ends in spaces or repeats a space before a separator.
func TestTheFrameIsClosedAndEven(t *testing.T) {
	for _, window := range []struct{ width, height int }{{180, 50}, {140, 40}, {80, 24}} {
		view := size(New(shotGuide()), window.width, window.height).View()
		lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

		for index, line := range lines {
			if width := lipgloss.Width(line); width != window.width {
				t.Fatalf("%dx%d: line %d is %d cells wide, want %d:\n%s", window.width, window.height, index, width, window.width, line)
			}
			if !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "╭") && !strings.HasPrefix(line, "├") && !strings.HasPrefix(line, "╰") {
				t.Fatalf("%dx%d: line %d is not framed: %q", window.width, window.height, index, line)
			}
			if strings.Contains(line, "  ·") || strings.Contains(line, "·  ") {
				t.Fatalf("%dx%d: line %d doubles the space around a separator: %q", window.width, window.height, index, line)
			}
		}
	}
}

// TestTheSelectedRowKeepsItsDetail pins the selection: one row only carries the
// tint, it starts with the accent edge, and the chip slab, the key text and the
// description survive on it — in the columns and in the LIST table, source
// included.
func TestTheSelectedRowKeepsItsDetail(t *testing.T) {
	truecolor(t)

	tint := strings.SplitN(styleSelectText.Render("x"), "x", 2)[0]
	edge := strings.SplitN(styleSelectEdge.Render("x"), "x", 2)[0]
	slab := strings.SplitN(styleSelectChip.Render("x"), "x", 2)[0]

	selected := func(view string) string {
		t.Helper()
		var rows []string
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, tint) {
				rows = append(rows, line)
			}
		}
		if len(rows) != 1 {
			t.Fatalf("%d rows carry the selection tint, want exactly one:\n%s", len(rows), view)
		}
		return rows[0]
	}

	row := selected(size(New(shotGuide()), 180, 50).View())
	for _, want := range []struct{ name, text string }{
		{"the accent edge", edge + edgeMark},
		{"the chip slab", slab + " prefix+k"},
		{"the key text", "prefix+k"},
		{"the description", "Command bar"},
	} {
		if !strings.Contains(row, want.text) {
			t.Fatalf("the selected row lost %s: %q", want.name, row)
		}
	}

	m := New(shotGuide())
	press(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	listRow := selected(size(m, 180, 50).View())
	for _, want := range []struct{ name, text string }{
		{"the accent edge", edge + edgeMark},
		{"the chip slab", slab + " prefix+k"},
		{"the description", "Command bar"},
		{"the source", "HS"},
	} {
		if !strings.Contains(listRow, want.text) {
			t.Fatalf("the selected LIST row lost %s: %q", want.name, listRow)
		}
	}
}

// TestTheFooterNamesTheSelectedBinding pins the first footer line: it describes
// the entry under the cursor, and follows the cursor when it moves.
func TestTheFooterNamesTheSelectedBinding(t *testing.T) {
	m := size(New(testGuide(group{name: NameHS, entries: hsEntries()})), 100, 30)

	for _, want := range []struct{ key, description, source string }{
		{"prefix+k", "Command bar", NameHS},
		{"ctrl+shift+u", "Open limits pane", NameHS},
	} {
		view := m.View()
		footer := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
		detail := footer[len(footer)-3]

		for _, text := range []string{want.key, want.description, want.source} {
			if !strings.Contains(detail, text) {
				t.Fatalf("the footer line %q does not name %q:\n%s", detail, text, view)
			}
		}
		press(m, tea.KeyMsg{Type: tea.KeyDown})
	}
}

// TestTheUnboundActionIsDrawnUnset pins the discovered actions: they declare no
// key, so their row shows a plain dim dash in the key cell and no chip slab at
// all, and the row never claims a key it does not have.
func TestTheUnboundActionIsDrawnUnset(t *testing.T) {
	truecolor(t)

	view := size(New(testGuide(group{name: NameDiscovered, entries: []Entry{
		{Key: "enter", Description: "Open the bar", Kind: KindAction, ActionID: "herdr-bar.open"},
		{Key: "enter", Description: "Restore the session", Kind: KindAction, ActionID: "herdr-suite.restore"},
	}})), 80, 24).View()

	var row string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Restore the session") {
			row = line
		}
	}
	if row == "" {
		t.Fatalf("the discovered action has no row:\n%s", view)
	}
	if !strings.Contains(row, unsetKey) {
		t.Fatalf("the discovered action's row has no unset dash: %q", row)
	}
	if strings.Contains(row, "enter") {
		t.Fatalf("the discovered action's row shows a key it does not have: %q", row)
	}
	chipPaint := strings.SplitN(styleChip.Render("x"), "x", 2)[0]
	if strings.Contains(row, chipPaint) {
		t.Fatalf("the unset row still carries a chip slab, so the group reads as empty boxes: %q", row)
	}
}

// TestTheZeroMatchStateIsCalm pins the empty answer of a filter that matches
// nothing: one centred message and the way out, never a blank screen.
func TestTheZeroMatchStateIsCalm(t *testing.T) {
	m := New(shotGuide())
	press(m, typeKey("nothing-matches-this"))

	view := size(m, 140, 40).View()
	if !strings.Contains(view, "No binding matches") {
		t.Fatalf("the zero-match state does not say so:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+u") || !strings.Contains(view, "esc") {
		t.Fatalf("the zero-match state does not offer a way out:\n%s", view)
	}
	if strings.Contains(view, "HS 0") {
		t.Fatalf("the zero-match state still draws the empty columns:\n%s", view)
	}
}

// TestTheLISTViewIsAnAlignedTable pins the LIST view: one table with a chip, a
// description and a source column, and the same chip column on every row.
func TestTheLISTViewIsAnAlignedTable(t *testing.T) {
	guide := testGuide(
		group{name: NameHS, entries: hsEntries()},
		group{name: NameCustom, entries: []Entry{{Key: "prefix+g", Description: "Git status", Kind: KindManual}}},
	)
	m := New(guide)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	view := size(m, 140, 40).View()

	for _, label := range []string{bindingLabel, entryLabel, sourceLabel} {
		if !strings.Contains(view, label) {
			t.Fatalf("the LIST view has no %q column header:\n%s", label, view)
		}
	}

	chipAt, sourceEnd, width := -1, -1, 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, separator) {
			continue // the footer names the selection too; the table is the body
		}
		plain := plainText(line)
		key := ""
		for _, candidate := range []string{"prefix+k", "ctrl+shift+u", "prefix+g"} {
			if strings.Contains(plain, candidate) {
				key = candidate
			}
		}
		if key == "" {
			continue
		}
		width = lipgloss.Width(plain)
		if at := lipgloss.Width(plain[:strings.Index(plain, key)]); chipAt == -1 {
			chipAt = at
		} else if at != chipAt {
			t.Fatalf("the chip column starts at %d and %d, want one column:\n%s", chipAt, at, view)
		}
		// The source is flush right: every row ends on the same cell, the last one
		// inside the frame's inner padding.
		end := -1
		for _, name := range []string{"HS", "CUSTOM"} {
			if index := strings.LastIndex(plain, name); index >= 0 {
				end = lipgloss.Width(plain[:index]) + len(name)
			}
		}
		if end < 0 {
			t.Fatalf("the LIST row does not name its source: %q", plain)
		}
		if end != width-2 {
			t.Fatalf("the source ends at %d of %d, want it flush against the inner padding (cell %d):\n%s", end, width, width-3, view)
		}
		if sourceEnd == -1 {
			sourceEnd = end
		} else if end != sourceEnd {
			t.Fatalf("the source column ends at %d and %d, want one column:\n%s", sourceEnd, end, view)
		}
	}
	if chipAt < 0 || sourceEnd < 0 {
		t.Fatalf("the LIST view drew no rows:\n%s", view)
	}
	if sourceEnd <= chipAt {
		t.Fatalf("the source column starts before the chip column:\n%s", view)
	}

	// The rule under the header spans the whole table, and so does the selection
	// band: nothing is left stranded on the right.
	var rule string
	for _, line := range strings.Split(view, "\n") {
		plain := plainText(line)
		content := strings.TrimSuffix(strings.TrimPrefix(plain, "│ "), " │")
		if content != "" && strings.Trim(content, borderHorizontal) == "" {
			rule = plain
		}
	}
	if rule == "" {
		t.Fatalf("the LIST view drew no header rule:\n%s", view)
	}
	if got := strings.Count(rule, borderHorizontal); got != width-4 {
		t.Fatalf("the header rule spans %d of %d cells:\n%q", got, width-4, rule)
	}
}

// TestTheHeaderNamesThePageWhenTheFlowPages pins the page hint: a window too
// short for the flow says which page the cursor is on.
func TestTheHeaderNamesThePageWhenTheFlowPages(t *testing.T) {
	m := New(shotGuide())

	narrow := size(m, 80, 24).View()
	if !strings.Contains(narrow, "page 1 / ") {
		t.Fatalf("a paged guide does not name its page:\n%s", narrow)
	}

	wide := size(New(shotGuide()), 180, 50).View()
	if strings.Contains(wide, "page 1 / ") {
		t.Fatalf("a guide that fits claims to page:\n%s", wide)
	}
}

// TestTheFilterRowSitsAboveTheColumnRules pins the frame's order: the header's
// rule has no joints, the filter row and its legend come whole, and only then do
// the column rules open, one blank row under the first group title.
func TestTheFilterRowSitsAboveTheColumnRules(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(size(New(shotGuide()), 140, 40).View(), "\n"), "\n")

	filter, opened, title := -1, -1, -1
	for index, line := range lines {
		if strings.Contains(line, filterPrompt) && filter < 0 {
			filter = index
		}
		if strings.Contains(line, borderTopTee) && opened < 0 {
			opened = index
		}
		if strings.Contains(line, "HS 20") && title < 0 {
			title = index
		}
	}
	if filter < 0 || opened < 0 || title < 0 {
		t.Fatalf("the frame lost the filter row, the opening rule or the first group:\n%s", strings.Join(lines, "\n"))
	}
	if !(filter < opened && opened < title) {
		t.Fatalf("the frame order is filter %d, opening rule %d, first group %d", filter, opened, title)
	}
	if opened != title-2 {
		t.Fatalf("the gap between the rule and the first group is %d rows, want exactly one blank row", title-opened-1)
	}
	if strings.Contains(lines[opened-2], borderTopTee) || strings.Contains(lines[opened-2], borderBottomTee) {
		t.Fatalf("the rule under the header carries a joint: %q", lines[opened-2])
	}
	if filterLine := lines[filter]; strings.Count(filterLine, borderVertical) != 2 {
		t.Fatalf("a column rule crosses the filter row: %q", filterLine)
	}
}

// TestTheStatusMessageNeverAddsARow pins the view confirmation: switching to
// LIST changes no frame height, because the message rides the detail line.
func TestTheStatusMessageNeverAddsARow(t *testing.T) {
	m := size(New(shotGuide()), 140, 40)
	press(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	withMessage := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")

	message := m.footer
	m.footer = ""
	withoutMessage := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
	m.footer = message

	if len(withMessage) != len(withoutMessage) {
		t.Fatalf("the message grew the frame from %d to %d rows", len(withoutMessage), len(withMessage))
	}
	list := withMessage
	if !strings.Contains(m.footer, "view:") {
		t.Fatalf("the view switch recorded no message, footer = %q", m.footer)
	}
	var carriers []string
	for _, line := range list {
		if strings.Contains(line, m.footer) {
			carriers = append(carriers, line)
		}
	}
	if len(carriers) != 1 {
		t.Fatalf("the message appears on %d rows, want the detail line only:\n%s", len(carriers), strings.Join(list, "\n"))
	}
	if !strings.Contains(carriers[0], "Command bar") {
		t.Fatalf("the message is not riding the detail line: %q", carriers[0])
	}
}

// TestEveryFrameGlyphUsesTheBorderRole pins the frame's paint: borders, tees and
// joints all carry the border role, never the terminal's default foreground.
func TestEveryFrameGlyphUsesTheBorderRole(t *testing.T) {
	truecolor(t)

	view := size(New(shotGuide()), 140, 40).View()
	paint := strings.SplitN(styleBorder.Render("x"), "x", 2)[0]
	if paint == "" {
		t.Fatal("the border role rendered no colour, so the test cannot judge the frame")
	}

	joints := 0
	framed := 0
	for _, line := range strings.Split(view, "\n") {
		plain := plainText(line)
		if !strings.HasPrefix(plain, "│") && !strings.HasPrefix(plain, "├") && !strings.HasPrefix(plain, "╭") && !strings.HasPrefix(plain, "╰") {
			continue
		}
		framed++
		joints += strings.Count(line, borderTopTee) + strings.Count(line, borderBottomTee)
		for _, glyph := range []string{borderVertical, borderLeftTee, borderRightTee, borderTopTee, borderBottomTee, borderHorizontal, borderTopLeft, borderTopRight, borderBottomLeft, borderBottomRight} {
			if strings.Contains(line, "\x1b[0m"+glyph) {
				t.Fatalf("the glyph %q follows a reset, so it is painted with the terminal's default: %q", glyph, line)
			}
		}
	}
	if framed == 0 {
		t.Fatal("the frame drew nothing")
	}
	if joints == 0 {
		t.Fatal("the frame drew no joints")
	}
}

// TestChipsKeepTheirTextAndOnePlainSpace pins the chip spelling: the background
// covers the key text only, the label follows after one plain space, and items
// are separated by two plain spaces.
func TestChipsKeepTheirTextAndOnePlainSpace(t *testing.T) {
	truecolor(t)

	view := size(New(shotGuide()), 180, 50).View()
	// The column chips: the slab covers the key plus one cell of padding on each
	// side, and the description follows after exactly one plain space.
	row := styleChip.Render(" prefix+p ") + " " + styleText.Render("Plugin manager")
	if !strings.Contains(view, row) {
		t.Fatalf("the column chip is not a padded slab followed by one plain space:\n%s", view)
	}

	paint := strings.SplitN(styleHint.Render("x"), "x", 2)[0]
	if !strings.Contains(view, paint+" enter ") {
		t.Fatalf("the enter chip does not cover its key with one cell of padding on each side:\n%s", view)
	}
	hints := ""
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "show command") {
			hints = line
		}
	}
	want := styleHint.Render(" enter ") + " " + styleMuted.Render("show command") + chipGutter +
		styleHint.Render(" ↑↓ ") + " " + styleMuted.Render("move")
	if !strings.Contains(hints, want) {
		t.Fatalf("the hint bar does not use one plain space and then the two-space gutter: %q", hints)
	}
}

// TestUnsetChipsArePaddedLikeKeys pins the chip column's right edge: a dash is
// padded to the column's chip width, so every description starts on one cell.
func TestUnsetChipsArePaddedLikeKeys(t *testing.T) {
	m := New(testGuide(
		group{name: NameHS, entries: []Entry{{Key: "prefix+d", Description: "Dashboard", Kind: KindShell}}},
		group{name: NameDiscovered, entries: []Entry{{Key: "enter", Description: "Open the bar", Kind: KindAction, ActionID: "herdr-bar.open"}}},
	))
	lines := strings.Split(strings.TrimSuffix(size(m, 80, 24).View(), "\n"), "\n")

	texts := map[string]int{}
	for _, line := range lines {
		plain := plainText(line)
		if !strings.Contains(plain, treeMiddle) && !strings.Contains(plain, treeLast) && !strings.Contains(plain, edgeMark) {
			continue // the footer names the selection too; the rows are the body
		}
		for _, description := range []string{"Dashboard", "Open the bar"} {
			if index := strings.Index(plain, description); index >= 0 {
				texts[description] = lipgloss.Width(plain[:index])
			}
		}
	}
	if len(texts) != 2 {
		t.Fatalf("the two rows are not on one page: %#v", texts)
	}
	if texts["Dashboard"] != texts["Open the bar"] {
		t.Fatalf("descriptions start at %d for a bound key and %d for an unset one", texts["Dashboard"], texts["Open the bar"])
	}
}
