package herdrmock

import (
	"context"
	"sync"
)

// Fake is an in-memory Client for tests. It never opens a socket: every method
// returns scripted state and records the call it received.
type Fake struct {
	Actions []Action
	Current Session

	// Installed is the scripted plugin inventory. A successful install appends
	// to it, so a test sees the second sync report everything current.
	Installed []Plugin

	PluginActionsErr  error
	SessionErr        error
	FocusWorkspaceErr error
	FocusTabErr       error
	FocusAgentErr     error
	InstalledErr      error
	InstallErrs       map[string]error

	mu    sync.Mutex
	calls []Call
}

// Calls returns the calls recorded so far, oldest first.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

func (f *Fake) record(method, target string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, Call{Method: method, Target: target})
}

// PluginActions returns the scripted actions, narrowed to plugin when it is not
// empty.
func (f *Fake) PluginActions(ctx context.Context, plugin string) ([]Action, error) {
	f.record("PluginActions", plugin)
	if f.PluginActionsErr != nil {
		return nil, f.PluginActionsErr
	}
	actions := make([]Action, 0, len(f.Actions))
	for _, action := range f.Actions {
		if plugin == "" || action.Plugin == plugin {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

// Session returns the scripted snapshot.
func (f *Fake) Session(ctx context.Context) (Session, error) {
	f.record("Session", "")
	if f.SessionErr != nil {
		return Session{}, f.SessionErr
	}
	return f.Current, nil
}

// FocusWorkspace records the focus change.
func (f *Fake) FocusWorkspace(ctx context.Context, workspaceID string) error {
	f.record("FocusWorkspace", workspaceID)
	return f.FocusWorkspaceErr
}

// FocusTab records the focus change.
func (f *Fake) FocusTab(ctx context.Context, tabID string) error {
	f.record("FocusTab", tabID)
	return f.FocusTabErr
}

// FocusAgent records the focus change.
func (f *Fake) FocusAgent(ctx context.Context, target string) error {
	f.record("FocusAgent", target)
	return f.FocusAgentErr
}

// Plugins returns the scripted inventory.
func (f *Fake) Plugins(ctx context.Context) ([]Plugin, error) {
	f.record("Plugins", "")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.InstalledErr != nil {
		return nil, f.InstalledErr
	}
	return append([]Plugin(nil), f.Installed...), nil
}

// InstallPlugin records the install and, unless the repo is scripted to fail,
// adds it to the inventory.
func (f *Fake) InstallPlugin(ctx context.Context, repo, ref string) error {
	f.record("InstallPlugin", repo)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.InstallErrs[repo]; err != nil {
		return err
	}
	f.Installed = append(f.Installed, Plugin{ID: repo, Ref: ref})
	return nil
}

var _ Client = (*Fake)(nil)
