# herdr-utils — a container of small herdr utilities

Go plugin container for [herdr](https://herdr.dev): small, feature-toggled
utilities that are useful to many but too small to be their own plugin, plus a
curated catalog of recommended herdr plugins and a machine inventory that
installs them. Command: `hu`. Sibling of
[herdr-suite](https://github.com/WilberC/herdr-suite).

## Specs

- S1 (T1): Repo setup mirrors herdr-suite's standard ("Hagamos una buena config
  de la repo como se hizo con hs ... recuerda poner el agents.md, y todo"):
  AGENTS.md, docs/ topic split, README product landing (≤160 lines), LICENSE
  (MIT), CHANGELOG, CONTRIBUTING, SECURITY, issue templates, CODEOWNERS,
  .gitattributes/.editorconfig, CI + release workflows with the smoke job, and
  the offline-test rule (herdrmock pattern; tests never touch a socket).
- S2 (T1-T3): User: "es importante que en utils todas las herramientas tengan
  prendido y apagado porque no siempre van a querer usar todo" — every tool in
  the container is toggleable via the plugin config (`config.json`, per-tool
  keys), and tools default OFF unless the user says otherwise. The container
  itself declares one `[[startup]]` gate like herdr-suite's gs-restore.
- S3 (T2): User: "es importante que estos plugins que vamos agregando se
  mencionen en utils para que puedan instalarlos directamente" — a declarative
  catalog (`plugins.toml` in the repo, listing recommended external plugins
  with owner/repo + one-line reason) renders the README "Recommended plugins"
  section (generated, never hand-edited), and `hu plugins sync` installs the
  missing ones on this machine via `herdr plugin install` (idempotent, `--json`
  report, `--dry-run`). `hu plugins list` shows installed-vs-catalog state.
- S4 (T3, S18-style): User on hu which: "a mi en realidad lo unico que me
  importa son los key bindings ... sera mejor hacer uno nuestro en go? ...
  incluso podria ir en -utils quiza?" — approved: `hu which`, a keybinding
  guide built in Go with Bubbletea, inspired by herdr-pretty-which's interface
  (user: "la distribucion y el como se ve esta genial ... la interfaz de
  inspiracion") but reimplemented minimal: NO herdr-internal defaults, NO
  launcher (herdr-bar owns it), NO shell mode, NO theming engine.
- S5 (T3): Sources architecture — the guide renders SOURCES, each source
  declares entries (binding, description, how-to-run): `HS` (hs actions via
  `herdr plugin action list` + keybindings read from the user's herdr
  config.toml), `GENTLE` (gentle-shell commands; v1 reads what gentle exposes,
  fallback to manual entries in config), `CUSTOM` (user TOML entries),
  `DISCOVERED` (live plugin actions from installed plugins). Each source is
  toggleable: `which.sources = {hs, gentle, custom, discovered}` and the tool
  itself via `tools.which` — per S2's everything-toggles rule.
- S6 (T3): Interface contract (from the pretty-which screenshot the user
  approved): grouped multi-column layout with a count per group, live filter,
  footer hints row, KEYS/LIST views. Bubbletea; ANSI terminal colors (no theming
  engine). Popup pane declared in the manifest.
- S7 (T4): User: "recuerda que hu which tambien necesita un shortcut probablemente
  para no solo abrirlo por comando" — the manifest declares the `which` global
  action so the user binds a key (suggested: `prefix+w`), and the docs say so.
- S8 (T5): Release discipline of herdr-suite: tag `v*` → cross-built binaries +
  SHA256SUMS → smoke job. Version bumps in the manifest; tags are the owner's
  decision.

## Tasks

| ID | S# | Route | Commit | Task |
| --- | --- | --- | --- | --- |
| T1 | S1,S2 | w1 | chore(repo) | Scaffold: go.mod, cmd/hu, internal/{config,tools}, internal/herdrmock reuse pattern, herdr-plugin.toml (which action + startup gate), AGENTS.md, docs/ split, hygiene files, README landing + banner (Krea), Makefile, .gitignore, offline test harness. |
| T2 | S3 | w2 | feat(catalog) | plugins.toml catalog + generated README section + `hu plugins list/sync [--dry-run|--json]` with herdrmock-faked installs. |
| T3 | S4,S5,S6 | w3 | feat(which) | `hu which`: sources (hs/gentle/custom/discovered), toggles, Bubbletea grouped-column guide with filter and views. |
| T4 | S7 | parent | docs+config | Keybinding suggestion documented; binding added to the owner's dotfiles herdr config (prefix+w). |
| T5 | S8 | parent | release | Tag v0.1.0 after the owner confirms; release runs the smoke job. |
| T6 | S6 | w1 | style(which) | Visual redesign of the guide after the pretty-which reference: framed popup, theme roles, gold glyph family, chips, aligned columns with `(cont.)` flow, LIST table, styled empty and zero-match states. Behavior unchanged. |
| T7 | S6 | worker | pending | Migrate the `hu which` render layer to `github.com/WilberC/tui-kit` v0.1.0 (tui-kit T3, S4): compose `theme`/`text`/`chip`/`frame` and delete the private copies. Screens render as before. |
| T8 | S6,S7 | worker | pending | tui-kit v0.2.0 in `hu which`: paged hint bar (`chip.HintBar`, `chip.PageKey`) and the pinned-footer frame (`frame.Surface`/`frame.Size`) replacing the content-hugging height arithmetic, with the popup sized inside the frame's bounds (tui-kit T6, S6, S7). |
| T9 | S6 | worker | pending | tui-kit v0.3.0 in `hu which`: mouse support (a click selects, a double click runs what enter runs, hover previews, the wheel moves and pages, the paging chip is `tab`, the header KEYS/LIST names are clickable) and one copy key (`ctrl+y`, OSC 52), plus the kit's self-resetting hint bar, `text.JoinDropRight`, `list.Entry` and `Surface.PopupZones` (tui-kit T9, S8, S9). |

## Log

- L1 (2026-11-13): User approved building herdr-utils with this scope:
  "genial! Hagamos una buena config de la repo como se hiso con hs y luego suma
  la implementacion de esto (recuerda poner el agents.md, y todo) puedes usar
  krea mcp para imagenes que necesites y recuerda que hu which tambien necesita
  un shortcut probablemente para no solo abrirlo por comando".
- L2 (2026-11-13): Catalog and machine-inventory decision originated earlier
  ("es importante que estos plugins que vamos agregando se mencionen en utils
  para que puedan instalarlos directamente") and the everything-toggles rule
  ("es importante que en utils todas las herramientas tengan prendido y apagado
  porque no siempre van a querer usar todo").
- L3 (2026-11-13): hu which scope refined across the pretty-which discussion:
  keybindings-only ("lo unico que me importa son los key bindings"), own Go
  implementation over the Rust beta ("el costo de rust es fuerte"), sources
  architecture with built-ins hs + gentle-shell and expandable customs ("que
  tambien se pueda expandir cosa que podemos poner customs, y por defecto
  quisiera soportar gentle-shell commands y tambien los de hs"), and the
  everything-toggles rule restated for which's sources.
- L4 (2026-11-13): Interface inspiration registered from the user's approved
  screenshot: grouped multi-column layout with per-group counts, live filter,
  footer hints, KEYS/LIST views; in hu which the groups are the sources (HS,
  GENTLE, CUSTOM, DISCOVERED).
- L5 (2026-11-13): T2 worker report evidence: RED (build failures, then the render test caught plugins.toml resolution from the package dir); GREEN 7 packages; render section byte-idempotent between markers and pinned by a test against the real catalog. Ref pinning flagged by the worker as an assumption (herdr's `--ref`/registry keys unverified) — no seeded entry is pinned so the default sync path is unaffected; verify before adding pinned entries. Catalog seeded with the five ecosystem plugins (herdr-last and pretty-which optional). Commit follows.
- L6 (2026-11-13): T3 worker report: the worker completed code, tests and docs, then stalled in its final validation bash (root cause identified by the parent: TMPDIR pointed at a deleted scratch parent directory). Parent verified the delivered state directly: build/vet/gofmt clean, all 8 packages pass with -count=1, implementation covers S4-S6 (four sources HS/GENTLE/CUSTOM/DISCOVERED, per-source toggles default false, grouped multi-column guide with filter/views/footer). Docs were already shipped complete (docs/which.md, README rows) — no stale claims found. Commit follows.
- L7 (2026-11-13, this session): "el default en la aplicación también debe ser +i ... debe venir así" (L6 continuation: the binding must ship with the library, not live only in the owner's dotfiles). Rule S7 extended: the plugin provisions its own binding, because herdr's manifest cannot declare keys. Added internal/keybind (fenced block in the herdr config.toml, ownership line = the fence; key = prefix+i since prefix+w is herdr's workspace_picker; outside bindings reported, never touched; HERDR_CONFIG_PATH honored, missing config left missing), cmd/hu keys install|remove|status, and a [[startup]] hook step that ensures the binding before the config gate and re-installs after a config rewrite. Tests: 13 unit tests in internal/keybind (add/idempotent/legacy-repair/edited-fence-kept/outside-reported/same-key-kept/remove-strip/file round trips) + 8 in cmd/hu (install/remove/status/startup before gate/rewrite reinstall). Full suite 9 packages green, go vet clean. Docs updated (README, docs/which.md, herdr-plugin.toml comment). Risk: the hook writes the user's herdr config.toml — fenced, idempotent, no backup needed because it only inserts/strips its own lines.
- L8 (2026-11-13): "Hay que hacer un refactor unicamente en los diseños porque cumplen, pero no me gustan. Quiero que se vean muy bien, diseños bien pulidos, luxury, con calidad, creo que la TUI es muy importante para hs y para which [Image#1] mira ese ejemplo de herdr-pretty-which esa interfaz esta muy bien es una buena TUI, se distingue por colores, es claro, ordenado, etc. Nuestros diseños son muy pobres. En funcionalidad cumplen, pero en diseño dejan mucho que desear" — TUI visual redesign of hu which after the pretty-which reference
- L9 (2026-11-14): migrate the which render layer to github.com/WilberC/tui-kit v0.1.0 (tui-kit T3, S4). `internal/which` now composes the kit instead of keeping its own copies: `theme` (roles, selection, chips, `Dot`), `text` (`Clip`/`Fit`/`PadLeft`/`Join`/`Center`, glyphs), `chip` (`Pair`/`Cell`/`UnsetCell`/`Width`/`Legend`/`Hints`), `frame` (Frame `Top`/`Bottom`/`Row`/`Rule`, `ColumnRule`/`ColumnWidths`/`ColumnCount`/`Grid`). Deleted: `truncate`, `padPlain`, `padLeft`, `padStyled`, `fitStyled`, the `joinSides` placement, `topBorder`, `bottomBorder`, `plainRule`, `panelLine`, `columnRule`, `columnWidths`, `columnCount`, `assemble`, `chipPair`, `chipCell`, the legend/hint layout, and the whole palette + style block. `theme.go` is now the source-colour map plus `titleAt`; the two files went from 1046 to 786 lines (425 removed, 165 added). Evidence: two new tests RED on the base tree, GREEN here (`TestTheTealIsThePalettes`, `TestTheRuleUnderTheHeaderUsesTheGuideRole`); 40 tests in `internal/which` and 167 across the 9 packages, all green with `-count=1`; `go vet ./...` clean; `gofmt -l cmd internal` empty. Shot harness (10 states) before/after: every line cell-identical; bytes differ only on (a) the teal role, now the kit's `#73daca` instead of `#7dcfff` (header `KEYS`, `GENTLE` group title) — the intended change, (b) the rule under the header, whose body is now the guide role because `frame.Frame.Rule` follows DESIGN §1 (the private rule painted it with the border role), (c) the top border, where `frame.Frame.Top` paints the padding space with the border role (same cells). A 3420-frame sweep (5 states × widths 30–200 × heights 12/20/24/40) against the base tree found no cell difference at any width ≥ 65 besides the teal; below 65 two kit behaviours show: `text.Clip` ends an honest cut with `…` where the old `fitStyled` hard-clipped via `lipgloss MaxWidth`, and `chip.Hints` drops the remaining hints from the first that does not fit where the old loop skipped it and continued (DESIGN §3/§7, kit-side; no herdr-utils wiring can undo it). tui-kit gap reported: no join that may drop the right side — `text.Join` and `frame.Footer` clip the left and keep the right, while DESIGN §7 needs the filter legend dropped first — so `internal/which` keeps that five-line policy (`sides`) and delegates the placement. Commit pending.
- L10 (2026-11-14): tui-kit v0.2.0 in `hu which` (tui-kit T6, S6, S7): the hint bar pages and the frame is pinned, no behaviour change beyond those two. S6: the model holds a `chip.HintBar`; `handleKey` consumes `chip.PageKey` (`tab`) before the rune branch, so it pages the footer and never types into the filter, and wraps from the last page to the first; the page returns to the first one on every keystroke that changes what the footer means (view switch, live filter, cursor move). At 80 columns the footer shows `enter/↑↓/type` and ends with `tab  more 1/2`; page 2 carries the rest; at 180 everything fits on one line with no paging chip and `tab` redraws nothing. S7: `View` builds a `frame.Surface` — `Size: frame.Recommended` (60..160 x 16..40), header + rule + filter + opening column rule on top, closing rule + detail line + hint bar + notes pinned to the bottom, `Columns: widths` so the column rule runs unbroken through the padding and spare rows — and returns `Popup`, so the frame is the clamped pane centred, never the content's height. The body asks `Surface.BodyRows`: `bodyGap`/`gapColumns` are gone (the surface owns the top padding row), the column flow pages against the real row count, and the LIST table lost its own leading blank row. Popup size in `herdr-plugin.toml`: 74% x 62% (177 cells on a 240-wide terminal, past the 160 maximum, leaving a band the frame cannot use) became 66% x 62% — about 118x31 at 180x50, 158x37 at 240x60, both inside the bounds, so the clamped frame fills the popup; a manifest comment explains the numbers. Evidence: RED first (`TestTabPagesTheHintBarAndWraps`, `TestANewHintSetStartsOnItsFirstPage`, `TestTheFrameIsAsTallAsTheClampedPane` 14 vs 40 rows, `TestTheFooterIsPinnedToTheLastInnerRows`, `TestTheBodyKeepsItsPaddingRows`, `TestTheFrameNeverLeavesThePane` 180 vs 160 cells, `TestPluginPaneSizeFitsTheFrameBounds`), GREEN after the wiring; two cosmetic assertions updated for the clamp and centring (`TestTheFrameIsClosedAndEven` measures the frame, `TestTheFrameFitsEveryWindow` gained a paged-hints state). 48 tests in `internal/which` (was 40) and 176 across the 9 packages (was 167), all green with `-count=1`; `go vet ./...` clean; `gofmt -l cmd internal` empty. Shot harness gained `tall-140x50`, `short-124x37` (the owner's screenshot: a 4-entry guide in a popup-sized pane) and `paged-80`; 13 ANSI states rendered to /tmp/hu-v2/*.png with freeze and reviewed by eye: footer on the last inner rows with no empty band below it, body padded top and bottom, column rule continuous, `tab  more 1/2` where the hints do not fit. Kit feedback for tui-kit: `Surface.BodyRows` needs the caller to already know Top/Bottom, but the header row wants the body's page number — `View` fills that row after asking `BodyRows`; a `BodyRows` that also took the row counts would break the cycle. Also `HintBar` cannot tell that the hint set changed, so each product must reset `Page` itself on every state change that rewrites the footer. Commit pending.
- L11 (2026-11-14): tui-kit v0.3.0: mouse support and ctrl+y copy (tui-kit T9,
  S8, S9). S8: `Model` holds the `zone.Map` the last render recorded
  (`PopupZones`, so a click is answered in pane cells after the frame's centring,
  border, header rows and column offsets); `list.Entry` draws every entry row and
  records `row:<i>` over the whole row and `chip:<i>` over the slab,
  `chip.HintBar.RenderZones` records the bar's chips and `chip:page` for the
  paging chip, and the header records `view:keys`/`view:list` only when the right
  side was actually drawn. A click selects and never runs, in KEYS and LIST; a
  second click on the same zone within 400 ms (`Model.now`, injected by tests)
  runs `activate`, the same path enter takes — the command in the footer for
  HS/CUSTOM, the action invoked for DISCOVERED; a slower second click is a new
  selection. Motion sets `hover` and repaints with `list.RowRolesFor` (the kit's
  faint `theme.HoverBG`, no accent edge), the selection winning where both point
  at the row; the wheel is `move(±1)`, so it pages the body exactly as the cursor
  keys do; a click on `chip:page` calls `bar.Next` like `tab` (the test compares
  the two renders byte for byte); a click on a header name opens that view
  through the same `setMode` ctrl+t uses, saving it. `Run` enables
  `tea.WithMouseAllMotion`: herdr forwards a motion event to a pane only while
  the pane asked for DEC mode 1003 (`herdr src/pane/terminal.rs:2073`,
  `mode_get(MODE_MOUSE_ANY_MOTION)`), and `WithMouseCellMotion` (1002) would
  never see a hover. S9: `ctrl+y` writes `clipboard.OSC52` of the entry's command
  (HS/CUSTOM) or action id (DISCOVERED) to the screen's own writer — the
  sequence has no newline, so `tea.Printf` would scroll the frame — the detail
  line shows `copied <value>` and clears it after 1.5 s through a `tea.Tick`, and
  an entry with nothing to copy says so and writes nothing. `ctrl+y` was free:
  `handleKey` handled no ctrl+y and the rune branch never sees a named key, and
  herdr's only default control binding is `ctrl+b`. herdr forwards a pane's OSC
  52 write to the terminal it runs in (`src/client/mod.rs:12`, `src/pane.rs:2182`
  → `src/client/clipboard_forwarding.rs:14` → `src/selection.rs:312`), so the key
  works inside a popup; the outer terminal still decides whether it honours the
  sequence. S10: `go get`/`go mod tidy` to v0.3.0; the page resets in
  `cycleMode`/`move`/`setQuery` are gone (HintBar resets itself when the hint set
  changes, so a view switch keeps the page and a selection whose enter action
  differs starts over); the local `sides` helper is `text.JoinDropRight`; the
  header, filter and notes rows go through `Surface.Row`/`Rows`, entries through
  `list.Entry`, the body and the hint bar through `PopupZones`/`RenderZones`.
  Evidence: TDD — the new tests first failed to build against the v0.2.0 model
  (`Model.zones`, `rowZone`, `m.now`, `copyExpired` undefined), then failed
  behaviourally before the last fixes (the LIST row's tint broke on the plain
  gaps between the description and the source column; the seventh hint made the
  bar page at 96 columns, so `TestTheFooterNamesTheKeysAndTheEnterAction` now
  renders at 180); GREEN: 268 tests run across the 9 packages (185 top-level, 82
  subtests, 1 skipped shot harness), 0 failures, `go vet ./...` clean,
  `gofmt -l cmd internal` empty, `TMPDIR=/tmp/hu-tmp`, 66 of them in
  `internal/which`. Nine tests added — `TestAClickSelectsTheEntryUnderThePointer`
  (every row zone in both views at 180x50 and 80x24, plus the chip click),
  `TestASingleClickNeverRunsTheSelection`,
  `TestHoverTintsTheRowFaintlyAndTheSelectionWins`,
  `TestTheWheelMovesTheSelectionAndPages`,
  `TestAClickOnThePagingChipIsThePagingKey`,
  `TestAClickOnTheHeaderSwitchesTheView`,
  `TestCtrlYCopiesTheSelectionWithOSC52`, `TestCtrlYWithNothingToCopySaysSo`,
  `TestTheHintBarNamesTheCopyKey` — and `TestANewHintSetStartsOnItsFirstPage`
  became `TestTheHintPageFollowsTheHintSet`. Visual: the shot harness gained
  `hover`, `hover-80` and `copied`; 16 ANSI states rendered to /tmp/hu-v4/*.png
  with freeze and reviewed by eye (the hover tint sits one step above the panel
  and below the selection, the selection keeps its edge, the LIST tint runs
  unbroken into the source column, `ctrl+y  copy` on the bar). Accepted cell
  changes from the kit migration: entries hang off `list.Entry`'s guide, two
  cells further in than the private one and closing with `└─` per group, and the
  selected row starts `▌ ├─` instead of `▌─ `. Kit feedback: `list.Entry` has no
  third column, so the LIST table's flush-right source rides inside
  `Description`; `list.Roles` offers no accent chip role, so `rolesAt` restores
  `theme.ChipAccent` over the kit's `ChipText` to keep the keys accent-coloured.
- L12 (2026-11-14): parent review restored the muted description for entries that declare no key (descriptionRole, shared by KEYS and LIST), lost in the list.Entry move; pinned by TestTheUnboundActionKeepsItsDescriptionMuted.
