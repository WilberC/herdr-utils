package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/which"
)

// TestWhichRefusesWhileTheToolIsOff pins the tool gate: a source the user turned
// on does nothing while tools.which is off, and the refusal names the fix.
func TestWhichRefusesWhileTheToolIsOff(t *testing.T) {
	for name, body := range map[string]string{
		"no config file at all":         "",
		"the tool off with a source on": `{"tools": {"which": false}, "which": {"sources": {"hs": true}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			run := newTestRun(t)
			if body != "" {
				run.writeConfig(t, body)
			}

			code := run.run(t, "which")

			if code != 2 {
				t.Fatalf("exit = %d, want 2\nstderr:\n%s", code, run.stderr.String())
			}
			for _, want := range []string{"tools.which", "config.json"} {
				if !strings.Contains(run.stderr.String(), want) {
					t.Fatalf("stderr = %q, want it to contain %q", run.stderr.String(), want)
				}
			}
			if run.stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want the refusal on stderr only", run.stdout.String())
			}
		})
	}
}

// TestWhichBuildsTheGuideFromTheConfigAndTheFakeClient pins the wiring the
// screen relies on: the enabled sources, the saved view, the config path, the
// persist callback that writes through internal/config, and the invoke callback
// that reaches herdr.
func TestWhichBuildsTheGuideFromTheConfigAndTheFakeClient(t *testing.T) {
	run := newTestRun(t)
	run.writeConfig(t, `{
  "tools": {"which": true},
  "which": {
    "sources": {"gentle": true, "discovered": true},
    "view": "list",
    "gentle": [{"key": "ctrl+g", "description": "Lazygit", "command": "fish -c lazygit"}]
  }
}`)
	client := &herdrmock.Fake{Actions: []herdrmock.Action{{Plugin: "herdr-bar", ID: "open", Title: "Open bar"}}}
	run.deps.client = client
	var shown *which.Guide
	run.deps.guide = func(guide *which.Guide, in io.Reader, out io.Writer) error {
		shown = guide
		return nil
	}

	code := run.run(t, "which")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if shown == nil {
		t.Fatal("the terminal seam was never reached")
	}
	if run.stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want silence", run.stderr.String())
	}

	names := make([]string, 0, len(shown.Sources))
	for _, source := range shown.Sources {
		names = append(names, source.Name())
	}
	if got, want := strings.Join(names, ","), which.NameGentle+","+which.NameDiscovered; got != want {
		t.Fatalf("sources = %q, want %q", got, want)
	}
	if want := filepath.Join(run.env["HERDR_PLUGIN_CONFIG_DIR"], "config.json"); shown.ConfigPath != want {
		t.Fatalf("ConfigPath = %q, want %q", shown.ConfigPath, want)
	}
	if shown.View != config.ViewList {
		t.Fatalf("View = %q, want the saved %q", shown.View, config.ViewList)
	}
	if entries := shown.Sources[0].Entries(); len(entries) != 1 || entries[0].Description != "Lazygit" {
		t.Fatalf("GENTLE entries = %#v, want the manual entry", entries)
	}
	if entries := shown.Sources[1].Entries(); len(entries) != 1 || entries[0].ActionID != "herdr-bar.open" {
		t.Fatalf("DISCOVERED entries = %#v, want the live action", entries)
	}

	if err := shown.Invoke(context.Background(), "herdr-bar.open"); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if calls := client.Calls(); len(calls) == 0 || calls[len(calls)-1] != (herdrmock.Call{Method: "InvokeAction", Target: "herdr-bar.open"}) {
		t.Fatalf("calls = %#v, want the action invoked through herdr", calls)
	}

	if err := shown.Persist(config.ViewKeys); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	document, err := config.Load(run.deps.getenv)
	if err != nil {
		t.Fatalf("reload the config: %v", err)
	}
	if document.Config.Which.View != config.ViewKeys {
		t.Fatalf("saved view = %q, want %q", document.Config.Which.View, config.ViewKeys)
	}
	if len(document.Config.Which.Gentle) != 1 {
		t.Fatalf("saved gentle entries = %#v, want them kept", document.Config.Which.Gentle)
	}
}

func TestWhichRefusesWithoutATerminal(t *testing.T) {
	run := newTestRun(t)
	run.writeConfig(t, `{"tools": {"which": true}, "which": {"sources": {"gentle": true}}}`)
	run.deps.in = &bytes.Buffer{}
	opened := false
	run.deps.guide = func(*which.Guide, io.Reader, io.Writer) error {
		opened = true
		return nil
	}

	code := run.run(t, "which")

	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr:\n%s", code, run.stderr.String())
	}
	if opened {
		t.Fatal("the guide opened without a terminal")
	}
	for _, want := range []string{"terminal", "herdr-utils.which"} {
		if !strings.Contains(run.stderr.String(), want) {
			t.Fatalf("stderr = %q, want it to contain %q", run.stderr.String(), want)
		}
	}
}

func TestWhichReportsABrokenConfigAndAFailedScreen(t *testing.T) {
	t.Run("a malformed config", func(t *testing.T) {
		run := newTestRun(t)
		run.writeConfig(t, "not json")

		code := run.run(t, "which")

		if code != 1 {
			t.Fatalf("exit = %d, want 1\nstderr:\n%s", code, run.stderr.String())
		}
		if !strings.Contains(run.stderr.String(), "config.json") {
			t.Fatalf("stderr = %q, want the file named", run.stderr.String())
		}
	})

	t.Run("the terminal cannot be opened", func(t *testing.T) {
		run := newTestRun(t)
		run.writeConfig(t, `{"tools": {"which": true}}`)
		run.deps.guide = func(*which.Guide, io.Reader, io.Writer) error {
			return errors.New("could not open a new TTY")
		}

		code := run.run(t, "which")

		if code != 1 {
			t.Fatalf("exit = %d, want 1\nstderr:\n%s", code, run.stderr.String())
		}
		if !strings.Contains(run.stderr.String(), "could not open a new TTY") {
			t.Fatalf("stderr = %q, want the screen failure reported", run.stderr.String())
		}
	})
}
