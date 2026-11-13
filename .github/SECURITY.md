# Security policy

## Supported versions

One line: the latest release is supported, and upgrading is as simple as
re-running `herdr plugin install WilberC/herdr-utils` (or `herdr plugin update`).

## What this plugin touches

Reading what a plugin can reach is the first question. This one:

- talks to herdr through the local CLI, like every plugin, and never opens a
  socket of its own;
- reads and writes its own config file in the herdr plugin config directory,
  and one state file in the plugin state directory recording which server
  sockets already saw the startup hint;
- downloads release binaries and verifies them against `SHA256SUMS`.

It collects no telemetry, runs no background daemons, and sends nothing anywhere
except herdr's own CLI.

## How to report

Open a private security advisory through GitHub's "Report a vulnerability" on
this repository. Do not open a public issue with exploit details.
