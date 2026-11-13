package plugins

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

func catalogOf(entries ...Entry) Catalog { return Catalog{Entries: entries} }

func core(repo string) Entry { return Entry{Repo: repo, Reason: "a core plugin"} }

func optional(repo string) Entry {
	return Entry{Repo: repo, Reason: "an optional plugin", Optional: true}
}

func TestInspectReportsInstalledAndMissing(t *testing.T) {
	client := &herdrmock.Fake{Installed: []herdrmock.Plugin{{ID: "owner/core", Ref: "v1"}}}

	items, err := Inspect(context.Background(), client, catalogOf(core("owner/core"), core("owner/gone")))
	if err != nil {
		t.Fatalf("Inspect error = %v, want nil", err)
	}

	if len(items) != 2 {
		t.Fatalf("Inspect = %#v, want two items", items)
	}
	if !items[0].Installed || items[0].InstalledRef != "v1" {
		t.Fatalf("item 0 = %#v, want installed at v1", items[0])
	}
	if items[1].Installed {
		t.Fatalf("item 1 = %#v, want missing", items[1])
	}
}

func TestInspectComparesAPinnedRef(t *testing.T) {
	entry := core("owner/repo")
	entry.Ref = "v2"

	tests := []struct {
		name          string
		installedRef  string
		wantRefMatch  bool
		wantRefKnown  bool
		wantReasonBit string
	}{
		{name: "matching ref", installedRef: "v2", wantRefMatch: true, wantRefKnown: true},
		{name: "mismatched ref", installedRef: "v1", wantRefMatch: false, wantRefKnown: true},
		{name: "herdr reports no ref", installedRef: "", wantRefMatch: false, wantRefKnown: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &herdrmock.Fake{Installed: []herdrmock.Plugin{{ID: "owner/repo", Ref: tt.installedRef}}}

			items, err := Inspect(context.Background(), client, catalogOf(entry))
			if err != nil {
				t.Fatalf("Inspect error = %v, want nil", err)
			}
			got := items[0]
			if got.RefMatch == nil {
				t.Fatalf("RefMatch = nil, want it reported for a pinned entry")
			}
			if *got.RefMatch != tt.wantRefMatch {
				t.Fatalf("RefMatch = %v, want %v", *got.RefMatch, tt.wantRefMatch)
			}
			if got.RefKnown != tt.wantRefKnown {
				t.Fatalf("RefKnown = %v, want %v", got.RefKnown, tt.wantRefKnown)
			}
		})
	}
}

func TestInspectLeavesRefOpenWhenUnpinned(t *testing.T) {
	client := &herdrmock.Fake{Installed: []herdrmock.Plugin{{ID: "owner/repo", Ref: "v9"}}}

	items, err := Inspect(context.Background(), client, catalogOf(core("owner/repo")))
	if err != nil {
		t.Fatalf("Inspect error = %v, want nil", err)
	}
	if items[0].RefMatch != nil {
		t.Fatalf("RefMatch = %v, want nil for an unpinned entry", *items[0].RefMatch)
	}
}

func TestInspectReportsAHerdrFailure(t *testing.T) {
	client := &herdrmock.Fake{InstalledErr: errors.New("no server running")}

	_, err := Inspect(context.Background(), client, catalogOf(core("owner/repo")))

	if err == nil || !strings.Contains(err.Error(), "no server running") {
		t.Fatalf("Inspect error = %v, want the herdr failure", err)
	}
}

func TestSyncInstallsMissingAndSkipsOptionalByDefault(t *testing.T) {
	client := &herdrmock.Fake{}
	catalog := catalogOf(core("owner/core"), optional("owner/opt"))

	report, err := Sync(context.Background(), client, catalog, Options{})
	if err != nil {
		t.Fatalf("Sync error = %v, want nil", err)
	}

	if report.Installed != 1 || report.Failed != 0 || report.Skipped != 1 || report.Current != 0 {
		t.Fatalf("report = %#v, want installed 1, skipped 1", report)
	}
	installs := installCalls(client)
	if len(installs) != 1 || installs[0] != "owner/core" {
		t.Fatalf("installs = %v, want [owner/core]", installs)
	}
}

