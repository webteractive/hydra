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
		t.Error("expected an error for an unknown command")
	}
}

func TestRunLifecycleProject(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "add",
		"--glob", "app/Http/Controllers/**",
		"--title", "Extend BaseController",
		"--note", "Every controller extends BaseController.",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "sync"); err != nil {
		t.Fatal(err)
	}

	claude := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if !strings.Contains(claude, ".hydra/rules/controllers.md") {
		t.Errorf("CLAUDE.md missing the indexed rule:\n%s", claude)
	}

	out, err := runCLI(t, "list")
	if err != nil || !strings.Contains(out, "controllers") {
		t.Errorf("list: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "doctor"); err != nil {
		t.Fatalf("doctor should pass on a fresh install: %v", err)
	}
	if out, err := runCLI(t, "doctor", "--json"); err != nil || !strings.Contains(out, `"severity"`) {
		t.Errorf("doctor --json: out=%q err=%v", out, err)
	}
}

func TestRunAddInitializesFromScratch(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "add", "--always",
		"--title", "Never commit automatically",
		"--note", "Ask before git commit.",
	); err != nil {
		t.Fatal(err)
	}
	claude := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if !strings.Contains(claude, "Ask before git commit.") {
		t.Errorf("always-rule not inlined:\n%s", claude)
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
	target := filepath.Join(home, ".claude", "CLAUDE.md")
	got := readFile(t, target)
	if !strings.Contains(got, filepath.Join(home, ".hydra", "rules")) {
		t.Errorf("global block should reference absolute paths:\n%s", got)
	}
	if _, err := runCLI(t, "doctor", "--global"); err != nil {
		t.Fatal(err)
	}
}

func TestRunSyncUninitializedFails(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "sync"); err == nil {
		t.Error("sync on an uninitialized project should fail")
	}
}
