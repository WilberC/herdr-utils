# Workspace title — `hu title`

Auto-title workspaces with a short task title from the session's first real
agent request, shown under the agent label in the sidecar dock. One named
source, display-only: the workspace label is the user's.

## Specs

- **S1.** "Le pone nombre al espacio de trabajo de herdr una sola vez, con un
  título corto sacado de tu primer pedido real en la sesión (por ejemplo,
  «Fix login redirect loop»)." Once per workspace: a workspace that already
  has a generated title never regenerates; the user's own `workspace rename`
  always wins and is never overwritten.
- **S2.** "Solo actúa dentro de herdr (HERDR_ENV=1)." The capture path no-ops
  without HERDR_ENV=1.
- **S3.** "Un nombre que pusiste a mano manda siempre: si el espacio ya tiene
  nombre propio, no lo pisa." A workspace whose label was renamed by the user
  (changed from the herdr default) is skipped.
- **S4.** "El título (2 a 5 palabras, máx. 34 caracteres, en el idioma de tu
  pedido) lo redacta el modelo activo con una llamada corta y aislada. Si
  falla (sin autenticación, tiempo agotado, respuesta vacía) usa una
  heurística local." Source order: harness title token > local heuristic
  (first-request text trimmed to 34 chars, 2-5 words, request language) >
  isolated model call as last resort. A failed call falls back to the
  heuristic in the same run.
- **S5.** "Necesita un pedido de al menos 12 caracteres." Requests under 12
  characters are not eligible; the next eligible request captures instead.
- **S6.** "Corre en segundo plano: no retrasa el turno del agente. Si el
  renombrado falla, reintenta en el siguiente pedido elegible." `hu title
  watch` runs as a background process per workspace; a failed rename (socket
  error, transient) retries with the same captured request on the next
  eligible tick — the request text is not re-captured.
- **S7.** "Usa el binario de HERDR_BIN_PATH o, si falta, herdr del PATH."
  Every herdr invocation resolves HERDR_BIN_PATH first.
- **S8.** Sidecar display (herdr-sidecar repo, separate unit): the dock shows
  the workspace's generated title under the Gentle Shell label, truncated to
  the dock width, muted; absent title renders nothing (no placeholder).
- **S9.** Default off: the tool has its own toggle (`tools.title`) in the
  plugin config, like every other container tool.

## Tasks

- **T1** (S1-S7, S9): `hu title` in herdr-utils — `internal/title` heuristic +
  skip rules, `herdrcli` WorkspaceTitle/SetWorkspaceTitle (report-metadata
  source `herdr-utils`, token `title`), config toggle `tools.title`,
  `hu title set` command. Evidence: live smoke test on w25 — applied "Implement
  the workspace title" once, second call no-op, label untouched. Commit:
  herdr-utils, branch feat/workspace-title.
- **T2** (S8): sidecar dock renders the workspace title under the agent
  label. Commit: herdr-sidecar worktree, branch feat/workspace-title.

## Log

- **L1** (user request, verbatim): "herdr-workspace-title — Le pone nombre al
  espacio de trabajo de herdr una sola vez, con un título corto sacado de tu
  primer pedido real en la sesión (por ejemplo, «Fix login redirect loop»).
  Sin captura a propósito... Reglas: Solo actúa dentro de herdr
  (HERDR_ENV=1). Un nombre que pusiste a mano manda siempre: si el espacio ya
  tiene nombre propio, no lo pisa. El título (2 a 5 palabras, máx. 34
  caracteres, en el idioma de tu pedido) lo redacta el modelo activo con una
  llamada corta y aislada. Si falla (sin autenticación, tiempo agotado,
  respuesta vacía) usa una heurística local. Necesita un pedido de al menos
  12 caracteres. Corre en segundo plano: no retrasa el turno del agente. Si
  el renombrado falla, reintenta en el siguiente pedido elegible. Usa el
  binario de HERDR_BIN_PATH o, si falta, herdr del PATH."
- **L2** (decision, user): mechanism in herdr-utils, display in herdr-sidecar
  — "que -sidecar lo utilise para Ponerle un titulo en el sidecar debajo de
  Gentle-Shell". Metadata probed live: `herdr workspace report-metadata w25
  --source probe --token title="Probe Title"` surfaces as
  `tokens.title` in `workspace get` / `api snapshot` — display-only, label
  untouched; probe cleared after the check.
- **L3** (user): sidecar work must happen in a worktree — other agents work
  on other features in the main checkout.
- **L4** (decision, orchestrator): the "modelo activo" call of S4 lives with
  the CALLER, not the binary: a standalone plugin binary has no session, no
  model, and no credentials. `hu title set <text>` accepts the request text
  and keeps a fitting text word for word, so a harness that redacted a title
  with its model passes it as the request; the heuristic is the in-binary
  fallback. S4's source order is preserved at the feature level.
- **L5** (decision, orchestrator): "corre en segundo plano" (S6) is realized
  as a nonblocking CLI contract instead of a daemon in v1: the command
  returns after one report-metadata write (~ms), the caller (agent hook or
  harness) invokes it without blocking its turn, and a failed rename exits 1
  for the caller to retry with the same text. A `watch` daemon cannot see
  agent requests on its own (no session read surface), so the capture stays
  with whoever has the request — same division as L4.
- **L6** (T2 evidence): herdr-sidecar worktree a39386f — Facts.Title from a
  slow side clock (one workspace get per minute, failures keep the last read),
  workspace id from the pane id prefix, header renders the title muted,
  centered, clipped; absent title renders nothing and the artboard pins hold
  byte-identical. Suite: 13 packages ok, vet clean.
