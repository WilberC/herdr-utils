// Package herdrcli implements the herdr client by running the herdr binary.
// herdr-utils never opens a herdr socket itself: every read and write goes
// through the CLI, so the binary behaves the same whether it runs outside herdr
// or inside a plugin action.
package herdrcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

// EnvBinary names the environment variable that overrides where the herdr binary
// is found. Without it the binary is looked up on PATH.
const EnvBinary = "HERDR_BIN_PATH"

// Runner executes the herdr binary and returns its standard streams.
type Runner func(ctx context.Context, bin string, args ...string) (stdout, stderr []byte, err error)

// Client is the herdrmock.Client implementation that shells out to the herdr
// binary.
type Client struct {
	Bin string
	Run Runner
}

// New returns a client that resolves the herdr binary on every call, so a
// HERDR_BIN_PATH exported after start still applies.
func New() *Client {
	return &Client{}
}

// PluginActions lists the declared plugin actions, reading what
// "herdr plugin list --json" reports for every installed plugin or for the one
// named.
func (c *Client) PluginActions(ctx context.Context, plugin string) ([]herdrmock.Action, error) {
	args := []string{"plugin", "list", "--json"}
	if plugin != "" {
		args = append(args, "--plugin", plugin)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var document struct {
		Result struct {
			Plugins []struct {
				ID      string `json:"plugin_id"`
				Actions []struct {
					ID          string   `json:"id"`
					Title       string   `json:"title"`
					Description string   `json:"description"`
					Contexts    []string `json:"contexts"`
					Command     []string `json:"command"`
				} `json:"actions"`
			} `json:"plugins"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		return nil, fmt.Errorf("parse herdr plugin list --json: %w", err)
	}

	actions := make([]herdrmock.Action, 0, len(document.Result.Plugins))
	for _, plugin := range document.Result.Plugins {
		for _, action := range plugin.Actions {
			actions = append(actions, herdrmock.Action{
				Plugin:      plugin.ID,
				ID:          action.ID,
				Title:       action.Title,
				Description: action.Description,
				Contexts:    action.Contexts,
			})
		}
	}
	return actions, nil
}

// Plugins lists the installed plugins, reading the ids and refs
// "herdr plugin list --json" reports.
func (c *Client) Plugins(ctx context.Context) ([]herdrmock.Plugin, error) {
	out, err := c.run(ctx, "plugin", "list", "--json")
	if err != nil {
		return nil, err
	}

	var document struct {
		Result struct {
			Plugins []struct {
				ID      string `json:"plugin_id"`
				Ref     string `json:"ref"`
				Version string `json:"version"`
			} `json:"plugins"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		return nil, fmt.Errorf("parse herdr plugin list --json: %w", err)
	}

	installed := make([]herdrmock.Plugin, 0, len(document.Result.Plugins))
	for _, plugin := range document.Result.Plugins {
		ref := plugin.Ref
		if ref == "" {
			ref = plugin.Version
		}
		installed = append(installed, herdrmock.Plugin{ID: plugin.ID, Ref: ref})
	}
	return installed, nil
}

// InstallPlugin installs one plugin, pinned to ref when the catalog declares
// one, and answers the -y herdr asks for on a non-interactive install.
func (c *Client) InstallPlugin(ctx context.Context, repo, ref string) error {
	args := []string{"plugin", "install", repo, "-y"}
	if ref != "" {
		args = append(args, "--ref", ref)
	}
	_, err := c.run(ctx, args...)
	return err
}

// Session reads the live session through "herdr api snapshot".
func (c *Client) Session(ctx context.Context) (herdrmock.Session, error) {
	out, err := c.run(ctx, "api", "snapshot")
	if err != nil {
		return herdrmock.Session{}, err
	}

	var document struct {
		Result struct {
			Snapshot struct {
				FocusedPaneID      string `json:"focused_pane_id"`
				FocusedTabID       string `json:"focused_tab_id"`
				FocusedWorkspaceID string `json:"focused_workspace_id"`
				Workspaces         []struct {
					ID          string `json:"workspace_id"`
					Label       string `json:"label"`
					Focused     bool   `json:"focused"`
					ActiveTabID string `json:"active_tab_id"`
				} `json:"workspaces"`
				Tabs []struct {
					ID          string `json:"tab_id"`
					WorkspaceID string `json:"workspace_id"`
					Label       string `json:"label"`
					Focused     bool   `json:"focused"`
				} `json:"tabs"`
				Agents []struct {
					Kind        string `json:"agent"`
					Status      string `json:"agent_status"`
					Title       string `json:"terminal_title"`
					PaneID      string `json:"pane_id"`
					TabID       string `json:"tab_id"`
					WorkspaceID string `json:"workspace_id"`
					Focused     bool   `json:"focused"`
				} `json:"agents"`
			} `json:"snapshot"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		return herdrmock.Session{}, fmt.Errorf("parse herdr api snapshot: %w", err)
	}

	snapshot := document.Result.Snapshot
	session := herdrmock.Session{
		FocusedWorkspaceID: snapshot.FocusedWorkspaceID,
		FocusedTabID:       snapshot.FocusedTabID,
		FocusedPaneID:      snapshot.FocusedPaneID,
		Workspaces:         make([]herdrmock.Workspace, 0, len(snapshot.Workspaces)),
		Tabs:               make([]herdrmock.Tab, 0, len(snapshot.Tabs)),
		Agents:             make([]herdrmock.Agent, 0, len(snapshot.Agents)),
	}
	for _, workspace := range snapshot.Workspaces {
		session.Workspaces = append(session.Workspaces, herdrmock.Workspace{
			ID:          workspace.ID,
			Label:       workspace.Label,
			Focused:     workspace.Focused,
			ActiveTabID: workspace.ActiveTabID,
		})
	}
	for _, tab := range snapshot.Tabs {
		session.Tabs = append(session.Tabs, herdrmock.Tab{
			ID:          tab.ID,
			WorkspaceID: tab.WorkspaceID,
			Label:       tab.Label,
			Focused:     tab.Focused,
		})
	}
	for _, agent := range snapshot.Agents {
		session.Agents = append(session.Agents, herdrmock.Agent{
			Kind:        agent.Kind,
			Status:      agent.Status,
			Title:       agent.Title,
			PaneID:      agent.PaneID,
			TabID:       agent.TabID,
			WorkspaceID: agent.WorkspaceID,
			Focused:     agent.Focused,
		})
	}
	return session, nil
}

// FocusWorkspace focuses one workspace.
func (c *Client) FocusWorkspace(ctx context.Context, workspaceID string) error {
	_, err := c.run(ctx, "workspace", "focus", workspaceID)
	return err
}

// FocusTab focuses one tab.
func (c *Client) FocusTab(ctx context.Context, tabID string) error {
	_, err := c.run(ctx, "tab", "focus", tabID)
	return err
}

// FocusAgent focuses one agent by name or by the pane hosting it.
func (c *Client) FocusAgent(ctx context.Context, target string) error {
	_, err := c.run(ctx, "agent", "focus", target)
	return err
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	bin, err := binary(c.Bin)
	if err != nil {
		return nil, err
	}
	run := c.Run
	if run == nil {
		run = execRunner
	}
	stdout, stderr, err := run(ctx, bin, args...)
	if err != nil {
		if message := strings.TrimSpace(string(stderr)); message != "" {
			return nil, fmt.Errorf("herdr %s: %w: %s", strings.Join(args, " "), err, message)
		}
		return nil, fmt.Errorf("herdr %s: %w", strings.Join(args, " "), err)
	}
	return stdout, nil
}

func execRunner(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func binary(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if path := os.Getenv(EnvBinary); path != "" {
		return path, nil
	}
	path, err := exec.LookPath("herdr")
	if err != nil {
		return "", fmt.Errorf("herdr is required; install it or set %s", EnvBinary)
	}
	return path, nil
}

var _ herdrmock.Client = (*Client)(nil)
