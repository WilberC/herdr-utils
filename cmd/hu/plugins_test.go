package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/plugins"
)

const testCatalog = `[[plugin]]
repo = "owner/core"
reason = "the core plugin"

[[plugin]]
repo = "owner/other"
reason = "another core plugin"

[[plugin]]
repo = "owner/optional"
reason = "an optional plugin"
optional = true
`

// seedCatalog writes the catalog in the working directory the test runs in, so
// the binary resolves it exactly the way an installation does.
func (r *testRun) seedCatalog(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.root, "plugins.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write the catalog: %v", err)
	}
}

func (r *testRun) fake(installed ...string) *herdrmock.Fake {
	fake := &herdrmock.Fake{}
	for _, repo := range installed {
		fake.Installed = append(fake.Installed, herdrmock.Plugin{ID: repo})
	}
	r.deps.client = fake
	return fake
}

func TestPluginsListTable(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	run.fake("owner/core")

	code := run.run(t, "plugins", "list")

	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, run.stdout.String(), run.stderr.String())
	}
	for _, want := range []string{
		"PLUGIN", "STATE", "KIND", "WHY",
		"owner/core", "installed", "the core plugin",
		"owner/other", "missing",
		"owner/optional", "optional",
	} {
		if !strings.Contains(run.stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, run.stdout.String())
		}
	}
	if run.stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want silence", run.stderr.String())
	}
}

func TestPluginsListSucceedsWhenNothingIsInstalled(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	run.fake()

	if code := run.run(t, "plugins", "list"); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stdout.String(), "missing") {
		t.Fatalf("stdout = %q, want the missing state", run.stdout.String())
	}
}

func TestPluginsListJSON(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	run.fake("owner/core")

	if code := run.run(t, "plugins", "list", "--json"); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}

	var document struct {
		Plugins []struct {
			Repo      string `json:"repo"`
			Reason    string `json:"reason"`
			Optional  bool   `json:"optional"`
			Installed bool   `json:"installed"`
			State     string `json:"state"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(run.stdout.Bytes(), &document); err != nil {
		t.Fatalf("decode the list document: %v\n%s", err, run.stdout.String())
	}
	if len(document.Plugins) != 3 {
		t.Fatalf("plugins = %#v, want the whole catalog", document.Plugins)
	}
	if !document.Plugins[0].Installed || document.Plugins[0].State != "installed" {
		t.Fatalf("first entry = %#v, want installed", document.Plugins[0])
	}
	if document.Plugins[2].State != "missing" || !document.Plugins[2].Optional {
		t.Fatalf("third entry = %#v, want missing and optional", document.Plugins[2])
	}
}

func TestPluginsListReportsAPinnedRef(t *testing.T) {
	pinned := strings.Replace(testCatalog, "reason = \"the core plugin\"", "reason = \"the core plugin\"\nref = \"v2\"", 1)

	t.Run("matching", func(t *testing.T) {
		run := newTestRun(t)
		run.seedCatalog(t, pinned)
		run.deps.client = &herdrmock.Fake{Installed: []herdrmock.Plugin{{ID: "owner/core", Ref: "v2"}}}

		if code := run.run(t, "plugins", "list"); code != 0 {
			t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
		}
		if !strings.Contains(run.stdout.String(), "match") {
			t.Fatalf("stdout = %q, want the ref match reported", run.stdout.String())
		}
	})

	t.Run("mismatched", func(t *testing.T) {
		run := newTestRun(t)
		run.seedCatalog(t, pinned)
		run.deps.client = &herdrmock.Fake{Installed: []herdrmock.Plugin{{ID: "owner/core", Ref: "v1"}}}

		if code := run.run(t, "plugins", "list"); code != 0 {
			t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
		}
		if !strings.Contains(run.stdout.String(), "mismatch") {
			t.Fatalf("stdout = %q, want the ref mismatch reported", run.stdout.String())
		}
	})
}

func TestPluginsSyncInstallsMissingAndSkipsOptional(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	fake := run.fake()

	code := run.run(t, "plugins", "sync")

	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, run.stdout.String(), run.stderr.String())
	}
	if len(fake.Installed) != 2 {
		t.Fatalf("installed = %#v, want the two core entries", fake.Installed)
	}
	for _, want := range []string{"installed 2", "failed 0", "skipped 1", "current 0"} {
		if !strings.Contains(run.stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, run.stdout.String())
		}
	}

	run.stdout.Reset()
	if code := run.run(t, "plugins", "sync"); code != 0 {
		t.Fatalf("second exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stdout.String(), "current 2") || !strings.Contains(run.stdout.String(), "installed 0") {
		t.Fatalf("second stdout = %q, want everything current", run.stdout.String())
	}
	if len(fake.Installed) != 2 {
		t.Fatalf("installed = %#v, want the second run to install nothing", fake.Installed)
	}
}

func TestPluginsSyncOptional(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	fake := run.fake()

	if code := run.run(t, "plugins", "sync", "--optional"); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if len(fake.Installed) != 3 {
		t.Fatalf("installed = %#v, want every entry", fake.Installed)
	}
}

func TestPluginsSyncDryRunPrintsCommandsAndInstallsNothing(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	fake := run.fake()

	code := run.run(t, "plugins", "sync", "--dry-run", "--optional")

	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	if len(fake.Installed) != 0 {
		t.Fatalf("installed = %#v, want a dry run to install nothing", fake.Installed)
	}
	for _, call := range fake.Calls() {
		if call.Method == "InstallPlugin" {
			t.Fatalf("dry run called InstallPlugin(%q)", call.Target)
		}
	}
	for _, want := range []string{
		"herdr plugin install owner/core -y",
		"herdr plugin install owner/optional -y",
		"would install 3",
	} {
		if !strings.Contains(run.stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, run.stdout.String())
		}
	}
}

func TestPluginsSyncReportsAFailureAndContinues(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	fake := run.fake()
	fake.InstallErrs = map[string]error{"owner/core": errors.New("no such repository")}

	code := run.run(t, "plugins", "sync")

	if code != 1 {
		t.Fatalf("exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, run.stdout.String(), run.stderr.String())
	}
	for _, want := range []string{"failed", "no such repository", "owner/other", "installed 1", "failed 1"} {
		if !strings.Contains(run.stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, run.stdout.String())
		}
	}
}

func TestPluginsSyncJSON(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	run.fake()

	if code := run.run(t, "plugins", "sync", "--json"); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}

	var document struct {
		Entries []struct {
			Repo   string `json:"repo"`
			Action string `json:"action"`
		} `json:"entries"`
		Summary struct {
			Installed int `json:"installed"`
			Failed    int `json:"failed"`
			Skipped   int `json:"skipped"`
			Current   int `json:"current"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(run.stdout.Bytes(), &document); err != nil {
		t.Fatalf("decode the sync document: %v\n%s", err, run.stdout.String())
	}
	if document.Summary.Installed != 2 || document.Summary.Skipped != 1 {
		t.Fatalf("summary = %#v, want installed 2, skipped 1", document.Summary)
	}
	if len(document.Entries) != 3 {
		t.Fatalf("entries = %#v, want one per catalog entry", document.Entries)
	}
}

func TestPluginsRefusesBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown subcommand", args: []string{"plugins", "bogus"}, want: "usage: hu plugins"},
		{name: "list refuses dry-run", args: []string{"plugins", "list", "--dry-run"}, want: `unknown flag "--dry-run"`},
		{name: "sync refuses optional for list", args: []string{"plugins", "sync", "extra"}, want: "takes no arguments"},
		{name: "render needs a readme path", args: []string{"plugins", "render", "--readme"}, want: "--readme needs a path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := newTestRun(t)
			run.seedCatalog(t, testCatalog)

			if code := run.run(t, tt.args...); code != 2 {
				t.Fatalf("exit = %d, want 2\nstdout:\n%s\nstderr:\n%s", code, run.stdout.String(), run.stderr.String())
			}
			if !strings.Contains(run.stderr.String(), tt.want) {
				t.Fatalf("stderr does not contain %q:\n%s", tt.want, run.stderr.String())
			}
		})
	}
}

