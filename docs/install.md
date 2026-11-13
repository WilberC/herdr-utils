# Install

Back to the [README](../README.md). Also read [Health](health.md) for the
post-install check and [Which](which.md) for the only surface v0.1.0 exposes.

The plugin requires herdr 0.7.5 or newer and installs like any other herdr
plugin:

```bash
herdr plugin install WilberC/herdr-utils
```

The floor is measured, not guessed: `herdr api snapshot` — the session read
behind `internal/herdrcli` — first shipped in herdr 0.7.2 (`src/cli/api.rs`
gained the snapshot subcommand there; 0.7.1 has no `api` dispatch), and `herdr
plugin list --json` has reported each plugin's `actions` array since plugin
actions arrived in 0.7.0. 0.7.5 covers both, which is what
`herdr-plugin.toml` declares and what `cmd/hu/plugin_manifest_test.go` pins.

Each `v*` tag publishes four static binaries — `hu-linux-amd64`,
`hu-linux-arm64`, `hu-darwin-amd64` and `hu-darwin-arm64` — next to a
`SHA256SUMS` file on [the releases
page](https://github.com/WilberC/herdr-utils/releases). `herdr plugin install`
leaves a checkout whose path `herdr plugin list --plugin herdr-utils --json`
reports as `plugin_root`; run this inside it, picking the asset for this machine:

```bash
ASSET=hu-linux-amd64   # or hu-darwin-arm64, hu-darwin-amd64, hu-linux-arm64
curl -fsSLO "https://github.com/WilberC/herdr-utils/releases/latest/download/$ASSET"
curl -fsSLO https://github.com/WilberC/herdr-utils/releases/latest/download/SHA256SUMS
grep -F "$ASSET" SHA256SUMS | sha256sum -c -   # macOS: | shasum -a 256 -c -
mkdir -p bin && install -m 755 "$ASSET" bin/hu
```

`SHA256SUMS` covers every asset, so the check reads only the line of the binary
this machine downloaded. From this point herdr's `[[build]]` hook takes over: on
`plugin install` (and update) `bin/ensure-binary.sh --in-tree` provisions
`bin/hu` — local Go build first, prebuilt release binary as fallback — while
`herdr plugin link` runs no build, so a working tree builds it with `make hu`.
Every command the manifest declares expects exactly that path.

A published binary self-reports its release, and `hu version --check` compares it
against the manifest beside it — that is the check to run after installing or
updating. See [Health](health.md).

## Running it

v0.1.0 installs no `hu` shim on `PATH`: the plugin actions run `bin/hu` from the
plugin root, and everything else runs from that directory too.

```bash
cd "$(herdr plugin list --plugin herdr-utils --json | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["plugins"][0]["plugin_root"])')"
bin/hu version
bin/hu startup        # the same gate herdr runs at server start
```

`make hu` inside a working tree writes `bin/hu` with no release version
embedded, which is why a local binary reports `hu version dev` and never fails
`hu version --check`.
