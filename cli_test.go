package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(args, &out, &out)
	return out.String(), err
}

func TestRunVersionHelp(t *testing.T) {
	out, err := runCLI(t, "version")
	if err != nil || !strings.Contains(out, "hydra "+version()) {
		t.Errorf("version: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "help"); err != nil || !strings.Contains(out, "Usage:") {
		t.Errorf("help: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "bogus"); err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRunLifecycleProject(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "doctor"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "new", "extra-skill"); err != nil {
		t.Fatal(err)
	}
	if !resolves(filepath.Join(tmp, ".claude", "skills", "extra-skill")) {
		t.Error("new skill not synced via CLI")
	}
	// --global after the subcommand still parses
	if _, err := runCLI(t, "log", "CREATE", "extra-skill", "made it"); err != nil {
		t.Fatal(err)
	}
}

func TestRunGlobalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	t.Setenv("HOME", home)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	if !resolves(filepath.Join(home, ".claude", "skills", "skill-curator")) {
		t.Error("global skill-curator not synced")
	}
	if !fileContains(filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".hydra", "curator-reminder.sh")) {
		t.Error("global hook should be wired with an absolute command path")
	}
	if _, err := runCLI(t, "doctor", "--global"); err != nil {
		t.Fatal(err)
	}
}
