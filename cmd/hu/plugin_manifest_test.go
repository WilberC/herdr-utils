package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type manifestAction struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Contexts    []string `json:"contexts"`
	Command     []string `json:"command"`
	Platforms   []string `json:"platforms"`
}

type manifestCommand struct {
	Command []string `json:"command"`
}

type pluginManifest struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	MinHerdrVersion string            `json:"min_herdr_version"`
	Description     string            `json:"description"`
	Platforms       []string          `json:"platforms"`
	Actions         []manifestAction  `json:"actions"`
	Startup         []manifestCommand `json:"startup"`
	Build           []manifestCommand `json:"build"`
}

// manifestPath is the plugin manifest this repository ships.
func manifestPath() string {
	return filepath.Join("..", "..", "herdr-plugin.toml")
}

func readManifestBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(manifestPath())
	if err != nil {
		t.Fatalf("read the plugin manifest: %v", err)
	}
	return data
}

// parseManifest reads the manifest with the TOML parser the ecosystem already
// uses, so the test agrees with what herdr reads instead of reimplementing TOML.
func parseManifest(t *testing.T, data []byte) pluginManifest {
	t.Helper()
	interpreter, err := exec.LookPath("python3")
	if err != nil {
		interpreter, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("reading the plugin manifest needs a python with tomllib (3.11+)")
	}

	script := "import json,sys,tomllib;json.dump(tomllib.loads(sys.stdin.read()),sys.stdout)"
	cmd := exec.Command(interpreter, "-c", script)
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("tomllib refused the plugin manifest: %v\n%s", err, stderr.String())
	}

	var document pluginManifest
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("decode the parsed manifest: %v\n%s", err, stdout.String())
	}
	return document
}

func TestPluginManifestParses(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	if document.ID != "herdr-utils" {
		t.Fatalf("manifest id = %q, want %q", document.ID, "herdr-utils")
	}
	if document.Version != "0.1.0" {
		t.Fatalf("manifest version = %q, want %q", document.Version, "0.1.0")
	}
	if document.MinHerdrVersion != "0.7.5" {
		t.Fatalf("min_herdr_version = %q, want %q", document.MinHerdrVersion, "0.7.5")
	}
	if document.Name == "" || document.Description == "" {
		t.Fatalf("manifest = %#v, want a name and a description", document)
	}
	for _, want := range []string{"linux", "macos"} {
		if !strings.Contains(strings.Join(document.Platforms, ","), want) {
			t.Fatalf("manifest platforms = %v, want %q", document.Platforms, want)
		}
	}
}

// TestPluginManifestRequiresSnapshotCapableHerdr pins the verified finding behind
// min_herdr_version: "herdr api snapshot", the session read this plugin's client
// wraps, first shipped in herdr 0.7.2 (src/cli/api.rs gained the snapshot
// subcommand there; 0.7.1 has no api dispatch), and plugin actions arrived in
// 0.7.0. 0.7.5 covers both, so the declared minimum stays where herdr-suite put
// it.
func TestPluginManifestRequiresSnapshotCapableHerdr(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	if compareVersions(document.MinHerdrVersion, "0.7.2") < 0 {
		t.Fatalf("min_herdr_version = %q, want at least 0.7.2, the release that added 'herdr api snapshot'",
			document.MinHerdrVersion)
	}
}

// TestPluginDeclaresTheWhichActionAsTheOnlySurface pins S7: the manifest declares
// the which action so the user binds a key, and nothing else is user-facing in
// v0.1.0.
func TestPluginDeclaresTheWhichActionAsTheOnlySurface(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	if len(document.Actions) != 1 {
		t.Fatalf("manifest actions = %#v, want exactly the which action", document.Actions)
	}
	action := document.Actions[0]
	if action.ID != "which" {
		t.Fatalf("action id = %q, want %q", action.ID, "which")
	}
	if action.Title != "Open the keybinding guide" {
		t.Fatalf("action which title = %q, want %q", action.Title, "Open the keybinding guide")
	}
	if action.Description == "" {
		t.Fatal("action which declares no description")
	}
	if len(action.Contexts) != 1 || action.Contexts[0] != "global" {
		t.Fatalf("action which contexts = %v, want [global]", action.Contexts)
	}
	want := []string{"bin/hu", "which"}
	if strings.Join(action.Command, " ") != strings.Join(want, " ") {
		t.Fatalf("action which command = %v, want %v", action.Command, want)
	}
}

