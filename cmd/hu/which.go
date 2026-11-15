package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/WilberC/herdr-utils/internal/config"
	"github.com/WilberC/herdr-utils/internal/which"
	"github.com/WilberC/tui-kit/theme"
)

// guideFunc is the terminal seam: production runs the Bubbletea program, a test
// records the guide it was handed.
type guideFunc func(*which.Guide, io.Reader, io.Writer) error

func runWhich(d deps, args []string) int {
	_ = theme.Use(theme.Auto(d.getenv("HU_THEME"), huThemeDir()))
	if len(args) != 0 {
		return report(usageError{reason: "which takes no arguments", usage: whichUsage}, d.err)
	}

	document, err := config.Load(d.getenv)
	if err != nil {
		return report(err, d.err)
	}
	configPath, err := config.Path(d.getenv)
	if err != nil {
		return report(err, d.err)
	}
	if !document.Config.ToolEnabled(config.ToolWhich) {
		fmt.Fprintf(d.err, "hu: the keybinding guide is off; set tools.which = true in %s\n", configPath)
		return 2
	}
	if d.in != nil && !isTerminal(d.in) {
		fmt.Fprint(d.err, "hu: hu which needs a terminal; bind the herdr-utils.which action (see docs/which.md) or run it in a pane\n")
		return 2
	}

	var invoke func(context.Context, string) error
	if d.client != nil {
		invoke = d.client.InvokeAction
	}
	built := which.Build(context.Background(), which.Options{
		Config:     document.Config,
		ConfigPath: configPath,
		Getenv:     d.getenv,
		Actions:    d.client,
		Invoke:     invoke,
		Persist:    viewSaver(d),
	})

	run := d.guide
	if run == nil {
		run = func(built *which.Guide, in io.Reader, out io.Writer) error {
			return which.New(*built).Run(in, out)
		}
	}
	return report(run(&built, d.in, d.out), d.err)
}

// viewSaver records the view the guide last drew, writing through internal/config
// so the manual entry arrays and every unknown key survive the round trip.
func viewSaver(d deps) func(string) error {
	return func(view string) error {
		document, err := config.Load(d.getenv)
		if err != nil {
			return err
		}
		path, err := config.Path(d.getenv)
		if err != nil {
			return err
		}
		document.Config.Which.View = view
		return config.Save(path, document)
	}
}

func isTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// huThemeDir is where a custom theme's JSON file lives: the platform's config
// directory, under hu/themes. The HU_THEME setting names a file in it —
// "gruvbox" loads gruvbox.json — and every failure to load one, like a
// missing file or a broken one, leaves the built-in Tokyo Night palette in
// force. With no setting the guide asks the terminal for its own colours and
// renders the theme the terminal describes.
var huThemeDir = func() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "hu", "themes")
}
