package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilberC/herdr-utils/internal/keybind"
)

func herdrConfigPath(t *testing.T, r *testRun) string {
	t.Helper()
	return filepath.Join(r.root, "herdr", "config.toml")
}

func seedHerdrConfig(t *testing.T, r *testRun, body string) string {
	t.Helper()
	path := herdrConfigPath(t, r)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestKeysInstallAddsTheBlock(t *testing.T) {
	run := newTestRun(t)
	path := seedHerdrConfig(t, run, "[terminal]\nscrollback_limit_bytes = 1\n")
	run.env["HERDR_CONFIG_PATH"] = path

	if code := run.run(t, "keys", "install"); code != 0 {
		t.Fatalf("keys install exit = %d\nstderr:\n%s", code, run.stderr.String())
	}
	if !strings.Contains(run.stdout.String(), "bound the guide on "+keybind.Key) {
		t.Fatalf("keys install stdout = %q", run.stdout.String())
	}
	text, _ := os.ReadFile(path)
	if !strings.Contains(string(text), keybind.Block()) {
		t.Fatal("config does not carry the fenced block")
	}
}

func TestKeysInstallIsIdempotent(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n")

	if code := run.run(t, "keys", "install"); code != 0 {
		t.Fatalf("first install exit = %d", code)
	}
	run.stdout.Reset()
	if code := run.run(t, "keys", "install"); code != 0 {
		t.Fatalf("second install exit = %d", code)
	}
	if !strings.Contains(run.stdout.String(), "already bound") {
		t.Fatalf("second install stdout = %q", run.stdout.String())
	}
}

func TestKeysInstallReportsAnOutsideBinding(t *testing.T) {
	run := newTestRun(t)
	outside := strings.Replace(strings.Replace(keybind.Block(), keybind.BeginMarker+"\n", "", 1), "\n"+keybind.EndMarker, "", 1)
	outside = strings.Replace(outside, "key = \""+keybind.Key+"\"", "key = \"prefix+g\"", 1)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, outside+"\n")

	if code := run.run(t, "keys", "install"); code != 0 {
		t.Fatalf("keys install exit = %d", code)
	}
	if !strings.Contains(run.stdout.String(), "outside the herdr-utils fence") {
		t.Fatalf("keys install stdout = %q", run.stdout.String())
	}
}

func TestKeysRemoveStripsOnlyTheFence(t *testing.T) {
	run := newTestRun(t)
	path := seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n"+keybind.Block()+"\n")
	run.env["HERDR_CONFIG_PATH"] = path

	if code := run.run(t, "keys", "remove"); code != 0 {
		t.Fatalf("keys remove exit = %d", code)
	}
	text, _ := os.ReadFile(path)
	updated := string(text)
	if strings.Contains(updated, "[[keys.command]]") || strings.Contains(updated, "herdr-utils.which") {
		t.Fatal("config still carries the binding")
	}
	if !strings.Contains(updated, "[terminal]") {
		t.Fatal("config lost the user's own lines")
	}
}

func TestKeysStatusReportsTheSource(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n"+keybind.Block()+"\n")

	if code := run.run(t, "keys", "status"); code != 0 {
		t.Fatalf("keys status exit = %d", code)
	}
	if !strings.Contains(run.stdout.String(), "by the herdr-utils block") {
		t.Fatalf("keys status stdout = %q", run.stdout.String())
	}
}

func TestKeysStatusWithoutABinding(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n")

	if code := run.run(t, "keys", "status"); code != 0 {
		t.Fatalf("keys status exit = %d", code)
	}
	if !strings.Contains(run.stdout.String(), "not bound") {
		t.Fatalf("keys status stdout = %q", run.stdout.String())
	}
}

func TestStartupInstallsTheBindingBeforeTheGate(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n")

	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("startup exit = %d\nstderr:\n%s", code, run.stderr.String())
	}
	text, _ := os.ReadFile(herdrConfigPath(t, run))
	if !strings.Contains(string(text), keybind.Block()) {
		t.Fatal("startup did not install the binding")
	}
}

func TestStartupReinstallsAfterAConfigRewrite(t *testing.T) {
	run := newTestRun(t)
	run.env["HERDR_CONFIG_PATH"] = seedHerdrConfig(t, run, "[terminal]\nbytes = 1\n")

	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("first startup exit = %d", code)
	}
	if err := os.WriteFile(herdrConfigPath(t, run), []byte("[terminal]\nbytes = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run.stdout.Reset()
	if code := run.run(t, "startup"); code != 0 {
		t.Fatalf("second startup exit = %d", code)
	}
	if !strings.Contains(run.stdout.String(), "bound the guide on "+keybind.Key) {
		t.Fatalf("second startup stdout = %q", run.stdout.String())
	}
}