func TestPluginsRefusesAMissingCatalog(t *testing.T) {
	run := newTestRun(t)

	if code := run.run(t, "plugins", "list"); code != 1 {
		t.Fatalf("exit = %d, want 1\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), "plugins.toml") {
		t.Fatalf("stderr = %q, want it to name the missing catalog", run.stderr.String())
	}
}

func TestPluginsRenderWritesTheSectionAndIsIdempotent(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	readme := filepath.Join(run.root, "README.md")
	original := "# README\n\nintro\n\n" + plugins.StartMarker + "\nstale\n" + plugins.EndMarker + "\n\ntail\n"
	if err := os.WriteFile(readme, []byte(original), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	if code := run.run(t, "plugins", "render"); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	first, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if strings.Contains(string(first), "stale") || !strings.Contains(string(first), "owner/core") {
		t.Fatalf("README was not rendered:\n%s", first)
	}
	if !strings.HasPrefix(string(first), "# README\n\nintro\n\n"+plugins.StartMarker) {
		t.Fatalf("render changed the prefix:\n%s", first)
	}
	if !strings.HasSuffix(string(first), plugins.EndMarker+"\n\ntail\n") {
		t.Fatalf("render changed the suffix:\n%s", first)
	}

	run.stdout.Reset()
	if code := run.run(t, "plugins", "render"); code != 0 {
		t.Fatalf("second exit = %d, want 0\nstderr:\n%s", code, run.stderr.String())
	}
	second, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("a second render changed the README:\n%s\n---\n%s", first, second)
	}
	if !strings.Contains(run.stdout.String(), "current") {
		t.Fatalf("second render stdout = %q, want the current state", run.stdout.String())
	}
}

func TestPluginsRenderNeedsMarkers(t *testing.T) {
	run := newTestRun(t)
	run.seedCatalog(t, testCatalog)
	if err := os.WriteFile(filepath.Join(run.root, "README.md"), []byte("# README\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	if code := run.run(t, "plugins", "render"); code != 1 {
		t.Fatalf("exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, run.stdout.String(), run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), plugins.StartMarker) {
		t.Fatalf("stderr = %q, want it to name the missing marker", run.stderr.String())
	}
}

// TestRepositoryRecommendedSectionIsRendered pins the generated README section
// against the catalog this repository ships: a hand-edited table or a catalog
// change without a render fails here.
func TestRepositoryRecommendedSectionIsRendered(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	catalog, err := plugins.Read(filepath.Join("..", "..", plugins.FileName))
	if err != nil {
		t.Fatalf("read the catalog: %v", err)
	}

	rendered, err := plugins.ReplaceSection(string(readme), plugins.Render(catalog))
	if err != nil {
		t.Fatalf("ReplaceSection: %v", err)
	}
	if rendered != string(readme) {
		t.Fatalf("README's generated section is stale; run hu plugins render:\n%s", rendered)
	}
}
