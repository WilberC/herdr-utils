package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/WilberC/herdr-utils/internal/plugins"
)

const pluginsUsage = `usage: hu plugins <command> [options]

Keep the recommended herdr plugins installed on this machine. The catalog is
plugins.toml beside the binary or in the working directory.

Commands:
  list     Show every catalog plugin and whether this machine has it
  sync     Install the catalog plugins this machine is missing
  render   Regenerate this README's "Recommended plugins" section (maintainers)

Options:
  --json           Print one JSON document (list, sync)
  --dry-run        Print the commands sync would run, and run none
  --optional       Include the catalog entries marked optional (sync)
  --readme <path>  Render into that README instead of ./README.md (render)

sync is an explicit command: it runs whether or not tools.plugins is on. That
toggle gates the [[startup]] automation, never a command you typed.
`

func runPlugins(d deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(d.err, pluginsUsage)
		return 2
	}

	switch args[0] {
	case "list":
		return runPluginsList(d, args[1:])
	case "sync":
		return runPluginsSync(d, args[1:])
	case "render":
		return runPluginsRender(d, args[1:])
	}
	return report(usageError{reason: fmt.Sprintf("unknown plugins command %q", args[0]), usage: pluginsUsage}, d.err)
}

type pluginFlags struct {
	json     bool
	dryRun   bool
	optional bool
	readme   string
}

func runPluginsList(d deps, args []string) int {
	flags, err := parsePluginsFlags("list", args, pluginsUsage, pluginFlagJSON)
	if err != nil {
		return report(err, d.err)
	}

	catalog, err := loadCatalog(d)
	if err != nil {
		return report(err, d.err)
	}
	items, err := plugins.Inspect(context.Background(), d.client, catalog)
	if err != nil {
		return report(err, d.err)
	}

	if flags.json {
		document := listDocument{Plugins: make([]listPlugin, 0, len(items))}
		for _, item := range items {
			document.Plugins = append(document.Plugins, listPlugin{
				Repo:         item.Entry.Repo,
				Reason:       item.Entry.Reason,
				Optional:     item.Entry.Optional,
				Ref:          item.Entry.Ref,
				Installed:    item.Installed,
				InstalledRef: item.InstalledRef,
				RefMatch:     item.RefMatch,
				State:        item.State(),
			})
		}
		return report(writeJSON(d.out, document), d.err)
	}

	table := tabwriter.NewWriter(d.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "PLUGIN\tSTATE\tKIND\tREF\tWHY")
	for _, item := range items {
		ref := item.RefState()
		if ref == "" {
			ref = "-"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			item.Entry.Repo, item.State(), item.Entry.Kind(), ref, item.Entry.Reason)
	}
	if err := table.Flush(); err != nil {
		return report(err, d.err)
	}
	return 0
}

type listDocument struct {
	Plugins []listPlugin `json:"plugins"`
}

type listPlugin struct {
	Repo         string `json:"repo"`
	Reason       string `json:"reason"`
	Optional     bool   `json:"optional"`
	Ref          string `json:"ref,omitempty"`
	Installed    bool   `json:"installed"`
	InstalledRef string `json:"installed_ref,omitempty"`
	RefMatch     *bool  `json:"ref_match,omitempty"`
	State        string `json:"state"`
}

func runPluginsSync(d deps, args []string) int {
	flags, err := parsePluginsFlags("sync", args, pluginsUsage, pluginFlagJSON, pluginFlagDryRun, pluginFlagOptional)
	if err != nil {
		return report(err, d.err)
	}

	catalog, err := loadCatalog(d)
	if err != nil {
		return report(err, d.err)
	}
	reportResult, err := plugins.Sync(context.Background(), d.client, catalog, plugins.Options{
		DryRun:   flags.dryRun,
		Optional: flags.optional,
	})
	if err != nil {
		return report(err, d.err)
	}

	if flags.json {
		document := syncDocument{
			DryRun:   flags.dryRun,
			Optional: flags.optional,
			Entries:  make([]syncEntry, 0, len(reportResult.Results)),
			Summary: syncSummary{
				Installed:    reportResult.Installed,
				Failed:       reportResult.Failed,
				Skipped:      reportResult.Skipped,
				Current:      reportResult.Current,
				WouldInstall: reportResult.WouldInstall,
			},
		}
		for _, result := range reportResult.Results {
			document.Entries = append(document.Entries, syncEntry{
				Repo:    result.Entry.Repo,
				Action:  result.Action,
				Detail:  result.Detail,
				Command: result.Command,
			})
		}
		if err := writeJSON(d.out, document); err != nil {
			return report(err, d.err)
		}
		return exitCode(reportResult.Failed == 0)
	}

	table := tabwriter.NewWriter(d.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "PLUGIN\tACTION\tDETAIL")
	for _, result := range reportResult.Results {
		fmt.Fprintf(table, "%s\t%s\t%s\n", result.Entry.Repo, result.Action, result.Detail)
	}
	if err := table.Flush(); err != nil {
		return report(err, d.err)
	}
	fmt.Fprintln(d.out, reportResult.Summary())
	return exitCode(reportResult.Failed == 0)
}

