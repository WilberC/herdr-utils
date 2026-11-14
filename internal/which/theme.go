package which

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The palette. Every colour the guide draws comes from a role here — the render
// code names roles, never raw hex — so the look changes in this one file. It is
// Tokyo Night: the terminal's own background is left alone, and the accents stay
// limited to blue, gold and teal.
const (
	roleText   lipgloss.Color = "#c0caf5"
	roleMuted  lipgloss.Color = "#565f89"
	roleGuide  lipgloss.Color = "#3b4261"
	roleBorder lipgloss.Color = "#3d59a1"
	roleAccent lipgloss.Color = "#7aa2f7"
	roleGold   lipgloss.Color = "#e0af68"
	roleTeal   lipgloss.Color = "#7dcfff"
	roleGreen  lipgloss.Color = "#9ece6a"
	roleChipBG lipgloss.Color = "#2f3549"

	// The selection is a tint, never a fill: the row keeps every element it has
	// when it is not selected, lifted so each role still reads on the tint.
	roleSelectionBG   lipgloss.Color = "#283457"
	roleSelectionEdge lipgloss.Color = roleAccent
	roleChipOnSelect  lipgloss.Color = "#3d59a1"
	roleMutedOnSelect lipgloss.Color = "#737aa2"
)

// The colours each source's title carries. The glyph of every group is gold, so
// the four groups read as one family; the title keeps the source's identity
// inside the three accent roles.
const (
	colorHS         lipgloss.Color = roleGold
	colorGentle     lipgloss.Color = roleTeal
	colorCustom     lipgloss.Color = roleAccent
	colorDiscovered lipgloss.Color = roleGold
)

// The styles the guide draws with, by role: text, muted and guide for the
// hierarchy, accent/gold/teal for the accents, green only for a binding.
var (
	styleProduct    = lipgloss.NewStyle().Bold(true).Foreground(roleGold)
	styleText       = lipgloss.NewStyle().Foreground(roleText)
	styleMuted      = lipgloss.NewStyle().Foreground(roleMuted)
	styleAccent     = lipgloss.NewStyle().Foreground(roleAccent)
	styleAccentBold = lipgloss.NewStyle().Bold(true).Foreground(roleAccent)
	styleGold       = lipgloss.NewStyle().Bold(true).Foreground(roleGold)
	styleGreen      = lipgloss.NewStyle().Foreground(roleGreen)
	styleMessage    = lipgloss.NewStyle().Foreground(roleGold)
	styleBorder     = lipgloss.NewStyle().Foreground(roleBorder)
	styleRule       = lipgloss.NewStyle().Foreground(roleGuide)

	// A key chip: dark slab, bold accent label, one cell of padding each side.
	styleChip = lipgloss.NewStyle().Background(roleChipBG).Foreground(roleAccent).Bold(true)
	styleHint = lipgloss.NewStyle().Background(roleChipBG).Foreground(roleText).Bold(true)

	// The selected row: a tint behind the whole row, the accent edge in its first
	// cell, the chip slab one step brighter, the description in bold text and any
	// secondary text lifted, so nothing on the row turns into a flat block.
	styleSelectEdge  = lipgloss.NewStyle().Background(roleSelectionBG).Foreground(roleSelectionEdge).Bold(true)
	styleSelectChip  = lipgloss.NewStyle().Background(roleChipOnSelect).Foreground(roleText).Bold(true)
	styleSelectText  = lipgloss.NewStyle().Background(roleSelectionBG).Foreground(roleText).Bold(true)
	styleSelectMuted = lipgloss.NewStyle().Background(roleSelectionBG).Foreground(roleMutedOnSelect)
)

// frame draws one frame glyph — a border, a tee or a rule — with the border
// role. The glyphs are rendered at draw time: a string rendered once at init
// would freeze the colour profile the process started with and fall back to the
// terminal's default foreground.
func frame(text string) string { return styleBorder.Render(text) }

// guide draws a tree guide or a quiet separator.
func guide(text string) string { return styleRule.Render(text) }

// titleAt renders a group title in its source's colour.
func titleAt(name string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(name)
}

// truncate cuts plain text to a cell budget and ends an honest cut with an
// ellipsis instead of wrapping or clipping mid-cell.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return ellipsis
	}
	used := 0
	for index, r := range text {
		cell := lipgloss.Width(string(r))
		if used+cell > width-1 {
			return text[:index] + ellipsis
		}
		used += cell
	}
	return text
}

// padPlain fills plain text out to a cell width, truncating first when it is
// longer. It never touches styled text: escapes would be cut mid-sequence.
func padPlain(text string, width int) string {
	text = truncate(text, width)
	if fill := width - lipgloss.Width(text); fill > 0 {
		text += strings.Repeat(" ", fill)
	}
	return text
}

// padLeft fills plain text out to a cell width on its left, so the text ends on
// the last cell of its column.
func padLeft(text string, width int) string {
	text = truncate(text, width)
	if fill := width - lipgloss.Width(text); fill > 0 {
		return strings.Repeat(" ", fill) + text
	}
	return text
}

// fitStyled is the last guard before a line enters the frame: it cuts styled
// text to the budget without wrapping it.
func fitStyled(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}

// padStyled fills already-styled text out to a cell width with plain spaces, so
// the frame's vertical rules stay in one column.
func padStyled(text string, width int) string {
	text = fitStyled(text, width)
	if fill := width - lipgloss.Width(text); fill > 0 {
		text += strings.Repeat(" ", fill)
	}
	return text
}

// joinSides places one part at the left edge and one at the right edge of the
// budget. The right part is dropped before the left one is cut: the guide's
// header keeps its title, and the legend disappears first.
func joinSides(left, right string, width int) string {
	if right != "" && lipgloss.Width(left)+lipgloss.Width(right)+1 <= width {
		fill := width - lipgloss.Width(left) - lipgloss.Width(right)
		return left + strings.Repeat(" ", fill) + right
	}
	return padStyled(left, width)
}
