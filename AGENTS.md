# Repository instructions

herdr-utils: small, toggleable utilities for [herdr](https://herdr.dev) — one Go
binary (`hu`) serving a CLI surface and a herdr plugin manifest. Sibling of
[herdr-suite](https://github.com/WilberC/herdr-suite), and it follows the same
rules. Public product; users install it with `herdr plugin install
WilberC/herdr-utils`.

Short by design. Each rule names what enforces it. Detail lives in the file that
owns it; this file links, never restates.

## The layout

| Directory | Holds | Never holds |
| --- | --- | --- |
| `cmd/hu/` | the entry point: the command table, dispatch, exit-code contract, CLI texts | domain logic |
| `internal/<name>/` | every package the binary is made of | CLI surfaces |
| `internal/herdrmock/` | the herdr surface as an interface, and its offline fake | production reads |
| `internal/config/` | the plugin config file: every toggle, its default, unknown-key preservation | CLI texts |
| `internal/tools/` | the startup gate and the once-per-socket hint | tool implementations |
| `docs/` | every document, one topic per file (`README` only links) | long-form content in README |
| `docs/media/` | README images | — |
| `.github/workflows/` | CI and the release pipeline | tests that live outside Go |
| `odd/tasks/` | feature specifications and the decision log | code |
| `bin/ensure-binary.sh` | provisions `bin/hu` on install and update | a second entry point |
| `herdr-plugin.toml` | the plugin manifest: build hook, startup gate, actions | logic |

Four sentences that make the rest follow.

1. **One binary (`hu`), one exit-code contract.** Every command runs through
   `run(args, stdout, stderr) int`. Exit 0 = success, 1 = operation failure,
   2 = refusal/usage. Refusals name the fix. Pinned by `cmd/hu/main_test.go`.
2. **Every tool is default off, and so is every source.** The startup gate reads
   the config and does nothing until the user opts in; with everything off it
   prints one hint per server socket. Pinned by `internal/tools/startup_test.go`.
3. **Tests never touch a socket, a terminal, the network or a user's real config.**
   The herdr surface is an interface (`internal/herdrmock`); production code
   implements it in `internal/herdrcli`, whose tests point `HERDR_BIN_PATH` at a
   stub script. Every test that writes state uses a `t.TempDir()`.
4. **Nothing user-specific enters the code.** No home paths, no user names, no
   `~/.gentle-shell` assumptions. Detection is capability-based (is the needed
   command resolvable? is the env var present?). Review rejects paths.

## What an agent must do before writing

1. **Read the spec first.** Feature work lives in `odd/tasks/herdr-utils.md`, in
   this order: header → `## Specs` (S#: the user's words, verbatim) →
   `## Tasks` → `## Log` (L#: every user correction and decision, verbatim).
   Write nothing until you can name the S#s you are covering. Report them.
2. **Consult [docs/](docs/) before changing behavior** — the docs are the
   audited truth of shipped behavior; a change that contradicts one updates that
   doc in the same commit.
3. **Check the herdr floor.** A herdr CLI surface newer than `min_herdr_version`
   in `herdr-plugin.toml` must raise it, with the version that introduced the API
   named in the commit message.
4. **Add a tool or a source through its toggle.** A tool without a
   `tools.<name>` switch, or a which source without a `which.sources.<name>`
   switch, is not ready to ship.

## The rules, and what enforces each

| Rule | Enforced by |
| --- | --- |
| **Test-first**: observe RED, then GREEN, then refactor; assert exit codes, stdout AND stderr of every command | Review; `TestRunContract`-style tables pin the contract |
| **Commands are table-driven**: a new command is a row in the shared table in `cmd/hu/main.go` plus its usage text, never a second dispatch path | Review; `plugin_manifest_test.go` refuses a manifest command the table does not hold |
| **stdout is scriptable ground.** Progress and hints go to stderr; nothing outcome-relevant moves off stdout once shipped | Review; differential checks against the base before finishing |
| **Plugin manifest stays parseable** by herdr's own parser, and its commands name only implemented commands and documented flags | `plugin_manifest_test.go`, which parses it with python's `tomllib` |
| **Every tool and every source is toggleable, and defaults off** | `internal/config/config_test.go` pins the default matrix; `internal/tools/startup_test.go` pins the gate |
| **Unknown config keys survive a save**, so a newer build's setting is never dropped by an older one | `internal/config/config_test.go` |
| **The release matrix and the manifest platforms agree** | `cmd/hu/release_workflow_test.go` reads both workflows |
| **New dependency only with a stated reason** in the commit; stdlib and in-package implementations win ties | Review |
| **No comments explaining what goes in `docs/`** — code states mechanics, docs state contracts and reasons | Review |
| **Commits are Conventional, one reviewable unit, tests and docs beside the behavior**; the decision log gains an L# per worker report | Review (the log is parent-owned: agents report, the parent appends) |
| **A push, a tag and a release are the owner's decision.** Agents never commit, never push tags | Review |

## Dev commands

```sh
mise exec go -- go build ./... && mise exec go -- go vet ./...
mise exec go -- gofmt -l cmd internal      # must print nothing
mise exec go -- go test -count=1 ./...
make hu                                    # build bin/hu (dev binary)
```

## Where the truth lives

| Question | Read |
| --- | --- |
| Install, binary download, build hook | [`docs/install.md`](docs/install.md) |
| The keybinding guide, its sources and its toggles | [`docs/which.md`](docs/which.md) |
| The recommended-plugins catalog and its commands | [`docs/plugins.md`](docs/plugins.md) |
| `hu version`, the startup gate, the config file, exit codes | [`docs/health.md`](docs/health.md) |
| Tests, workflows, release flow, repo layout | [`docs/development.md`](docs/development.md) |
| What was decided and why, for every feature | `odd/tasks/herdr-utils.md` § Log |
| How to contribute and what is reviewed | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| What changed in each release | [`CHANGELOG.md`](CHANGELOG.md) |

## Adding a command

1. Name the spec (`odd/tasks/...`, S#) it belongs to — no spec, no command.
2. Write the contract test first: usage text, exit path, stdout/stderr.
3. Add the command row to the shared table in `cmd/hu/main.go` and its usage
   constant beside it.
4. Update the topic doc in `docs/`, the README cheatsheet row (one line), and the
   exit-code table when the command introduces a new refusal class.
5. If the command can change the config, write through `internal/config` so
   unknown keys and the full default matrix are preserved.

## Releasing

1. Bump `version` in `herdr-plugin.toml` (the manifest is the source of truth).
2. Tag `v<version>` — the release workflow verifies, cross-builds (the embedded
   version comes from the tag), publishes binaries + `SHA256SUMS`, and runs the
   smoke job that downloads its own artifact.
3. A tag without the manifest bump fails the smoke job on purpose.

## Compatibility posture

- Every user-facing addition stays **default-off** until the user asks for it:
  a new tool needs a `tools.<name>` switch, a new which source needs a
  `which.sources.<name>` switch.
- Behavior other tools consume (stdout shapes, JSON keys, exit codes, plugin
  action ids, config file names and keys) is a contract: deprecate loudly, never
  break silently.
