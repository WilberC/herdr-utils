package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/title"
)

const titleUsage = `usage: hu title set <request text>... [--workspace <id>]

Give the focused workspace (or --workspace <id>) a short display-only title
built from the agent's first real request: one title per workspace, 2 to 5
words capped at 34 characters, in the language of the request. The title is
reported through "herdr workspace report-metadata", so it shows up as
tokens.title under the workspace while the label you can rename stays
untouched; the sidecar dock reads it and draws it under the agent label.

Rules the command enforces:
  - the workspace must not already carry a title: once per workspace
  - a label you renamed yourself always wins: the command skips it
  - the request needs at least 12 characters
  - outside herdr (HERDR_ENV unset or not 1) the command does nothing
  - a failed rename exits 1: run the same command again with the same text

The tool is default off: set tools.title to true in the plugin config, at
$HERDR_PLUGIN_CONFIG_DIR/config.json or the herdr-utils entry under
$XDG_CONFIG_HOME/herdr/plugins/config.

A caller that already redacted the title with the session's model — the
short isolated call the workspace-title feature describes — passes that text
as the request; a fitting text is kept word for word, so this binary itself
never calls a model.
`

// runTitleSet gives one workspace its one-time display-only title.
func runTitleSet(d deps, args []string) int {
	document, err := config.Load(d.getenv)
	if err != nil {
		fmt.Fprintf(d.err, "herdr-utils: %v; the title stays unset\n", err)
		return 1
	}
	if !document.Config.ToolEnabled(config.ToolTitle) {
		path, _ := config.Path(d.getenv)
		fmt.Fprintf(d.err, "hu title is off; set tools.title = true in %s\n", path)
		return 2
	}
	// S2: the command only ever acts inside herdr.
	if d.getenv("HERDR_ENV") != "1" {
		return 0
	}

	request, workspaceID, err := titleArgs(args)
	if err != nil {
		fmt.Fprintf(d.err, "hu title set: %v\n\n%s", err, titleUsage)
		return 2
	}
	if !title.Eligible(request) {
		fmt.Fprintf(d.err, "hu title set: the request needs at least %d characters, got %q\n",
			title.MinRequestLen, strings.TrimSpace(request))
		return 2
	}

	ctx := context.Background()
	if workspaceID == "" {
		workspaceID, err = focusedWorkspace(ctx, d.client)
		if err != nil {
			fmt.Fprintf(d.err, "herdr-utils: %v\n", err)
			return 1
		}
	}

	existing, label, defaultLabel, err := workspaceNaming(ctx, d.client, workspaceID)
	if err != nil {
		// A read failure before any write is retryable: nothing was applied.
		fmt.Fprintf(d.err, "herdr-utils: %v; run the same command again\n", err)
		return 1
	}
	if title.Skip(existing, label, defaultLabel) {
		return 0
	}

	name := title.From(request)
	if err := d.client.SetWorkspaceTitle(ctx, workspaceID, name); err != nil {
		// S6: the rename failed, the request stays eligible; the caller
		// retries the same text and the once-only check still holds.
		fmt.Fprintf(d.err, "herdr-utils: %v\n", err)
		return 1
	}
	fmt.Fprintln(d.out, name)
	return 0
}

// titleArgs splits "<request text>... [--workspace <id>]": runTitle already
// dispatched the "set" subcommand, so everything left is request text and the
// workspace flag.
func titleArgs(args []string) (request, workspaceID string, err error) {
	words := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--workspace" {
			if i+1 >= len(args) {
				return "", "", errors.New("--workspace needs a workspace id")
			}
			workspaceID = args[i+1]
			i++
			continue
		}
		words = append(words, args[i])
	}
	if len(words) == 0 {
		return "", "", errors.New("a request text is required")
	}
	return strings.Join(words, " "), workspaceID, nil
}

// focusedWorkspace resolves the workspace the command acts on when the caller
// named none: the session's focused one.
func focusedWorkspace(ctx context.Context, client herdrmock.Client) (string, error) {
	session, err := client.Session(ctx)
	if err != nil {
		return "", err
	}
	if session.FocusedWorkspaceID == "" {
		return "", errors.New("no focused workspace to title")
	}
	return session.FocusedWorkspaceID, nil
}

// workspaceNaming gathers the three inputs the skip rules need: the
// workspace's current title token, its label, and the default label herdr
// derived from the workspace's agent cwd (the basename of the agent's
// directory). No agent cwd in the workspace leaves the default empty.
func workspaceNaming(ctx context.Context, client herdrmock.Client, workspaceID string) (existing, label, defaultLabel string, err error) {
	session, err := client.Session(ctx)
	if err != nil {
		return "", "", "", err
	}
	for _, workspace := range session.Workspaces {
		if workspace.ID != workspaceID {
			continue
		}
		label = workspace.Label
	}
	for _, agent := range session.Agents {
		if agent.WorkspaceID == workspaceID && agent.Cwd != "" {
			defaultLabel = filepath.Base(agent.Cwd)
			break
		}
	}
	existing, err = client.WorkspaceTitle(ctx, workspaceID)
	if err != nil {
		return "", "", "", err
	}
	return existing, label, defaultLabel, nil
}
