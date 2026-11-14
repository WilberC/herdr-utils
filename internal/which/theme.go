package which

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/WilberC/tui-kit/theme"
)

// The guide's own palette decisions: the colour each source's title carries. The
// glyph of every group is gold, so the four groups read as one family; the title
// keeps the source's identity inside the palette's accents. Every other colour
// the guide draws is a role from tui-kit's theme, named at the call site.
const (
	colorHS         = theme.ColorGold
	colorGentle     = theme.ColorTeal
	colorCustom     = theme.ColorAccent
	colorDiscovered = theme.ColorGold
)

// titleAt renders a group title in its source's colour, bold, so a group header
// reads before the rows under it.
func titleAt(name string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(name)
}
