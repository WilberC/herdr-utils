package herdrmock_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

func fixture() *herdrmock.Fake {
	return &herdrmock.Fake{
		Actions: []herdrmock.Action{
			{Plugin: "herdr-suite", ID: "save", Title: "Save space", Description: "Snapshot the current workspace", Contexts: []string{"workspace", "pane"}},
			{Plugin: "herdr-bar", ID: "open", Title: "Open bar", Description: "Fuzzy-jump to any tab", Contexts: []string{"global"}},
		},
		Current: herdrmock.Session{
			FocusedWorkspaceID: "w-1",
			FocusedTabID:       "w-1:t1",
			FocusedPaneID:      "w-1:p1",
			Workspaces: []herdrmock.Workspace{
				{ID: "w-1", Label: "dotfiles", Focused: true, ActiveTabID: "w-1:t1"},
			},
			Tabs: []herdrmock.Tab{
				{ID: "w-1:t1", WorkspaceID: "w-1", Label: "main", Focused: true},
			},
			Agents: []herdrmock.Agent{
				{Kind: "pi", Status: "idle", PaneID: "w-1:p1", TabID: "w-1:t1", WorkspaceID: "w-1", Title: "dotfiles", Focused: true},
			},
		},
	}
}

func TestClientContract(t *testing.T) {
	tests := []struct {
		name      string
		fake      func() *herdrmock.Fake
		call      func(context.Context, herdrmock.Client) (any, error)
		want      any
		wantErr   string
		wantCalls []herdrmock.Call
	}{
		{
			name: "plugin actions returns every scripted action",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return client.PluginActions(ctx, "")
			},
			want: fixture().Actions,
			wantCalls: []herdrmock.Call{
				{Method: "PluginActions"},
			},
		},
		{
			name: "plugin actions filters by plugin and records the target",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return client.PluginActions(ctx, "herdr-bar")
			},
			want: fixture().Actions[1:],
			wantCalls: []herdrmock.Call{
				{Method: "PluginActions", Target: "herdr-bar"},
			},
		},
		{
			name: "plugin actions surfaces the scripted error",
			fake: func() *herdrmock.Fake {
				fake := fixture()
				fake.PluginActionsErr = errors.New("socket unavailable")
				return fake
			},
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return client.PluginActions(ctx, "")
			},
			wantErr:   "socket unavailable",
			wantCalls: []herdrmock.Call{{Method: "PluginActions"}},
		},
		{
			name: "invoke action records the fully qualified id",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.InvokeAction(ctx, "herdr-bar.open")
			},
			wantCalls: []herdrmock.Call{
				{Method: "InvokeAction", Target: "herdr-bar.open"},
			},
		},
		{
			name: "invoke action surfaces the scripted error",
			fake: func() *herdrmock.Fake {
				fake := fixture()
				fake.InvokeErr = errors.New("action is not available here")
				return fake
			},
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.InvokeAction(ctx, "herdr-bar.open")
			},
			wantErr:   "action is not available here",
			wantCalls: []herdrmock.Call{{Method: "InvokeAction", Target: "herdr-bar.open"}},
		},
		{
			name: "session returns the scripted snapshot",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return client.Session(ctx)
			},
			want:      fixture().Current,
			wantCalls: []herdrmock.Call{{Method: "Session"}},
		},
		{
			name: "session surfaces the scripted error",
			fake: func() *herdrmock.Fake {
				fake := fixture()
				fake.SessionErr = errors.New("no snapshot")
				return fake
			},
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return client.Session(ctx)
			},
			wantErr:   "no snapshot",
			wantCalls: []herdrmock.Call{{Method: "Session"}},
		},
		{
			name: "focus workspace records the id",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.FocusWorkspace(ctx, "w-1")
			},
			wantCalls: []herdrmock.Call{{Method: "FocusWorkspace", Target: "w-1"}},
		},
		{
			name: "focus tab records the id",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.FocusTab(ctx, "w-1:t1")
			},
			wantCalls: []herdrmock.Call{{Method: "FocusTab", Target: "w-1:t1"}},
		},
		{
			name: "focus agent records the target",
			fake: fixture,
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.FocusAgent(ctx, "w-1:p1")
			},
			wantCalls: []herdrmock.Call{{Method: "FocusAgent", Target: "w-1:p1"}},
		},
		{
			name: "focus surfaces the scripted error",
			fake: func() *herdrmock.Fake {
				fake := fixture()
				fake.FocusTabErr = errors.New("tab is gone")
				return fake
			},
			call: func(ctx context.Context, client herdrmock.Client) (any, error) {
				return nil, client.FocusTab(ctx, "w-1:t1")
			},
			wantErr:   "tab is gone",
			wantCalls: []herdrmock.Call{{Method: "FocusTab", Target: "w-1:t1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client herdrmock.Client = tt.fake()

			got, gotErr := tt.call(context.Background(), client)

			if tt.wantErr != "" {
				if gotErr == nil || gotErr.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", gotErr, tt.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("unexpected error: %v", gotErr)
			}
			if tt.wantErr == "" && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %#v, want %#v", got, tt.want)
			}
			fake, ok := client.(*herdrmock.Fake)
			if !ok {
				t.Fatalf("client is %T, want *herdrmock.Fake", client)
			}
			if !reflect.DeepEqual(fake.Calls(), tt.wantCalls) {
				t.Fatalf("calls = %#v, want %#v", fake.Calls(), tt.wantCalls)
			}
		})
	}
}

var _ herdrmock.Client = (*herdrmock.Fake)(nil)
