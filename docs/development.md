# Development

Back to the [README](../README.md). What the binary does is in
[Which](which.md), [Plugins](plugins.md) and [Health](health.md).

Requirements: Go 1.27 or newer. The toolchain is not pinned to a vendor; use
whatever `go` your environment provides.

```bash
make build   # go build ./...
make hu      # go build -o bin/hu ./cmd/hu, the binary the plugin runs
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

`.github/workflows/ci.yml` runs the same checks on `ubuntu-latest` and
`macos-latest` for every push and pull request: build, `go vet`, a `gofmt -l cmd
internal` that fails the job on any unformatted file, and `go test -count=1
./...`. Locally:

```bash
go build ./... && go vet ./... && test -z "$(gofmt -l cmd internal)" && go test -count=1 ./...
```

`.github/workflows/release.yml` repeats the checks once more on a `v*` tag, then
cross-compiles the four targets with `CGO_ENABLED=0`, writes `SHA256SUMS` with
`sha256sum`, and creates the GitHub release with `gh`. Each binary is built with
`-ldflags "-X main.version=<tag>"`, so it self-reports its release; a local build
keeps `dev`, which is why `hu version --check` can tell a stale binary from the
manifest it sits beside. Both workflows read the toolchain from
`go-version-file: go.mod`, so the version lives in one place, and neither needs a
release tool: the pipeline is `go`, `sha256sum` and `gh` only.

A `smoke` job runs after `publish` and checks the release rather than the working
tree: it downloads `hu-linux-amd64` and `SHA256SUMS` back from the tag it just
created, verifies the binary against the checksum, and runs `./hu-linux-amd64
version --check` and `./hu-linux-amd64 --help`. A published asset that is broken,
or stamped with a version the manifest does not declare, fails the run instead of
an install.

The matrix stops at linux and darwin, the two platforms `herdr-plugin.toml`
declares. WSL runs the linux asset, so a windows target would only add an asset
nothing here should run. `cmd/hu/release_workflow_test.go` reads both workflows:
it fails when the release matrix and the manifest platforms disagree, so a target
cannot ship from one without the other.

## Tests are offline

Tests never talk to a real herdr socket, binary or config:

- `internal/herdrmock` provides a fake `herdrmock.Client` that records every call
  and returns scripted state and errors.
- `internal/herdrcli`'s tests point `HERDR_BIN_PATH` at a stub shell script, so
  the exact arguments, the JSON parsing and the error wording are all pinned
  without a herdr on the machine.
- `cmd/hu`'s tests build a `deps` value with a fake client, a `getenv` map and a
  fixed clock, and every directory they touch is a `t.TempDir()`.
- The manifest test reads `herdr-plugin.toml` with python's `tomllib` (3.11+) and
  skips when no such interpreter is installed.

That is why `go test ./...` works in CI and on a laptop with no herdr running,
and why a test that needs a socket will not be accepted.

## Repository layout

| Path | Holds |
| --- | --- |
| `cmd/hu/` | the `hu` entry point, its command table and every CLI text |
| `internal/config/` | the plugin config document: tool and source toggles, defaults, unknown-key preservation |
| `internal/tools/` | the startup gate: the decision, the hint state, the state directory |
| `internal/herdrmock/` | the herdr surface as interfaces, and the offline fake |
| `internal/herdrcli/` | the herdr client: `plugin list --json`, `api snapshot` and the focus commands |
| `internal/manifest/` | the plugin manifest fields the binary reports: the version and the herdr floor |
| `internal/plugins/` | the recommended-plugins catalog: parsing `plugins.toml`, rendering the README section, list/sync inventory |
| `plugins.toml` | the recommended-plugins catalog the README section and the sync commands read |
| `bin/ensure-binary.sh` | provisions `bin/hu` from a local build or a release asset |
| `herdr-plugin.toml` | the herdr plugin manifest |
| `.github/workflows/` | `ci.yml`, the checks every push runs, and `release.yml`, the tagged release |
| `odd/tasks/herdr-utils.md` | the working spec and task log |

## Adding a tool to the container

1. Name the spec (`odd/tasks/herdr-utils.md`, S#) it belongs to.
2. Add its name to `internal/config` — the tool list, and the default is `false`
   — and extend the default-matrix test.
3. Add its `cmd/hu` row and usage text, and the contract cases for it.
4. If it needs herdr, add the calls to `internal/herdrmock`'s interfaces and to
   `internal/herdrcli`, with a stub-script test; never call `herdr` from
   `cmd/hu`.
5. Document it in `docs/`, with its toggle and its default.
