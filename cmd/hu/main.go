package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/WilberC/herdr-utils/internal/keybind"
	"time"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/herdrcli"
	"github.com/WilberC/herdr-utils/internal/herdrmock"
	"github.com/WilberC/herdr-utils/internal/tools"
)

const usage = `usage: hu [command]

Commands:
  which     Open the keybinding guide for herdr
  keys      Keep the herdr-utils.which action bound in your herdr config
  plugins   Keep the recommended herdr plugins installed
  startup   Apply the config gate; herdr runs this at server start
  version   Print the version this binary was built as

Every tool is default off; enable one in the plugin config.
Run "hu <command> --help" for command details.
`

const whichUsage = `usage: hu which

Open the keybinding guide for herdr: one screen over the bindings you have,
grouped by source.

The guide is default off: set tools.which to true in the plugin config, at
$HERDR_PLUGIN_CONFIG_DIR/config.json or the herdr-utils entry under
$XDG_CONFIG_HOME/herdr/plugins/config, then bind the herdr-utils.which action
to a key.

Sources, each default off under which.sources:
  hs          the [[keys.command]] entries in $HERDR_CONFIG_PATH or
              ~/.config/herdr/config.toml
  gentle      the manual entries in which.gentle
  custom      the manual entries in which.custom
  discovered  every action the installed plugins declare

In the guide, type to filter, enter on a bound key shows the command it runs,
enter on a discovered action runs it through herdr, ctrl+t switches between the
KEYS and LIST views (saved to the plugin config), and esc closes.
`

const startupUsage = `usage: hu startup

Apply what herdr runs this plugin for at server start.

First the which binding: the fenced block binding the herdr-utils.which action
in your herdr config (hu keys install), so a config rewrite or a fresh machine
never loses the key the guide opens on. Then the config gate: with every tool
off this prints one hint once per server socket and changes nothing else; with
any tool on there is nothing to start. It never blocks a herdr start.
`

type command struct {
	name    string
	summary string
	usage   string
}

var commands = []command{
	{name: "which", summary: "Open the keybinding guide for herdr", usage: whichUsage},
	{name: "keys", summary: "Keep the guide action bound in your herdr config", usage: keysUsage},
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
	in         io.Reader
	out        io.Writer
	err        io.Writer

	// guide is the terminal seam of hu which: nil runs the Bubbletea program on
	// in and out, and a test records the built guide instead.
	guide guideFunc

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
		in:         os.Stdin,
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
		return runWhich(d, args[1:])
	case "keys":
		return runKeys(d, args[1:])
	case "plugins":
		return runPlugins(d, args[1:])
	case "startup":
		return runStartup(d, args[1:])
	case "version":
		return runVersion(d, args[1:])
	}

	return unknownCommand(cmd.name, stderr)
}

func runStartup(d deps, args []string) int {
	if len(args) != 0 {
		return report(usageError{reason: "startup takes no arguments", usage: startupUsage}, d.err)
	}

	if result, err := keybind.EnsureFile(d.getenv); err != nil {
		fmt.Fprintf(d.err, "herdr-utils: %v; the guide binding stays as it is\n", err)
	} else if result.Changed {
		fmt.Fprintf(d.out, "herdr-utils: bound the guide on %s; reload herdr to pick it up\n", keybind.Key)
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
