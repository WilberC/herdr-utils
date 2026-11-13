# Version, the startup gate and exit codes

Back to the [README](../README.md). Installation itself is in
[Install](install.md), and the tools the gate reads about are in
[Which](which.md) and [Plugins](plugins.md).

## Version

A published binary knows its own release: the release workflow stamps the tag
into it with `-ldflags "-X main.version=<tag>"`, so `hu version` reports the tag
it was built from. A build from a working tree embeds nothing and keeps the
`dev` default — `make hu` passes no ldflags on purpose.

```bash
hu version                      # hu version 0.1.0, or hu version dev in a tree
hu version --json               # {"version":"0.1.0"}
hu version --check              # hu version 0.1.0 · manifest 0.1.0 · ok, exit 1 when the binary is stale
```

`hu version --check` reads the manifest beside the binary — the plugin root above
`bin/hu`, then the working directory; `--manifest <path>` overrides both — and
exits 0 when the two versions match, 1 when the binary is stale, naming both
values and the fix. A dev build has no release version to compare, so it reports
`dev build; no release version embedded` and always exits 0: a working tree is
never failed by its own build. This is the check a script runs after installing
or updating the plugin.

## The config file

The plugin config is JSON, at `$HERDR_PLUGIN_CONFIG_DIR/config.json` when herdr
supplies it, otherwise
`$XDG_CONFIG_HOME/herdr/plugins/config/herdr-utils/config.json`, then
`~/.config/herdr/plugins/config/herdr-utils/config.json`. A missing file is not an
error: it means every toggle is off, which is the documented default.

```json
{
  "tools": {"which": false, "plugins": false},
  "which": {"sources": {"hs": false, "gentle": false, "custom": false, "discovered": false}}
}
```

That is the whole documented document, and every value in it defaults to
`false`. A tool's switch is its key under `tools`; a which source's switch is its
key under `which.sources`. A key this build does not know is kept verbatim when
the file is rewritten, so a config written by a newer build survives a round trip
through an older one, and an unknown switch is simply off.

A malformed config never breaks an operation that only reads it: the startup gate
reports the parse error on stderr once and behaves as if every tool were off.

## The startup gate

The manifest declares exactly one `[[startup]]` hook:

```toml
[[startup]]
command = ["bin/hu", "startup"]
```

herdr runs it at server start, and it obeys the config:

| Config | What happens |
| --- | --- |
| every tool off (the default, including no config file) | one hint on stderr per server socket, then silence for that socket |
| any tool on | nothing at all: there is nothing to start or install |
| unreadable config | one line naming the parse error, then every tool stays off |

```text
herdr-utils: every tool is off; enable one in /home/me/.config/herdr/plugins/config/herdr-utils/config.json (for example tools.which = true for the keybinding guide)
```

The hint is recorded in `$HERDR_PLUGIN_STATE_DIR/startup-hint.json`, otherwise
`$XDG_STATE_HOME/herdr/plugins/herdr-utils/startup-hint.json`, then
`~/.local/state/herdr/plugins/herdr-utils/startup-hint.json`. It is keyed by
`$HERDR_SOCKET_PATH` (or `default`), so a restarted session stays quiet while a
new server is mentioned once. The file holds no user data beyond those socket
paths and timestamps, and deleting it is always safe: the next start hints again.

The gate never blocks a herdr start, and it writes to stderr only — stdout stays
scriptable ground.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | the operation succeeded, or `hu version --check` found a matching version, or a dev build reported itself |
| `1` | herdr failed, or `hu version --check` found a stale binary |
| `2` | bad arguments, an unknown command, an unknown flag, or a command that is not implemented in this build (it prints its usage) |

Inside herdr the plugin exposes the `which` action — see [Which](which.md).
