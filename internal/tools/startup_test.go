package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WilberC/herdr-utils/internal/config"
)

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func enabled(names ...string) config.Config {
	document := config.Default()
	for _, name := range names {
		document.Tools[name] = true
	}
	return document
}

// TestDecideGateMatrix pins S2's startup gate: nothing to start with every tool
// off, and silence once the user enabled one.
func TestDecideGateMatrix(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
		want   Plan
	}{
		{name: "every tool off", config: config.Default(), want: PlanHint},
		{name: "the which tool on", config: enabled(config.ToolWhich), want: PlanNothing},
		{name: "the plugins tool on", config: enabled(config.ToolPlugins), want: PlanNothing},
		{name: "both tools on", config: enabled(config.ToolWhich, config.ToolPlugins), want: PlanNothing},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Decide(test.config); got != test.want {
				t.Fatalf("Decide(%#v) = %q, want %q", test.config, got, test.want)
			}
		})
	}
}

func TestEnabledNamesTheToolsInCatalogOrder(t *testing.T) {
	if got := Enabled(enabled(config.ToolPlugins, config.ToolWhich)); !reflect.DeepEqual(got, []string{"which", "plugins"}) {
		t.Fatalf("Enabled = %v, want [which plugins]", got)
	}
	if got := Enabled(config.Default()); len(got) != 0 {
		t.Fatalf("Enabled(every tool off) = %v, want none", got)
	}
}

func TestHintPathFallsBackFromThePluginStateDirToXDGToHome(t *testing.T) {
	root := t.TempDir()
	override := filepath.Join(root, "state")

	path, err := HintPath(envFrom(map[string]string{"HERDR_PLUGIN_STATE_DIR": override, "XDG_STATE_HOME": filepath.Join(root, "xdg")}))
	if err != nil {
		t.Fatalf("HintPath: %v", err)
	}
	if want := filepath.Join(override, "startup-hint.json"); path != want {
		t.Fatalf("HintPath = %q, want %q", path, want)
	}

	path, err = HintPath(envFrom(map[string]string{"XDG_STATE_HOME": filepath.Join(root, "xdg")}))
	if err != nil {
		t.Fatalf("HintPath: %v", err)
	}
	want := filepath.Join(root, "xdg", "herdr", "plugins", "herdr-utils", "startup-hint.json")
	if path != want {
		t.Fatalf("HintPath = %q, want %q", path, want)
	}

	path, err = HintPath(envFrom(map[string]string{"HOME": filepath.Join(root, "home")}))
	if err != nil {
		t.Fatalf("HintPath: %v", err)
	}
	want = filepath.Join(root, "home", ".local", "state", "herdr", "plugins", "herdr-utils", "startup-hint.json")
	if path != want {
		t.Fatalf("HintPath = %q, want %q", path, want)
	}

	if _, err := HintPath(envFrom(map[string]string{})); err == nil {
		t.Fatal("HintPath with no environment = nil error, want the refusal naming the variables")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat %q = %v, want no state file until a hint is taken", path, err)
	}
}

func TestTakeHintOncePerSocket(t *testing.T) {
	root := t.TempDir()
	getenv := envFrom(map[string]string{
		"XDG_STATE_HOME":    root,
		"HERDR_SOCKET_PATH": "/tmp/herdr-one.sock",
	})
	now := time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)

	first, err := TakeHint(getenv, now)
	if err != nil {
		t.Fatalf("TakeHint: %v", err)
	}
	if !first {
		t.Fatal("first TakeHint = false, want the hint this server run")
	}

	second, err := TakeHint(getenv, now)
	if err != nil {
		t.Fatalf("second TakeHint: %v", err)
	}
	if second {
		t.Fatal("second TakeHint = true, want it silent for the same socket")
	}

	other := envFrom(map[string]string{
		"XDG_STATE_HOME":    root,
		"HERDR_SOCKET_PATH": "/tmp/herdr-two.sock",
	})
	third, err := TakeHint(other, now)
	if err != nil {
		t.Fatalf("TakeHint for another socket: %v", err)
	}
	if !third {
		t.Fatal("TakeHint for another socket = false, want the hint once per socket")
	}
}

func TestTakeHintSurvivesACorruptStateFile(t *testing.T) {
	root := t.TempDir()
	getenv := envFrom(map[string]string{"XDG_STATE_HOME": root})
	path, err := HintPath(getenv)
	if err != nil {
		t.Fatalf("HintPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := TakeHint(getenv, time.Now())
	if err != nil {
		t.Fatalf("TakeHint with a corrupt state file: %v", err)
	}
	if !first {
		t.Fatal("TakeHint = false, want a corrupt state file to behave like a fresh one")
	}
}
