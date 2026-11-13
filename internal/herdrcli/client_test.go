package herdrcli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

func writeStub(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

const pluginListStub = `#!/bin/sh
if [ "$1 $2" != "plugin list" ] || [ "$3" != "--json" ]; then
  echo "unexpected call: $*" >&2
  exit 9
fi
printf '%s' '{"id":"cli:plugin","result":{"plugins":[{"plugin_id":"herdr-suite","actions":[{"id":"save","title":"Save space","description":"Snapshot the current workspace","contexts":["workspace","pane"],"command":["bin/hs","space","save","--current"]},{"id":"load","title":"Load space","contexts":["global"],"command":["bin/hs","space","load"]}]},{"plugin_id":"herdr-bar","actions":[{"id":"open","title":"Open bar","description":"Fuzzy-jump to any tab","contexts":["global"]}]}]}}'
`

func TestPluginActionsRunsTheHerdrBinaryAndParsesIt(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, pluginListStub))

	actions, err := New().PluginActions(context.Background(), "")
	if err != nil {
		t.Fatalf("PluginActions error = %v, want nil", err)
	}

	want := []herdrmock.Action{
		{Plugin: "herdr-suite", ID: "save", Title: "Save space", Description: "Snapshot the current workspace", Contexts: []string{"workspace", "pane"}},
		{Plugin: "herdr-suite", ID: "load", Title: "Load space", Contexts: []string{"global"}},
		{Plugin: "herdr-bar", ID: "open", Title: "Open bar", Description: "Fuzzy-jump to any tab", Contexts: []string{"global"}},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("PluginActions = %#v, want %#v", actions, want)
	}
}

func TestPluginActionsFiltersThroughTheHerdrFlag(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, `#!/bin/sh
if [ "$*" != "plugin list --json --plugin herdr-bar" ]; then
  echo "unexpected call: $*" >&2
  exit 9
fi
printf '%s' '{"result":{"plugins":[{"plugin_id":"herdr-bar","actions":[{"id":"open","title":"Open bar"}]}]}}'
`))

	actions, err := New().PluginActions(context.Background(), "herdr-bar")
	if err != nil {
		t.Fatalf("PluginActions error = %v, want nil", err)
	}
	if len(actions) != 1 || actions[0].Plugin != "herdr-bar" || actions[0].ID != "open" {
		t.Fatalf("PluginActions = %#v, want the herdr-bar action", actions)
	}
}

func TestPluginActionsRejectsUnparsableOutput(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\nprintf 'not json'\n"))

	_, err := New().PluginActions(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "parse herdr plugin list") {
		t.Fatalf("PluginActions error = %v, want a parse error", err)
	}
}

func TestPluginActionsReportsTheHerdrError(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\necho 'no server running' >&2\nexit 3\n"))

	_, err := New().PluginActions(context.Background(), "")

	if err == nil {
		t.Fatal("PluginActions error = nil, want a failure")
	}
	for _, want := range []string{"herdr plugin list --json", "exit status 3", "no server running"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("PluginActions error = %q, want it to contain %q", err, want)
		}
	}
}

const snapshotStub = `#!/bin/sh
if [ "$1 $2" != "api snapshot" ]; then
  echo "unexpected call: $*" >&2
  exit 9
fi
printf '%s' '{"id":"cli:api:snapshot","result":{"snapshot":{"focused_pane_id":"w-1:p1","focused_tab_id":"w-1:t1","focused_workspace_id":"w-1","workspaces":[{"workspace_id":"w-1","label":"dotfiles","focused":true,"active_tab_id":"w-1:t1"}],"tabs":[{"tab_id":"w-1:t1","workspace_id":"w-1","label":"main","focused":true}],"agents":[{"agent":"pi","agent_status":"idle","pane_id":"w-1:p1","tab_id":"w-1:t1","workspace_id":"w-1","terminal_title":"dotfiles","focused":true}]}}}'
`

func TestSessionReadsTheSnapshot(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, snapshotStub))

	session, err := New().Session(context.Background())
	if err != nil {
		t.Fatalf("Session error = %v, want nil", err)
	}

	want := herdrmock.Session{
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
	}
	if !reflect.DeepEqual(session, want) {
		t.Fatalf("Session = %#v, want %#v", session, want)
	}
}

func TestSessionRejectsUnparsableOutput(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\nprintf 'not json'\n"))

	_, err := New().Session(context.Background())

	if err == nil || !strings.Contains(err.Error(), "parse herdr api snapshot") {
		t.Fatalf("Session error = %v, want a parse error", err)
	}
}

func TestFocusCallsReachTheRightHerdrCommand(t *testing.T) {
	tests := []struct {
		name string
		want string
		call func(context.Context, *Client) error
	}{
		{name: "workspace", want: "workspace focus w-1", call: func(ctx context.Context, client *Client) error {
			return client.FocusWorkspace(ctx, "w-1")
		}},
		{name: "tab", want: "tab focus w-1:t1", call: func(ctx context.Context, client *Client) error {
			return client.FocusTab(ctx, "w-1:t1")
		}},
		{name: "agent", want: "agent focus w-1:p1", call: func(ctx context.Context, client *Client) error {
			return client.FocusAgent(ctx, "w-1:p1")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\nif [ \"$*\" != \""+tt.want+"\" ]; then\n  echo \"unexpected call: $*\" >&2\n  exit 9\nfi\n"))

			if err := tt.call(context.Background(), New()); err != nil {
				t.Fatalf("focus error = %v, want nil", err)
			}
		})
	}
}

