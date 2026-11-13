# Recommended plugins

Back to the [README](../README.md). The toggles it reads are in [Health § The
config file](health.md#the-config-file).

The catalog answers "which herdr plugins should I install?" once, in this
repository, instead of in every machine's notes: `plugins.toml` lists
recommended external plugins with their `owner/repo` and a one-line reason, and
that file is the source of truth.

## The README section is generated

The "Recommended plugins" table in `README.md` is rendered from `plugins.toml` by
the binary and never edited by hand. Adding a recommendation is a `plugins.toml`
entry plus a render; a reviewer never reads a hand-edited table that has drifted
from the catalog.

## The tool switch

The catalog is off like every other tool: `tools.plugins` is `false` until you
set it, so installing the container installs no third-party plugin behind your
back.

```json
{
  "tools": {"plugins": true}
}
```

## Commands

| Command | Does |
| --- | --- |
| `hu plugins list` | show every catalog plugin with its installed-or-missing state on this machine |
| `hu plugins sync` | install the catalog plugins this machine is missing, through `herdr plugin install` |
| `hu plugins sync --dry-run` | print what a sync would install, and install nothing |
| `hu plugins sync --json` | print the same report as one JSON document, for scripts |

`sync` is idempotent: a plugin already installed is reported and skipped, so a
second run installs nothing. Installing a plugin is a change to the user's
machine, so a sync that would install something says so before it does —
`--dry-run` is the cheap way to look first.

## Status in this build

The toggle, its `false` default and the command's usage are shipped and tested.
The catalog file, the generated README section and `hu plugins list`/`sync` are
the next task in this release line. Until they land `hu plugins` prints its usage
and exits 2.