func TestSyncInstallsOptionalWhenAsked(t *testing.T) {
	client := &herdrmock.Fake{}
	catalog := catalogOf(core("owner/core"), optional("owner/opt"))

	report, err := Sync(context.Background(), client, catalog, Options{Optional: true})
	if err != nil {
		t.Fatalf("Sync error = %v, want nil", err)
	}

	if report.Installed != 2 || report.Skipped != 0 {
		t.Fatalf("report = %#v, want installed 2", report)
	}
	if installs := installCalls(client); len(installs) != 2 {
		t.Fatalf("installs = %v, want both entries", installs)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	client := &herdrmock.Fake{}
	catalog := catalogOf(core("owner/core"))

	if _, err := Sync(context.Background(), client, catalog, Options{}); err != nil {
		t.Fatalf("first Sync error = %v, want nil", err)
	}
	report, err := Sync(context.Background(), client, catalog, Options{})
	if err != nil {
		t.Fatalf("second Sync error = %v, want nil", err)
	}

	if report.Installed != 0 || report.Current != 1 {
		t.Fatalf("second report = %#v, want everything current", report)
	}
	if installs := installCalls(client); len(installs) != 1 {
		t.Fatalf("installs = %v, want the first run's install only", installs)
	}
}

func TestSyncReportsAFailedEntryAndContinues(t *testing.T) {
	client := &herdrmock.Fake{
		InstallErrs: map[string]error{"owner/bad": errors.New("no such repository")},
	}
	catalog := catalogOf(core("owner/bad"), core("owner/good"))

	report, err := Sync(context.Background(), client, catalog, Options{})
	if err != nil {
		t.Fatalf("Sync error = %v, want nil", err)
	}

	if report.Installed != 1 || report.Failed != 1 {
		t.Fatalf("report = %#v, want installed 1, failed 1", report)
	}
	if installs := installCalls(client); len(installs) != 2 {
		t.Fatalf("installs = %v, want both entries attempted", installs)
	}
	for _, result := range report.Results {
		if result.Entry.Repo == "owner/bad" && !strings.Contains(result.Detail, "no such repository") {
			t.Fatalf("failed entry detail = %q, want the herdr error", result.Detail)
		}
	}
}

func TestSyncDryRunRunsNothing(t *testing.T) {
	client := &herdrmock.Fake{}
	catalog := catalogOf(core("owner/core"), optional("owner/opt"))

	report, err := Sync(context.Background(), client, catalog, Options{DryRun: true, Optional: true})
	if err != nil {
		t.Fatalf("Sync error = %v, want nil", err)
	}

	if installs := installCalls(client); len(installs) != 0 {
		t.Fatalf("dry run installed %v", installs)
	}
	if report.WouldInstall != 2 || report.Installed != 0 {
		t.Fatalf("report = %#v, want 2 planned and 0 installed", report)
	}
	for _, result := range report.Results {
		if result.Command != InstallCommand(result.Entry.Repo, result.Entry.Ref) {
			t.Fatalf("result command = %q, want the exact install command", result.Command)
		}
	}
}

func TestSyncPinnedEntryPassesItsRef(t *testing.T) {
	client := &herdrmock.Fake{}
	entry := core("owner/core")
	entry.Ref = "v2"

	if _, err := Sync(context.Background(), client, catalogOf(entry), Options{}); err != nil {
		t.Fatalf("Sync error = %v, want nil", err)
	}

	if len(client.Installed) != 1 || client.Installed[0].Ref != "v2" {
		t.Fatalf("installed = %#v, want the entry pinned to v2", client.Installed)
	}
}

func TestInstallCommandIsTheHerdrInvocation(t *testing.T) {
	if got, want := InstallCommand("owner/repo", ""), "herdr plugin install owner/repo -y"; got != want {
		t.Fatalf("InstallCommand = %q, want %q", got, want)
	}
	if got, want := InstallCommand("owner/repo", "v1"), "herdr plugin install owner/repo -y --ref v1"; got != want {
		t.Fatalf("InstallCommand = %q, want %q", got, want)
	}
}

func installCalls(client *herdrmock.Fake) []string {
	var targets []string
	for _, call := range client.Calls() {
		if call.Method == "InstallPlugin" {
			targets = append(targets, call.Target)
		}
	}
	return targets
}
