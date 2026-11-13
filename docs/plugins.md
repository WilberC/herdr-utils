# Recommended plugins

Back to the [README](../README.md). The toggles are in [Health § The config
file](health.md#the-config-file); installation of this plugin itself is in
[Install](install.md).

The catalog answers "which herdr plugins should I install?" once, in this
repository, instead of in every machine's notes: `plugins.toml` lists
recommended external plugins with their `owner/repo` and a one-line reason, and
that file is the source of truth. `hu plugins list` compares it with this
machine, and `hu plugins sync` installs the missing entries.

## The catalog

`plugins.toml` lives beside the binary and in the repository root. Each
`[[plugin]]` entry declares:

| Key | Required | Meaning |
| --- | --- | --- |
| `repo` | yes | the plugin's `owner/repo`, exactly as `herdr plugin install` takes it |
| `reason` | yes | the one line that explains why it is recommended, rendered into the README |
| `optional` | no | `true` keeps the entry out of a default sync; `--optional` includes it |
| `ref` | no | pins the version a sync installs (`--ref <ref>`) and that list compares against |

A malformed entry — no `repo`, no `reason`, a repo that is not `owner/repo`, an
unknown key, or the same repo twice — refuses the whole catalog instead of
silently shrinking it: the commands name the entry and exit 1.

## The README section is generated

The "Recommended plugins" table in `README.md` is rendered from `plugins.toml`
and never edited by hand. The section lives between two markers:

```html
<!-- generated:recommended start -->
<!-- generated:recommended end -->
```

Maintainers add or change a `[[plugin]]` entry, then run:

```bash
hu plugins render                          # rewrite README.md between the markers
hu plugins render --readme path/to/file    # render somewhere else
```

`render` leaves everything outside the markers untouched, writes only when the
section changed (a second run reports `is current`), and refuses a README whose
markers are missing, duplicated or out of order. `go test ./cmd/hu` fails when
the committed section and the catalog disagree, so a hand edit cannot land.

## Commands

| Command | Does |
| --- | --- |
| `hu plugins list` | show every catalog plugin with its installed-or-missing state on this machine |
| `hu plugins list --json` | the same inventory as one JSON document, for scripts |
| `hu plugins sync` | install the core catalog plugins this machine is missing, through `herdr plugin install` |
| `hu plugins sync --optional` | include the entries the catalog marks optional |
| `hu plugins sync --dry-run` | print the exact commands a sync would run, and run none |
| `hu plugins sync --json` | print the same report as one JSON document, for scripts |

`list` exits 0 whether or not anything is installed: an empty machine is a valid
inventory, not a failure. Its table has one row per catalog entry — plugin,
state, kind (`core`/`optional`), ref state and the reason — and the JSON document
carries the same fields, including `ref_match` only for a pinned entry.

`sync` installs every missing core entry with
`herdr plugin install <owner/repo> -y`, appending `--ref <ref>` for a pinned
entry. A failed install is reported on its own row and the remaining entries
still run; the summary line (`installed n, failed m, skipped k, current c`) says
what happened, and the exit status is 1 when any entry failed. A dry run prints
the exact commands, installs nothing and reports `would install n`.

`sync` is idempotent: an entry herdr already reports is `current` and is not
installed again, so a second run changes nothing. Installed is installed — a
pinned entry whose ref differs is still reported `current`, with the mismatch in
its detail; `list` is where the ref comparison drives the `match`/`mismatch`
column.

## Explicit commands ignore the toggles

`tools.plugins` is `false` by default, and installing the container installs no
third-party plugin behind your back. But toggles gate the `[[startup]]`
automation, never explicit commands: `hu plugins list`, `hu plugins sync` and
`hu plugins render` run regardless of `tools.plugins`, because you typed them.
The switch exists so nothing installs plugins on its own at server start.

## Where the catalog comes from

`hu` looks for `plugins.toml` beside its own binary, then at the plugin root
above `bin/hu`, then in the working directory — the same lookup `hu version
--check` uses for the manifest. Whoever runs the command from the repository root
reads the repository's catalog.
