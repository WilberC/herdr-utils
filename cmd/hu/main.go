package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrcli"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/tools"
)

const usage = `usage: hu [command]

Commands:
  which     Open the keybinding guide for herdr
  plugins   Keep the recommended herdr plugins installed
  startup   Apply the config gate; herdr runs this at server start
  version   Print the version this binary was built as

Every tool is default off; enable one in the plugin config.
Run "hu <command> --help" for command details.
`

const whichUsage = `usage: hu which

Open the keybinding guide for herdr.

The guide is default off: set tools.which to true in the plugin config, at
$HERDR_PLUGIN_CONFIG_DIR/config.json or the herdr-utils entry under
$XDG_CONFIG_HOME/herdr/plugins/config, then bind the herdr-utils.which action
to a key.

Every source of bindings is default off as well, under which.sources: hs,
gentle, custom and discovered.
`

const startupUsage = `usage: hu startup

Apply the config gate at herdr server start.

With every tool off this prints one hint once per server socket and changes
nothing else; with any tool on it does nothing, because there is nothing to
install. It never blocks a herdr start.
`

type command struct {
	name    string
	summary string
	usage   string
}

var commands = []command{
	{name: "which", summary: "Open the keybinding guide for herdr", usage: whichUsage},
	{name: "plugins", summary: "Keep the recommended herdr plugins installed", usage: pluginsUsage},
	{name: "startup", summary: "Apply the config gate at server start", usage: startupUsage},
	{name: "version", summary: "Print the build version", usage: versionUsage},
}

type deps struct {
	client     herdrmock.Client
	getenv     func(string) string
	now        func() time.Time
	cwd        func() (string, error)
	executable func() (string, error)
	out        io.Writer
	err        io.Writer

	// version is the build version the binary reports. It is a field so a test
	// can pin an embedded release version; a build without one keeps the dev
	// default.
	version string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWith(defaultDeps(stdout, stderr), args, stdout, stderr)
}

func defaultDeps(stdout, stderr io.Writer) deps {
	return deps{
		client:     herdrcli.New(),
		getenv:     os.Getenv,
		now:        time.Now,
		cwd:        os.Getwd,
		executable: os.Executable,
		version:    version,
		out:        stdout,
		err:        stderr,
	}
}

func runWith(d deps, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if isHelp(args[0]) {
		fmt.Fprint(stdout, usage)
		return 0
	}

	cmd, ok := lookup(args[0])
	if !ok {
		return unknownCommand(args[0], stderr)
	}
	if hasHelp(args[1:]) {
		fmt.Fprint(stdout, cmd.usage)
		return 0
	}

	switch cmd.name {
	case "which":
		return runScaffold(d, cmd, args[1:])
	case "plugins":
		return runPlugins(d, args[1:])
	case "startup":
		return runStartup(d, args[1:])
	case "version":
		return runVersion(d, args[1:])
	}

	return unknownCommand(cmd.name, stderr)
}

// runScaffold is what a command answers before the work it names is implemented:
// its usage on stderr and the refusal status, so a script never mistakes it for
// success.
func runScaffold(d deps, cmd command, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(d.err, cmd.usage)
		return 2
	}
	if strings.HasPrefix(args[0], "-") {
		return report(usageError{reason: fmt.Sprintf("unknown flag %q", args[0]), usage: cmd.usage}, d.err)
	}
	return report(usageError{reason: fmt.Sprintf("%s takes no arguments", cmd.name), usage: cmd.usage}, d.err)
}

func runStartup(d deps, args []string) int {
	if len(args) != 0 {
		return report(usageError{reason: "startup takes no arguments", usage: startupUsage}, d.err)
	}

	document, err := config.Load(d.getenv)
	if err != nil {
		fmt.Fprintf(d.err, "herdr-utils: %v; every tool stays off\n", err)
		return 0
	}

	if tools.Decide(document.Config) == tools.PlanNothing {
		return 0
	}

	first, err := tools.TakeHint(d.getenv, d.clock())
	if err != nil {
		return report(err, d.err)
	}
	if !first {
		return 0
	}

	path, err := config.Path(d.getenv)
	if err != nil {
		return report(err, d.err)
	}
	fmt.Fprintf(d.err, "herdr-utils: every tool is off; enable one in %s (for example tools.which = true for the keybinding guide)\n", path)
	return 0
}

// clock is time.Now behind the deps seam, with a working default so a deps value
// built without one still reports a time.
func (d deps) clock() time.Time {
	if d.now == nil {
		return time.Now()
	}
	return d.now()
}

// unknownCommand is what any name the table does not dispatch answers: the exit
// status scripts rely on, with the usage they can read.
func unknownCommand(name string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "unknown command %q\n\n%s", name, usage)
	return 2
}

// usageError is a command-line mistake: it prints its reason, then the usage of
// the command that rejected the arguments.
type usageError struct {
	reason string
	usage  string
}

func (e usageError) Error() string { return e.reason }

// report turns an error into the exit status contract callers rely on: 0 when
// nothing failed, 2 for bad arguments, and 1 when herdr failed.
func report(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}

	var usageErr usageError
	if errors.As(err, &usageErr) {
		fmt.Fprintf(stderr, "hu: %s\n\n%s", usageErr.reason, usageErr.usage)
		return 2
	}

	fmt.Fprintf(stderr, "hu: %v\n", err)
	return 1
}

func lookup(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}
	return command{}, false
}

func isHelp(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if isHelp(arg) {
			return true
		}
	}
	return false
}
