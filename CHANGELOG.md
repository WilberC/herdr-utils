# Changelog

All notable changes to herdr-utils are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The container: one Go binary (`hu`), one herdr plugin manifest, and a config
  file whose every tool and every source defaults to `false`.
- `hu startup`: the manifest's single `[[startup]]` gate. With every tool off it
  prints one hint per server socket and changes nothing else; with a tool on it
  stays silent. It never blocks a herdr start.
- The `which` global action, so the keybinding guide can be bound to a key.
- The herdr client seam: `internal/herdrmock` (interfaces plus an offline fake)
  and `internal/herdrcli` (`herdr plugin list --json`, `herdr api snapshot`, the
  focus commands), tested against stub scripts with no herdr running.
- `hu version` with the release version embedded at build time; `--check`
  compares the binary against the manifest so a stale install is caught.
- Release pipeline: cross-built static binaries named `hu-<os>-<arch>`,
  `SHA256SUMS`, and a smoke job that downloads its own artifact, verifies it and
  runs it.

[Unreleased]: https://github.com/WilberC/herdr-utils/commits/main
