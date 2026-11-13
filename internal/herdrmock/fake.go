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

	PluginActionsErr  error
	SessionErr        error
	FocusWorkspaceErr error
	FocusTabErr       error
	FocusAgentErr     error

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

var _ Client = (*Fake)(nil)