func TestFocusReportsTheHerdrError(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\necho 'workspace is gone' >&2\nexit 4\n"))

	err := New().FocusWorkspace(context.Background(), "w-9")

	if err == nil {
		t.Fatal("FocusWorkspace error = nil, want a failure")
	}
	for _, want := range []string{"herdr workspace focus w-9", "exit status 4", "workspace is gone"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("FocusWorkspace error = %q, want it to contain %q", err, want)
		}
	}
}

func TestClientRefusesWhenHerdrIsMissing(t *testing.T) {
	t.Setenv(EnvBinary, "")
	t.Setenv("PATH", t.TempDir())

	_, err := New().Session(context.Background())

	if err == nil {
		t.Fatal("Session error = nil, want the missing herdr refused")
	}
	for _, want := range []string{"herdr is required", EnvBinary} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Session error = %q, want it to contain %q", err, want)
		}
	}
}

// TestClientHonoursAnExplicitBinary pins the Bin field the tests and the
// plugin hook both rely on: it wins over the environment.
func TestClientHonoursAnExplicitBinary(t *testing.T) {
	bin := writeStub(t, snapshotStub)

	if _, err := (&Client{Bin: bin}).Session(context.Background()); err != nil {
		t.Fatalf("Session error = %v, want nil", err)
	}
}

func TestPluginsReadsTheInstalledInventory(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, `#!/bin/sh
if [ "$*" != "plugin list --json" ]; then
  echo "unexpected call: $*" >&2
  exit 9
fi
printf '%s' '{"result":{"plugins":[{"plugin_id":"herdr-suite","ref":"v1.2.0"},{"plugin_id":"herdr-bar","version":"v0.4.1"},{"plugin_id":"herdr-utils"}]}}'
`))

	installed, err := New().Plugins(context.Background())
	if err != nil {
		t.Fatalf("Plugins error = %v, want nil", err)
	}

	want := []herdrmock.Plugin{
		{ID: "herdr-suite", Ref: "v1.2.0"},
		{ID: "herdr-bar", Ref: "v0.4.1"},
		{ID: "herdr-utils"},
	}
	if !reflect.DeepEqual(installed, want) {
		t.Fatalf("Plugins = %#v, want %#v", installed, want)
	}
}

func TestPluginsRejectsUnparsableOutput(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\nprintf 'not json'\n"))

	_, err := New().Plugins(context.Background())

	if err == nil || !strings.Contains(err.Error(), "parse herdr plugin list") {
		t.Fatalf("Plugins error = %v, want a parse error", err)
	}
}

func TestInstallPluginRunsTheHerdrInstallCommand(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{name: "unpinned", want: "plugin install owner/repo -y"},
		{name: "pinned", ref: "v1", want: "plugin install owner/repo -y --ref v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\nif [ \"$*\" != \""+tt.want+"\" ]; then\n  echo \"unexpected call: $*\" >&2\n  exit 9\nfi\n"))

			if err := New().InstallPlugin(context.Background(), "owner/repo", tt.ref); err != nil {
				t.Fatalf("InstallPlugin error = %v, want nil", err)
			}
		})
	}
}

func TestInstallPluginReportsTheHerdrError(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\necho 'repository not found' >&2\nexit 4\n"))

	err := New().InstallPlugin(context.Background(), "owner/gone", "")

	if err == nil {
		t.Fatal("InstallPlugin error = nil, want a failure")
	}
	for _, want := range []string{"herdr plugin install owner/gone -y", "repository not found"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("InstallPlugin error = %q, want it to contain %q", err, want)
		}
	}
}

var _ herdrmock.Client = (*Client)(nil)

func TestInvokeActionRunsTheFullyQualifiedActionId(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, `#!/bin/sh
if [ "$*" != "plugin action invoke herdr-bar.open" ]; then
  echo "unexpected call: $*" >&2
  exit 9
fi
`))

	if err := New().InvokeAction(context.Background(), "herdr-bar.open"); err != nil {
		t.Fatalf("InvokeAction error = %v, want nil", err)
	}
}

func TestInvokeActionReportsTheHerdrError(t *testing.T) {
	t.Setenv(EnvBinary, writeStub(t, "#!/bin/sh\necho 'action is not available here' >&2\nexit 5\n"))

	err := New().InvokeAction(context.Background(), "herdr-suite.save")

	if err == nil {
		t.Fatal("InvokeAction error = nil, want a failure")
	}
	for _, want := range []string{"herdr plugin action invoke herdr-suite.save", "exit status 5", "action is not available here"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("InvokeAction error = %q, want it to contain %q", err, want)
		}
	}
}
