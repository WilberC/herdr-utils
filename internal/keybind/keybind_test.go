package keybind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getenv(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestEnsureAddsTheBlockToADocumentWithoutOne(t *testing.T) {
	text := "keys.prefix = \"ctrl+b\"\n\n[terminal]\nscrollback_limit_bytes = 1000000\n"
	updated, result := Ensure(text)

	if !result.Changed || result.Reason != "added" {
		t.Fatalf("Ensure() = %+v, want changed added", result)
	}
	if !strings.Contains(updated, Block()) {
		t.Fatal("updated document does not carry the block")
	}
	if !strings.Contains(updated, "keys.prefix = \"ctrl+b\"") {
		t.Fatal("updated document lost the user's own lines")
	}
	state := Parse(updated)
	if state.FencedKey != Key || !state.Current {
		t.Fatalf("parse(updated) = %+v, want fenced %s current", state, Key)
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	text := "keys.prefix = \"ctrl+b\"\n" + Block() + "\n"
	updated, result := Ensure(text)
	if result.Changed || result.Reason != "current" {
		t.Fatalf("Ensure() = %+v, want unchanged current", result)
	}
	if updated != text {
		t.Fatal("Ensure rewrote an already-current document")
	}
}

func TestEnsureRepairsALegacyFence(t *testing.T) {
	legacy := strings.Replace(Block(), "key = \"prefix+i\"", "key = \"prefix+w\"", 1)
	_, result := Ensure("x = 1\n" + legacy + "\n")
	if !result.Changed || result.Reason != "updated" {
		t.Fatalf("Ensure() = %+v, want changed updated", result)
	}
}

func TestEnsureKeepsAnEditedFenceTheCommandStillMatches(t *testing.T) {
	edited := strings.Replace(Block(), "description = \"Keybinding guide\"", "description = \"my guide\"", 1)
	_, result := Ensure("x = 1\n" + edited + "\n")
	if result.Changed || result.Reason != "current" {
		t.Fatalf("Ensure() = %+v, want unchanged current", result)
	}
}

func TestEnsureReportsAnOutsideBindingAndChangesNothing(t *testing.T) {
	outside := strings.Replace(strings.Replace(Block(), BeginMarker+"\n", "", 1), "\n"+EndMarker, "", 1)
	outside = strings.Replace(outside, "key = \"prefix+i\"", "key = \"prefix+g\"", 1)
	text := outside + "\n"
	updated, result := Ensure(text)

	if result.Changed || result.Reason != "outside" {
		t.Fatalf("Ensure() = %+v, want unchanged outside", result)
	}
	if result.Manual == "" {
		t.Fatal("Ensure() returned no manual advice for the outside binding")
	}
	if updated != text {
		t.Fatal("Ensure rewrote a document it said it left alone")
	}
}

func TestEnsureMatchesTheOutsideBindingByCommand(t *testing.T) {
	outside := strings.Replace(strings.Replace(Block(), BeginMarker+"\n", "", 1), "\n"+EndMarker, "", 1)
	text := outside + "\n"
	updated, result := Ensure(text)
	if result.Changed || result.Reason != "current" {
		t.Fatalf("Ensure() = %+v, want the outside block already binding the action left alone", result)
	}
	if !strings.Contains(updated, "herdr-utils.which") || strings.Contains(updated, BeginMarker) {
		t.Fatal("outside binding was altered")
	}
}

func TestRemoveStripsTheFenceAndNothingElse(t *testing.T) {
	text := "keys.prefix = \"ctrl+b\"\n" + Block() + "\n[terminal]\nbytes = 1\n"
	updated, result := Remove(text)
	if !result.Changed || result.Reason != "removed" {
		t.Fatalf("Remove() = %+v, want changed removed", result)
	}
	if strings.Contains(updated, BeginMarker) || strings.Contains(updated, "[[keys.command]]") {
		t.Fatal("updated document still carries the binding")
	}
	if !strings.Contains(updated, "keys.prefix = \"ctrl+b\"") || !strings.Contains(updated, "[terminal]") {
		t.Fatal("updated document lost the user's own lines")
	}
}

func TestRemoveWithoutABlockChangesNothing(t *testing.T) {
	updated, result := Remove("keys.prefix = \"ctrl+b\"\n")
	if result.Changed || result.Reason != "absent" {
		t.Fatalf("Remove() = %+v, want unchanged absent", result)
	}
	if updated != "keys.prefix = \"ctrl+b\"\n" {
		t.Fatal("Remove rewrote a document without the block")
	}
}

func TestBlockShipsPrefixI(t *testing.T) {
	block := Block()
	for _, want := range []string{"key = \"prefix+i\"", "command = \"herdr-utils.which\"", "type = \"plugin_action\""} {
		if !strings.Contains(block, want) {
			t.Fatalf("block %q misses %q", block, want)
		}
	}
}

func TestPathPrefersHERDRConfigPath(t *testing.T) {
	got := Path(getenv(map[string]string{"HERDR_CONFIG_PATH": "/tmp/h/config.toml", "HOME": "/home/me"}))
	if got != "/tmp/h/config.toml" {
		t.Fatalf("Path() = %q", got)
	}
	got = Path(getenv(map[string]string{"HOME": "/home/me"}))
	if got != "/home/me/.config/herdr/config.toml" {
		t.Fatalf("Path() = %q", got)
	}
}

func TestEnsureFileWritesAndStaysQuiet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[terminal]\nbytes = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := getenv(map[string]string{"HERDR_CONFIG_PATH": path})

	result, err := EnsureFile(env)
	if err != nil || !result.Changed || result.Wrote != path {
		t.Fatalf("EnsureFile() = %+v, %v", result, err)
	}
	text, _ := os.ReadFile(path)
	if !strings.Contains(string(text), Block()) {
		t.Fatal("file does not carry the block")
	}

	again, err := EnsureFile(env)
	if err != nil || again.Changed || again.Reason != "current" {
		t.Fatalf("second EnsureFile() = %+v, %v", again, err)
	}
}

func TestEnsureFileLeavesAMissingConfigMissing(t *testing.T) {
	env := getenv(map[string]string{"HERDR_CONFIG_PATH": filepath.Join(t.TempDir(), "config.toml")})
	result, err := EnsureFile(env)
	if err != nil || result.Changed || result.Reason != "no-config" {
		t.Fatalf("EnsureFile() = %+v, %v", result, err)
	}
}

func TestRemoveFileStripsTheFence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[terminal]\nbytes = 1\n"+Block()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RemoveFile(getenv(map[string]string{"HERDR_CONFIG_PATH": path}))
	if err != nil || !result.Changed {
		t.Fatalf("RemoveFile() = %+v, %v", result, err)
	}
	text, _ := os.ReadFile(path)
	if strings.Contains(string(text), BeginMarker) {
		t.Fatal("file still carries the block")
	}
}
