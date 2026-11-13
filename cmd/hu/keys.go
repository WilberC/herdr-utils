package main

import (
	"fmt"
	"io"
	"os"

	"github.com/WilberC/herdr-utils/internal/keybind"
)

const keysUsage = `usage: hu keys <command>

Keep the herdr-utils.which action bound in your herdr config. herdr's plugin
manifest cannot declare a keybinding, so the plugin provisions its own: a
fenced block in config.toml, on prefix+i (prefix+w is herdr's workspace
picker). The plugin owns the fenced lines and only those; a binding you wrote
yourself is never touched.

Commands:
  install   Add or repair the fenced block (run at every server start)
  remove    Strip the fenced block and nothing else
  status    Show the key the guide is bound on and where it came from

install runs whether or not any tool is on, like every command you type.
`

func runKeys(d deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(d.err, keysUsage)
		return 2
	}

	switch args[0] {
	case "install":
		return runKeysInstall(d)
	case "remove":
		return runKeysRemove(d)
	case "status":
		return runKeysStatus(d)
	}
	return report(usageError{reason: fmt.Sprintf("unknown keys command %q", args[0]), usage: keysUsage}, d.err)
}

func runKeysInstall(d deps) int {
	result, err := keybind.EnsureFile(d.getenv)
	if err != nil {
		return report(err, d.err)
	}
	return printKeysResult(result, d.out)
}

func runKeysRemove(d deps) int {
	result, err := keybind.RemoveFile(d.getenv)
	if err != nil {
		return report(err, d.err)
	}
	return printKeysResult(result, d.out)
}

func printKeysResult(result keybind.Result, out io.Writer) int {
	if result.Manual != "" {
		fmt.Fprintf(out, "herdr-utils: %s\n", result.Manual)
		return 0
	}
	switch result.Reason {
	case "no-config":
		fmt.Fprintln(out, "herdr-utils: no herdr config to bind the guide in; nothing to do")
	case "current":
		fmt.Fprintf(out, "herdr-utils: the guide is already bound on %s\n", keybind.Key)
	case "added", "updated":
		fmt.Fprintf(out, "herdr-utils: bound the guide on %s in %s; reload herdr to pick it up\n", keybind.Key, result.Wrote)
	case "removed":
		fmt.Fprintf(out, "herdr-utils: removed the fenced guide binding from %s\n", result.Wrote)
	case "absent":
		fmt.Fprintln(out, "herdr-utils: the config carries no fenced guide binding")
	}
	return 0
}

func runKeysStatus(d deps) int {
	path := keybind.Path(d.getenv)
	text, err := readFileOrEmpty(path)
	if err != nil {
		return report(err, d.err)
	}
	state := keybind.Parse(text)
	switch {
	case state.FencedKey != "":
		fmt.Fprintf(d.out, "bound on %s by the herdr-utils block in %s\n", state.FencedKey, path)
	case state.OutsideKey != "":
		fmt.Fprintf(d.out, "bound on %s by a keybinding of your own in %s\n", state.OutsideKey, path)
	default:
		fmt.Fprintf(d.out, "not bound; the action herdr-utils.which is available in %s\n", path)
	}
	return 0
}

func readFileOrEmpty(path string) (string, error) {
	text, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(text), nil
}