type syncDocument struct {
	DryRun   bool        `json:"dry_run"`
	Optional bool        `json:"optional"`
	Entries  []syncEntry `json:"entries"`
	Summary  syncSummary `json:"summary"`
}

type syncEntry struct {
	Repo    string `json:"repo"`
	Action  string `json:"action"`
	Detail  string `json:"detail"`
	Command string `json:"command,omitempty"`
}

type syncSummary struct {
	Installed    int `json:"installed"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	Current      int `json:"current"`
	WouldInstall int `json:"would_install,omitempty"`
}

func runPluginsRender(d deps, args []string) int {
	flags, err := parsePluginsFlags("render", args, pluginsUsage, pluginFlagReadme)
	if err != nil {
		return report(err, d.err)
	}

	catalog, err := loadCatalog(d)
	if err != nil {
		return report(err, d.err)
	}

	readme := flags.readme
	if readme == "" {
		cwd, err := workingDirectory(d)
		if err != nil {
			return report(err, d.err)
		}
		readme = filepath.Join(cwd, "README.md")
	}
	document, err := os.ReadFile(readme)
	if err != nil {
		return report(fmt.Errorf("read %s: %w", readme, err), d.err)
	}

	rendered, err := plugins.ReplaceSection(string(document), plugins.Render(catalog))
	if err != nil {
		return report(fmt.Errorf("%s: %w", readme, err), d.err)
	}
	if rendered == string(document) {
		fmt.Fprintf(d.out, "%s is current\n", readme)
		return 0
	}
	if err := os.WriteFile(readme, []byte(rendered), 0o644); err != nil {
		return report(fmt.Errorf("write %s: %w", readme, err), d.err)
	}
	fmt.Fprintf(d.out, "rendered %s\n", readme)
	return 0
}

// loadCatalog reads the catalog this installation ships.
func loadCatalog(d deps) (plugins.Catalog, error) {
	path, err := locateCatalog(d)
	if err != nil {
		return plugins.Catalog{}, err
	}
	return plugins.Read(path)
}

func locateCatalog(d deps) (string, error) {
	executable := ""
	if d.executable != nil {
		if path, err := d.executable(); err == nil {
			executable = path
		}
	}
	cwd, err := workingDirectory(d)
	if err != nil {
		return "", err
	}
	return plugins.Locate("", executable, cwd)
}

func workingDirectory(d deps) (string, error) {
	if d.cwd == nil {
		return "", fmt.Errorf("cannot locate the working directory")
	}
	cwd, err := d.cwd()
	if err != nil {
		return "", fmt.Errorf("cannot locate the working directory: %w", err)
	}
	return cwd, nil
}

const (
	pluginFlagJSON     = "--json"
	pluginFlagDryRun   = "--dry-run"
	pluginFlagOptional = "--optional"
	pluginFlagReadme   = "--readme"
)

// parsePluginsFlags reads the flags one plugins subcommand accepts. A flag the
// subcommand does not document is refused with the top-level usage, so an
// unsupported combination never silently changes what a sync does.
func parsePluginsFlags(name string, args []string, usage string, allowed ...string) (pluginFlags, error) {
	var flags pluginFlags
	permitted := make(map[string]bool, len(allowed))
	for _, flag := range allowed {
		permitted[flag] = true
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == pluginFlagReadme && permitted[pluginFlagReadme]:
			if index+1 >= len(args) {
				return pluginFlags{}, usageError{reason: "--readme needs a path", usage: usage}
			}
			index++
			flags.readme = args[index]
		case arg == pluginFlagJSON && permitted[pluginFlagJSON]:
			flags.json = true
		case arg == pluginFlagDryRun && permitted[pluginFlagDryRun]:
			flags.dryRun = true
		case arg == pluginFlagOptional && permitted[pluginFlagOptional]:
			flags.optional = true
		case strings.HasPrefix(arg, "-"):
			return pluginFlags{}, usageError{reason: fmt.Sprintf("unknown flag %q", arg), usage: usage}
		default:
			return pluginFlags{}, usageError{reason: name + " takes no arguments", usage: usage}
		}
	}
	return flags, nil
}
