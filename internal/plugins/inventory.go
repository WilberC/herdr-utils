package plugins

import (
	"context"
	"fmt"

	"github.com/WilberC/herdr-utils/internal/herdrmock"
)

// Item is one catalog entry compared with this machine: whether herdr has it,
// the ref it reports, and whether that ref matches a pinned entry.
type Item struct {
	Entry        Entry
	Installed    bool
	InstalledRef string
	RefMatch     *bool
	RefKnown     bool
}

// Inspect reads the installed inventory once and returns the catalog in order.
func Inspect(ctx context.Context, client herdrmock.PluginClient, catalog Catalog) ([]Item, error) {
	installed, err := client.Plugins(ctx)
	if err != nil {
		return nil, err
	}

	refs := make(map[string]string, len(installed))
	for _, plugin := range installed {
		refs[plugin.ID] = plugin.Ref
	}

	items := make([]Item, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		ref, present := refs[entry.Repo]
		item := Item{Entry: entry, Installed: present, InstalledRef: ref}
		if present && entry.Ref != "" {
			match := ref != "" && ref == entry.Ref
			item.RefMatch = &match
			item.RefKnown = ref != ""
		}
		items = append(items, item)
	}
	return items, nil
}

// InstallCommand is the exact herdr invocation sync runs or prints.
func InstallCommand(repo, ref string) string {
	command := "herdr plugin install " + repo + " -y"
	if ref != "" {
		command += " --ref " + ref
	}
	return command
}

// Options are the sync switches: whether to plan only, and whether optional
// catalog entries are selected.
type Options struct {
	DryRun   bool
	Optional bool
}

// The actions a sync reports per entry.
const (
	ActionCurrent      = "current"
	ActionInstalled    = "installed"
	ActionWouldInstall = "would install"
	ActionFailed       = "failed"
	ActionSkipped      = "skipped"
)

// Result is one entry's outcome.
type Result struct {
	Entry   Entry
	Action  string
	Detail  string
	Command string
}

// Report is what a sync did or would do, with the counts the summary prints.
type Report struct {
	Options      Options
	Results      []Result
	Installed    int
	Failed       int
	Skipped      int
	Current      int
	WouldInstall int
}

// Sync compares the catalog with the installed inventory and installs what is
// missing. A failed install is recorded per entry and the rest continue; only a
// failure to read the inventory, or to reach herdr at all, is an error.
func Sync(ctx context.Context, client herdrmock.PluginClient, catalog Catalog, options Options) (Report, error) {
	items, err := Inspect(ctx, client, catalog)
	if err != nil {
		return Report{}, err
	}

	report := Report{Options: options, Results: make([]Result, 0, len(items))}
	for _, item := range items {
		result := Result{Entry: item.Entry}
		switch {
		case item.Installed:
			result.Action = ActionCurrent
			result.Detail = currentDetail(item)
			report.Current++
		case item.Entry.Optional && !options.Optional:
			result.Action = ActionSkipped
			result.Detail = "optional; pass --optional to install"
			report.Skipped++
		case options.DryRun:
			result.Action = ActionWouldInstall
			result.Command = InstallCommand(item.Entry.Repo, item.Entry.Ref)
			result.Detail = result.Command
			report.WouldInstall++
		default:
			result.Command = InstallCommand(item.Entry.Repo, item.Entry.Ref)
			if err := client.InstallPlugin(ctx, item.Entry.Repo, item.Entry.Ref); err != nil {
				result.Action = ActionFailed
				result.Detail = err.Error()
				report.Failed++
			} else {
				result.Action = ActionInstalled
				result.Detail = result.Command
				report.Installed++
			}
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

// currentDetail says what an already-installed entry's state is worth: pinned
// refs are compared, and an unpinned entry just reports itself installed.
func currentDetail(item Item) string {
	if item.RefMatch == nil {
		return "installed"
	}
	if *item.RefMatch {
		return "installed at " + item.InstalledRef
	}
	if !item.RefKnown {
		return "installed; herdr reports no ref, catalog pins " + item.Entry.Ref
	}
	return fmt.Sprintf("installed at %s; catalog pins %s", item.InstalledRef, item.Entry.Ref)
}

// Summary is the stable one-line result of a sync.
func (r Report) Summary() string {
	verb := "installed"
	count := r.Installed
	if r.Options.DryRun {
		verb = "would install"
		count = r.WouldInstall
	}
	return fmt.Sprintf("%s %d, failed %d, skipped %d, current %d", verb, count, r.Failed, r.Skipped, r.Current)
}

// Detail is the list report's state detail for one item.
func (i Item) State() string {
	if i.Installed {
		return ActionInstalled
	}
	return "missing"
}

// RefState is the list report's ref column: empty when the entry is unpinned,
// otherwise the comparison with what herdr reports.
func (i Item) RefState() string {
	if i.RefMatch == nil {
		return ""
	}
	switch {
	case *i.RefMatch:
		return "match"
	case !i.RefKnown:
		return "unknown"
	default:
		return "mismatch"
	}
}

// Kind is the entry's catalog kind.
func (e Entry) Kind() string {
	if e.Optional {
		return "optional"
	}
	return "core"
}

// Pinned reports whether the entry declares a ref.
func (e Entry) Pinned() bool { return e.Ref != "" }
