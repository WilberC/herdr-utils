package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/tools"
)

type testRun struct {
	deps           deps
	root           string
	env            map[string]string
	stdout, stderr bytes.Buffer
}

func newTestRun(t *testing.T) *testRun {
	t.Helper()
	root := t.TempDir()
	run := &testRun{root: root, env: map[string]string{
		"HERDR_PLUGIN_CONFIG_DIR": filepath.Join(root, "config"),
		"XDG_STATE_HOME":          filepath.Join(root, "state"),
		"HOME":                    filepath.Join(root, "home"),
	}}
	run.deps = deps{
		client:     &herdrmock.Fake{},
		getenv:     func(name string) string { return run.env[name] },
		now:        func() time.Time { return time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC) },
		cwd:        func() (string, error) { return root, nil },
		executable: func() (string, error) { return filepath.Join(root, "bin", "hu"), nil },
		version:    "dev",
		out:        &run.stdout,
		err:        &run.stderr,
	}
	return run
}

// writeConfig seeds the plugin config inside HERDR_PLUGIN_CONFIG_DIR, so a test
// exercises the same path production resolves.
func (r *testRun) writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := r.env["HERDR_PLUGIN_CONFIG_DIR"]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the plugin config directory: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write the plugin config: %v", err)
	}
	return path
}

func (r *testRun) run(t *testing.T, args ...string) int {
	t.Helper()
	return runWith(r.deps, args, &r.stdout, &r.stderr)
}

// TestRunContract pins the CLI contract every script and the plugin manifest
// depend on: usage and exit 2 with no command, 2 for an unknown command, 0 for
// help on stdout, and stderr-only refusals.
func TestRunContract(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		setup      func(t *testing.T, r *testRun)
		wantExit   int
		wantStdout []string
		wantStderr []string
	}{
		{
			name:       "no command prints usage and exits 2",
			wantExit:   2,
			wantStderr: []string{"usage: hu", "Commands:", "which", "plugins", "startup", "version"},
		},
		{
			name:       "an unknown command reports the name and exits 2",
			args:       []string{"bogus"},
			wantExit:   2,
			wantStderr: []string{`unknown command "bogus"`, "usage: hu"},
		},
		{
			name:       "the help flag prints usage on stdout and exits 0",
			args:       []string{"--help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu", "which", "plugins", "startup", "version"},
		},
		{
			name:       "the help command prints usage on stdout",
			args:       []string{"help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu"},
		},
		{
			name:       "bare which prints its usage on stderr",
			args:       []string{"which"},
			wantExit:   2,
			wantStderr: []string{"usage: hu which", "tools.which"},
		},
		{
			name:       "which help prints its usage on stdout",
			args:       []string{"which", "--help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu which", "which.sources"},
		},
		{
			name:       "which rejects an argument",
			args:       []string{"which", "extra"},
			wantExit:   2,
			wantStderr: []string{"which takes no arguments", "usage: hu which"},
		},
		{
			name:       "bare plugins prints its usage on stderr",
			args:       []string{"plugins"},
			wantExit:   2,
			wantStderr: []string{"usage: hu plugins", "tools.plugins"},
		},
		{
			name:       "plugins help prints its usage on stdout",
			args:       []string{"plugins", "--help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu plugins"},
		},
		{
			name:       "startup help prints its usage on stdout",
			args:       []string{"startup", "--help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu startup", "once per server socket"},
		},
		{
			name:       "startup rejects an argument",
			args:       []string{"startup", "extra"},
			wantExit:   2,
			wantStderr: []string{"startup takes no arguments", "usage: hu startup"},
		},
		{
			name:       "version help prints its usage on stdout",
			args:       []string{"version", "--help"},
			wantExit:   0,
			wantStdout: []string{"usage: hu version", "--check", "--json", "--manifest"},
		},
		{
			name: "startup with every tool off hints once and changes nothing",
			args: []string{"startup"},
			setup: func(t *testing.T, r *testRun) {
				r.writeConfig(t, `{"tools": {"which": false, "plugins": false}}`)
			},
			wantExit:   0,
			wantStderr: []string{"every tool is off", "tools.which", "config.json"},
		},
		{
			name: "startup with a tool on stays silent",
			args: []string{"startup"},
			setup: func(t *testing.T, r *testRun) {
				r.writeConfig(t, `{"tools": {"which": true}}`)
			},
			wantExit: 0,
		},
		{
			name: "startup refuses a malformed config without failing the server start",
			args: []string{"startup"},
			setup: func(t *testing.T, r *testRun) {
				r.writeConfig(t, `{"tools": {"which": "yes"}}`)
			},
			wantExit:   0,
			wantStderr: []string{"stays off", "config.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := newTestRun(t)
			if tt.setup != nil {
				tt.setup(t, run)
			}

			code := run.run(t, tt.args...)

			if code != tt.wantExit {
				t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.wantExit, run.stdout.String(), run.stderr.String())
			}
			for _, want := range tt.wantStdout {
				if !strings.Contains(run.stdout.String(), want) {
					t.Errorf("stdout does not contain %q; got:\n%s", want, run.stdout.String())
				}
			}
			for _, want := range tt.wantStderr {
				if !strings.Contains(run.stderr.String(), want) {
					t.Errorf("stderr does not contain %q; got:\n%s", want, run.stderr.String())
				}
			}
			if len(tt.wantStdout) == 0 && run.stdout.Len() != 0 {
				t.Errorf("stdout is not empty; got:\n%s", run.stdout.String())
			}
			if len(tt.wantStderr) == 0 && run.stderr.Len() != 0 {
				t.Errorf("stderr is not empty; got:\n%s", run.stderr.String())
			}
		})
	}
}

// TestStartupHintsOncePerSocket pins the gate's cost: one line per server
// socket while every tool is off, and nothing at all once a tool is on.
func TestStartupHintsOncePerSocket(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_SOCKET_PATH"] = "/tmp/herdr-one.sock"

	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("first startup exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), "every tool is off") {
		t.Fatalf("first startup stderr = %q, want the hint", run.stderr.String())
	}

	run.stdout.Reset()
	run.stderr.Reset()
	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("second startup exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if run.stderr.Len() != 0 {
		t.Fatalf("second startup stderr = %q, want silence for the same socket", run.stderr.String())
	}

	run.env["HERDR_SOCKET_PATH"] = "/tmp/herdr-two.sock"
	run.stderr.Reset()
	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("third startup exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), "every tool is off") {
		t.Fatalf("third startup stderr = %q, want the hint once for the new socket", run.stderr.String())
	}

	run.env["HERDR_SOCKET_PATH"] = "/tmp/herdr-three.sock"
	run.writeConfig(t, `{"tools": {"plugins": true}}`)
	run.stderr.Reset()
	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("fourth startup exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if run.stderr.Len() != 0 {
		t.Fatalf("fourth startup stderr = %q, want silence once a tool is on", run.stderr.String())
	}

	hint, err := tools.HintPath(run.deps.getenv)
	if err != nil {
		t.Fatalf("HintPath: %v", err)
	}
	recorded, err := os.ReadFile(hint)
	if err != nil {
		t.Fatalf("read %s: %v", hint, err)
	}
	if strings.Contains(string(recorded), "/tmp/herdr-three.sock") {
		t.Fatalf("the gate recorded a hint for a socket it should stay silent on:\n%s", recorded)
	}
}
