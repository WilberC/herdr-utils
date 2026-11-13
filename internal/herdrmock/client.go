// Package herdrmock is the herdr surface this plugin depends on, expressed as an
// interface, plus the offline fake every test uses. Production code implements it
// in internal/herdrcli; nothing here opens a socket.
package herdrmock

import "context"

// Action is one plugin action as herdr reports it: the plugin that declares it,
// and its id, title, description and contexts.
type Action struct {
	Plugin      string
	ID          string
	Title       string
	Description string
	Contexts    []string
}

// Workspace is one herdr workspace as reported by the session snapshot.
type Workspace struct {
	ID          string
	Label       string
	Focused     bool
	ActiveTabID string
}

// Tab is one herdr tab as reported by the session snapshot.
type Tab struct {
	ID          string
	WorkspaceID string
	Label       string
	Focused     bool
}

// Agent is one agent running in a pane. A pane without a recognized agent has no
// entry, so Kind is never empty here.
type Agent struct {
	Kind        string
	Status      string
	Title       string
	PaneID      string
	TabID       string
	WorkspaceID string
	Focused     bool
}

// Session is the live topology the session read returns: where the focus is, and
// the workspaces, tabs and agents behind it.
type Session struct {
	FocusedWorkspaceID string
	FocusedTabID       string
	FocusedPaneID      string
	Workspaces         []Workspace
	Tabs               []Tab
	Agents             []Agent
}

// Call records one invoked Client method and the identifier it targeted.
type Call struct {
	Method string
	Target string
}

// ActionClient is the part of Client that lists plugin actions. An empty plugin
// asks for every installed plugin; a plugin name narrows the read to it.
type ActionClient interface {
	PluginActions(ctx context.Context, plugin string) ([]Action, error)
}

// SessionClient is the part of Client that reads the live session.
type SessionClient interface {
	Session(ctx context.Context) (Session, error)
}

// FocusClient is the part of Client that moves the focus.
type FocusClient interface {
	FocusWorkspace(ctx context.Context, workspaceID string) error
	FocusTab(ctx context.Context, tabID string) error
	FocusAgent(ctx context.Context, target string) error
}

// Client is the minimal herdr surface herdr-utils depends on.
type Client interface {
	ActionClient
	SessionClient
	FocusClient
}
