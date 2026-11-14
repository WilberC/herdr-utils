# The keybinding guide

Back to the [README](../README.md). The toggles it reads are in [Health § The
config file](health.md#the-config-file), and the plugin action it opens is
described here too.

`hu which` is the keybinding guide for herdr: one searchable screen over the
bindings you actually have, instead of four config files. It is inspired by
[herdr-pretty-which](https://github.com/WilberC/herdr-pretty-which)'s
distribution and look, and reimplemented in Go with
[Bubbletea](https://github.com/charmbracelet/bubbletea). It ships **no**
herdr-internal defaults, no launcher, no shell mode and no theming engine: the
groups are your sources, and the entries are what those sources declare.

## Sources

The guide renders *sources*. Each source declares entries — a binding, a
description and the command that runs it — and each one is toggleable on its
own:

| Source | Key | Reads |
| --- | --- | --- |
| `hs` | `which.sources.hs` | `hs` plugin actions via `herdr plugin list --json`, plus the keybindings `hs` documents in your herdr `config.toml` |
| `gentle` | `which.sources.gentle` | what gentle-shell exposes; falls back to manual entries in your config |
| `custom` | `which.sources.custom` | your own entries, declared in the plugin config |
| `discovered` | `which.sources.discovered` | the live actions every installed plugin declares |

An unknown source key is never read, so a config written by a newer build is
safe here. `internal/config` owns the names and the `false` default.

## The tool switch

The guide is off like every other tool in the container: `tools.which` is
`false` until you set it, and a source you enabled does nothing while the tool
itself is off.

```json
{
  "tools": {"which": true},
  "which": {"sources": {"hs": true, "gentle": true}}
}
```

## Opening it: the plugin binds itself

Inside herdr the guide is a plugin action, so it needs no `PATH` entry at all.
The manifest declares one global action, `which` ("Open the keybinding guide"),
whose command opens the manifest's popup pane — `hu which` is a full-screen
TUI and an action command has no terminal of its own — and the plugin binds
the action itself: herdr's manifest format cannot declare a key,
so the `[[startup]]` hook maintains a fenced block in your herdr
`config.toml`:

```toml
# --- herdr-utils: which (hu which) — begin
[[keys.command]]
key = "prefix+i"
type = "plugin_action"
command = "herdr-utils.which"
description = "Keybinding guide"
# --- herdr-utils: which (hu which) — end
```

`prefix+i` is free in a default herdr config; `prefix+w` is the workspace
picker. The fence is the ownership line: ensure replaces the block when the
key moved (after you edited it by hand or a config rewrite dropped it) and
never touches a binding of your own — one that binds the same action on
another key is reported with `hu keys install`, left for you to move or
delete. `hu keys status` says where the guide is bound and by what, and
`hu keys remove` strips the fence and nothing else.

From a shell the same screen is `bin/hu which` in the plugin root. In herdr's
plugin manager the action is listed at `prefix+p` like any other plugin action,
and action output lands in `herdr plugin log herdr-utils`.

## The screen

The guide fills the pane with one framed popup. The look it wears is[`tui-kit`](https://github.com/WilberC/tui-kit)'s: the palette, the frame and its
rules, the chips and chips bar, the tree guide and the text fitting all come from
that module, and its
[DESIGN.md](https://github.com/WilberC/tui-kit/blob/main/DESIGN.md) is the
canonical guide this screen follows — the same one herdr-suite renders with.
`internal/which` keeps only what is specific to this guide: the sources, how the
groups flow into columns, paging, and the KEYS/LIST assembly. The render code
names roles, never colours (the one exception is the colour each source's title
carries, which is the guide's own decision), so a role change moves every
surface at once.

The bands run in one order: the border with the product name set into it, the
header, a rule, the filter row with its legend, the rule that opens the columns,
the columns, the rule that closes them, then the two footer lines.

The frame is sized, not guessed. Its width and height are the pane clamped to
tui-kit's recommended range — 60 to 160 cells wide, 16 to 40 rows tall — and a
smaller pane wins, because a frame never outgrows the pane showing it. The result
is centred in the pane and is always as tall as that clamp, never as tall as its
content, so the footer sits on the frame's last rows instead of leaving empty
pane below it. The body takes the rows between the header and the footer and
always keeps one blank row at each end, so two sections stay distinguishable; the
column rules run unbroken through those rows and through the spare room below the
content. `herdr-plugin.toml` sizes this popup at 66% x 62%, which lands inside
the range on a typical terminal (about 118x31 at 180x50, 158x37 at 240x60), so
the frame fills it.

- **The header** names the guide, how many sources it read, the live view
  (`KEYS` or `LIST`) and the shown-over-total count. When the flow needs more
  rows than the body has, the header adds the page.
- **The filter row** shows the live query with a block cursor, and the legend of
  chips: a key that needs the prefix first, a direct one, and the dim dash an
  entry carries when it has no key of its own (no chip, because there is nothing
  to bind). Long queries never push the legend onto
  a second line: the legend is dropped first.
- **The body** is the sources as columns — three on a wide terminal, two in the
  middle, one when narrow — separated by rules that grow out of the horizontal
  rules, with one blank row under the rule and the same gap between groups. Each
  group is a gold glyph, its name in the source's colour and its count. Entries
  hang off a dim guide: a chip whose slab covers the key plus one cell of padding
  on each side, padded to the column's widest key, then one plain space and the
  description, so every description in a column starts on one cell. An entry that
  declares no key of its own gets no slab: a dim dash sits in the cell the key's
  first letter would use, and its description keeps the muted tone — lifted onto
  the hover and the selection tints with the rest of the row — so a bound entry
  beside it stands out. A group too tall for one column continues in the next
  under `(cont.)`. The selected row keeps every one of those details: a tint
  behind the row instead of a filled bar, an accent `▌` in the row's first cell,
  the chip slab one step brighter while the key keeps its accent colour, the
  description in bold and the dim text lifted, so the row still says which key it
  is while it is selected.
- **The footer** is two lines: what the selected entry does, the binding that
  runs it (the only green on screen), its source and its kind, then the keys the
  guide answers to. A status message — the view you just switched to, a command
  the entry runs — rides the right edge of the detail line, so it never adds a
  row. The hint bar **pages** instead of hiding a hint: when they all fit it is
  one line, and when they do not it ends with `tab` and the position
  (`more 1/2`); `tab` shows the next page and wraps around, and it never types
  into the filter. The bar itself starts over on the first page when the hints
  change — another view is not one of those, but an entry whose `enter` action
  differs is — and keeps its page while they stay the same.

The `LIST` view is the same frame with one aligned table instead: dim
`BINDING · ENTRY · SOURCE` headers over a full-width rule, a key chip, the entry
filling every cell that is left and the source in a column reserved against the
row's edge. That column is left-aligned — every value starts on the cell the
`SOURCE` header names, with one cell of air after the longest label — so a short
source does not drift to the edge and the selection or hover tint runs through
the column to the row's last cell. A filter that matches
nothing, and the config where every source is off, both answer with one centred
message instead of empty columns.

## Mouse and copy

The mouse is a second way to reach the keys, never a second interface: every
click is a key you can also press.

| Pointer | Does | Same as |
| --- | --- | --- |
| click a row, or its chip | selects that entry | moving the cursor onto it |
| double click the same row or chip, within 400 ms | runs what enter runs — the command of a binding, the action of a discovered one | `enter` |
| hover | previews the row under the pointer in a fainter tint, with no accent edge; the selection always wins | — |
| wheel | moves the selection, and pages the body when it crosses a page | `↑` / `↓` |
| click `KEYS` / `LIST` in the header | switches the view | `ctrl+t` |
| click the paging chip (`tab  more 1/2`) | shows the next page | `tab` |

Only a row, a chip, the two header view names and the paging chip are click
targets: no border, rule, title or loose text reacts to a click. A single click
never runs anything — selecting is reversible — so a slipped press cannot fire an
action. The map of what is clickable is recorded by the render itself, so the
cells a click lands on are the cells the screen drew.

`ctrl+y` copies the one value the cursor is on, written as OSC 52 so the
terminal's clipboard takes it: the command a binding or a manual entry runs, or
the action id of a live action. The detail line answers `copied …` for a moment.
Nothing is copied when the entry has no command, so the clipboard is never
cleared by accident.

Inside herdr a pane that asks for mouse reporting receives clicks, motion and the
wheel, and an OSC 52 write from the pane is forwarded to herdr's own terminal;
whether the outer terminal honours the sequence is the terminal's choice. Text
selection is left to the terminal (shift+drag): the guide does not reimplement
it.
The screen runs in [Bubbletea](https://github.com/charmbracelet/bubbletea) with
all-motion mouse reporting, so hover is possible; `internal/which/shot_test.go`
renders it at several sizes — a tall pane, a short guide in a popup-sized pane,
both views, a hovered row, a copy confirmation and the paged hint bar — to an
ANSI file when `HU_WHICH_SHOT_DIR` is set, so the look can be reviewed as an
image.
