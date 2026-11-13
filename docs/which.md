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
and the plugin binds it itself: herdr's manifest format cannot declare a key,
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

## Status in this build

The command, its usage, its toggles and the action are shipped and tested. The
screen itself — the grouped multi-column layout with a count per group, the live
filter, the footer hints row and the `KEYS`/`LIST` views — is the next task in
this release line. Until it lands `hu which` prints its usage and exits 2, so a
script never mistakes "nothing drawn yet" for an open guide.