// TestPluginDeclaresOneStartupGate pins S2's single gate: one startup hook, and
// no events, so the container has exactly one place where it can speak.
func TestPluginDeclaresOneStartupGate(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	if len(document.Startup) != 1 {
		t.Fatalf("manifest startup hooks = %#v, want exactly one", document.Startup)
	}
	want := []string{"bin/hu", "startup"}
	if strings.Join(document.Startup[0].Command, " ") != strings.Join(want, " ") {
		t.Fatalf("startup command = %v, want %v", document.Startup[0].Command, want)
	}
}

// TestPluginBuildHookProvisionsTheBinary pins the [[build]] hook herdr runs on
// every install and update against a checkout that never carries the gitignored
// binary.
func TestPluginBuildHookProvisionsTheBinary(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	if len(document.Build) != 1 {
		t.Fatalf("manifest build hooks = %#v, want exactly one", document.Build)
	}
	want := []string{"bash", "bin/ensure-binary.sh", "--in-tree"}
	if strings.Join(document.Build[0].Command, " ") != strings.Join(want, " ") {
		t.Fatalf("build command = %v, want %v", document.Build[0].Command, want)
	}
}

// TestPluginCommandsNameSubcommandsTheCLIHolds refuses a manifest that runs a
// command or flag this binary does not dispatch.
func TestPluginCommandsNameSubcommandsTheCLIHolds(t *testing.T) {
	document := parseManifest(t, readManifestBytes(t))

	commands := make([][]string, 0, len(document.Actions)+len(document.Startup)+len(document.Build))
	for _, action := range document.Actions {
		commands = append(commands, action.Command)
	}
	for _, startup := range document.Startup {
		commands = append(commands, startup.Command)
	}
	for _, build := range document.Build {
		commands = append(commands, build.Command)
	}

	for _, command := range commands {
		switch command[0] {
		case "bin/hu":
			checkHuCommand(t, command)
		case "bash":
			if len(command) < 2 {
				t.Fatalf("command %v names no script", command)
			}
			if _, err := os.Stat(filepath.Join("..", "..", command[1])); err != nil {
				t.Fatalf("command %v names a missing script: %v", command, err)
			}
		default:
			t.Fatalf("command %v starts neither bin/hu nor bash", command)
		}
	}
}

// checkHuCommand asserts a command invokes a command the CLI dispatches, and
// only flags the usage text of that command documents.
func checkHuCommand(t *testing.T, command []string) {
	t.Helper()
	if len(command) < 2 {
		t.Fatalf("command %v names no command", command)
	}
	cmd, ok := lookup(command[1])
	if !ok {
		t.Fatalf("command %v names the unknown command %q", command, command[1])
	}
	for _, token := range command[2:] {
		if !strings.HasPrefix(token, "-") {
			continue
		}
		if !strings.Contains(cmd.usage, token) {
			t.Fatalf("command %v passes %q, which %q does not document", command, token, cmd.name)
		}
	}
}

func compareVersions(left, right string) int {
	parse := func(version string) []int {
		parts := strings.Split(version, ".")
		numbers := make([]int, 0, len(parts))
		for _, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil {
				return nil
			}
			numbers = append(numbers, number)
		}
		return numbers
	}
	leftParts, rightParts := parse(left), parse(right)
	if leftParts == nil || rightParts == nil {
		return -1
	}
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		got, want := 0, 0
		if index < len(leftParts) {
			got = leftParts[index]
		}
		if index < len(rightParts) {
			want = rightParts[index]
		}
		if got != want {
			if got < want {
				return -1
			}
			return 1
		}
	}
	return 0
}
