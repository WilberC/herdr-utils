package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

var errBoom = errors.New("boom")

// seedSession scripts a Fake session with one workspace and one agent, the way
// herdr api snapshot reports them: the workspace label matches the agent cwd's
// basename until someone renames it.
func seedSession(fake *herdrmock.Fake, workspace, label, cwd string) {
	fake.Current = herdrmock.Session{
		FocusedWorkspaceID: workspace,
		Workspaces: []herdrmock.Workspace{
			{ID: workspace, Label: label, Focused: true, ActiveTabID: workspace + ":t1"},
		},
		Agents: []herdrmock.Agent{
			{Kind: "pi", Status: "working", WorkspaceID: workspace, PaneID: workspace + ":p1", Cwd: cwd},
		},
	}
}

func TestTitleSetAppliesTheHeuristicTitle(t *testing.T) {
	r := newTestRun(t)
	fake := r.deps.client.(*herdrmock.Fake)
	seedSession(fake, "w1", "herdr-sidecar", "/home/wilber/Projects/Personal/herdr-sidecar")
	r.writeConfig(t, `{"tools": {"title": true}}`)
	r.env["HERDR_ENV"] = "1"

	exit := r.run(t, "title", "set", "fix", "the", "login", "redirect", "loop", "in", "the", "auth", "service", "when", "the", "cookie", "expires")
	if exit != 0 {
		t.Fatalf("exit = %d, stderr: %s", exit, r.stderr.String())
	}
	if got := r.stdout.String(); !strings.Contains(got, "Fix the login redirect loop") {
		t.Fatalf("stdout = %q, want the applied title", got)
	}
	var applied string
	for _, call := range fake.Calls() {
		if call.Method == "SetWorkspaceTitle" {
			applied = call.Target
		}
	}
	if applied == "" {
		t.Fatalf("SetWorkspaceTitle was never called; calls: %v", fake.Calls())
	}
	if !strings.HasPrefix(applied, "w1 ") || !strings.HasSuffix(applied, " Fix the login redirect loop") {
		t.Fatalf("applied = %q, want it bound to workspace w1", applied)
	}
}

func TestTitleSetContract(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		herdrEnv   string
		args       []string
		setup      func(t *testing.T, r *testRun)
		wantExit   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "the tool is off: refuse like every other tool",
			config:     `{"tools": {"title": false}}`,
			args:       []string{"title", "set", "fix the login redirect loop"},
			wantExit:   2,
			wantStderr: []string{"tools.title", "config.json"},
		},
		{
			name:     "outside herdr the command silently does nothing",
			config:   `{"tools": {"title": true}}`,
			args:     []string{"title", "set", "fix the login redirect loop"},
			wantExit: 0,
		},
		{
			name:       "a request under the floor is not eligible",
			config:     `{"tools": {"title": true}}`,
			herdrEnv:   "1",
			args:       []string{"title", "set", "hola"},
			wantExit:   2,
			wantStderr: []string{"12", "request"},
		},
		{
			name:     "set without text prints its usage",
			config:   `{"tools": {"title": true}}`,
			herdrEnv: "1",
			args:     []string{"title", "set"},
			wantExit: 2,
			wantStderr: []string{"usage: hu title set"},
		},
		{
			name:   "an already-titled workspace keeps its title",
			config: `{"tools": {"title": true}}`,
			herdrEnv: "1",
			args:   []string{"title", "set", "fix the login redirect loop"},
			setup: func(t *testing.T, r *testRun) {
				seedSession(r.deps.client.(*herdrmock.Fake), "w1", "herdr-sidecar", "/home/wilber/Projects/Personal/herdr-sidecar")
				r.deps.client.(*herdrmock.Fake).Titles = map[string]string{"w1": "Fix login redirect"}
			},
			wantExit: 0,
		},
		{
			name:   "a manually renamed workspace keeps its name",
			config: `{"tools": {"title": true}}`,
			herdrEnv: "1",
			args:   []string{"title", "set", "fix the login redirect loop"},
			setup: func(t *testing.T, r *testRun) {
				seedSession(r.deps.client.(*herdrmock.Fake), "w1", "my workspace", "/home/wilber/Projects/Personal/herdr-sidecar")
			},
			wantExit: 0,
		},
		{
			name:   "a herdr failure exits 1 for the next retry",
			config: `{"tools": {"title": true}}`,
			herdrEnv: "1",
			args:   []string{"title", "set", "fix the login redirect loop"},
			setup: func(t *testing.T, r *testRun) {
				seedSession(r.deps.client.(*herdrmock.Fake), "w1", "herdr-sidecar", "/home/wilber/Projects/Personal/herdr-sidecar")
				r.deps.client.(*herdrmock.Fake).SetTitleErr = errBoom
			},
			wantExit:   1,
			wantStderr: []string{"boom"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newTestRun(t)
			r.writeConfig(t, test.config)
			r.env["HERDR_ENV"] = test.herdrEnv
			if test.setup != nil {
				test.setup(t, r)
			}
			exit := r.run(t, test.args...)
			if exit != test.wantExit {
				t.Fatalf("exit = %d, want %d (stderr: %q)", exit, test.wantExit, r.stderr.String())
			}
			for _, want := range test.wantStdout {
				if !strings.Contains(r.stdout.String(), want) {
					t.Fatalf("stdout %q misses %q", r.stdout.String(), want)
				}
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(r.stderr.String(), want) {
					t.Fatalf("stderr %q misses %q", r.stderr.String(), want)
				}
			}
		})
	}
}

func TestTitleHelpPrintsItsUsage(t *testing.T) {
	r := newTestRun(t)
	if exit := r.run(t, "title", "--help"); exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	if out := r.stdout.String(); !strings.Contains(out, "usage: hu title") {
		t.Fatalf("stdout = %q", out)
	}
}
