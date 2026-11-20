# hu title

Gives a herdr workspace a short display-only title built from the agent's
first real request: "Fix login redirect loop". The title is reported through
`herdr workspace report-metadata`, so it appears as `tokens.title` under the
workspace while the label you can rename yourself stays untouched — herdr
tools and the herdr-sidecar dock (which draws it under the agent label) read
it without anything being renamed behind your back.

```bash
hu title set fix the login redirect loop in the auth service
hu title set "some request text" --workspace w3
```

Every rule of the workspace-title feature is enforced here:

- One title per workspace: a workspace that already carries a title token is
  skipped, whatever wrote it.
- Your own name always wins: a workspace whose label stopped matching the
  default herdr gave it (the basename of the workspace's agent cwd) is
  skipped.
- The request needs at least 12 characters; shorter ones never title.
- The heuristic keeps the first words within 2 to 5 words and 34 characters,
  in the language of the request, capitalized like the example above.
- Outside herdr (`HERDR_ENV` unset or not `1`) the command does nothing.
- A failed rename exits 1: run the same command again with the same text;
  nothing was applied, so the once-only rule still holds.

The tool is default off, like every tool in the container:

```json
{ "tools": { "title": true } }
```

at `$HERDR_PLUGIN_CONFIG_DIR/config.json` or the herdr-utils entry under
`$XDG_CONFIG_HOME/herdr/plugins/config`.

A caller that already redacted the title with the session's model — the short
isolated call the workspace-title feature describes — passes that text as the
request: a fitting text is kept word for word. This binary itself never calls
a model; the model path lives with the caller that owns the session.
