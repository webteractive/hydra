package main

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	if err != nil || !strings.Contains(out, "Rules in project scope (1)") || !strings.Contains(out, "Files: app/Http/Controllers/**") {
		t.Errorf("list: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "list", "--json"); err != nil || !strings.Contains(out, `"name": "controllers"`) {
		t.Errorf("list --json: out=%q err=%v", out, err)
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

func TestRunAbilityLifecycle(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	t.Setenv("HOME", home)

	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "ability", "new", "testing-notes"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "ability", "sync"); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, "ability", "list"); err != nil || !strings.Contains(out, "Invoke: $ability testing-notes") {
		t.Errorf("ability list: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "ability", "list", "--json"); err != nil || !strings.Contains(out, `"description"`) {
		t.Errorf("ability list --json: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "ability", "doctor"); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, "ability", "doctor", "--json"); err != nil || !strings.Contains(out, `"global abilities"`) {
		t.Errorf("ability doctor --json: out=%q err=%v", out, err)
	}
}

// filepath.Join("", ".hydra") is ".hydra", so a swallowed UserHomeDir failure
// would point --global at the current directory and scaffold there while
// reporting success. It must fail loudly instead.
func TestRunGlobalFailsWithoutHome(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", "")

	out, err := runCLI(t, "init", "--global")
	if err == nil {
		t.Fatalf("expected an error when the home directory cannot be resolved; out=%q", out)
	}
	if exists(filepath.Join(tmp, ".hydra")) {
		t.Error("a failed --global must not scaffold into the current directory")
	}
}

// A CLI must answer "which build is this?" without a subcommand.
func TestRootReportsVersionViaFlag(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		out, err := runCLI(t, flag)
		if err != nil {
			t.Errorf("%s: %v", flag, err)
		}
		if !strings.Contains(out, version()) {
			t.Errorf("%s: output %q should contain %q", flag, out, version())
		}
	}
	// The subcommand must keep working alongside the flag.
	out, err := runCLI(t, "version")
	if err != nil || !strings.Contains(out, version()) {
		t.Errorf("version subcommand: out=%q err=%v", out, err)
	}
}

func TestVersionFlagAndSubcommandAgree(t *testing.T) {
	flagOut, _ := runCLI(t, "--version")
	subOut, _ := runCLI(t, "version")
	if strings.TrimSpace(flagOut) != strings.TrimSpace(subOut) {
		t.Errorf("version surfaces disagree:\n  --version: %q\n  version:   %q", flagOut, subOut)
	}
}

// Triggers decide invocation, so a script reading the machine-readable output
// cannot reason about an ability without them.
func TestAbilityListJSONExposesTriggers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(home, ".hydra", "abilities", "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers:\n  - ship the thing\n---\n\n# Shipper\n")
	if _, err := runCLI(t, "ability", "sync"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "ability", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var infos []AbilityInfo
	if err := json.Unmarshal([]byte(out), &infos); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(infos) != 1 || len(infos[0].Triggers) != 1 || infos[0].Triggers[0] != "ship the thing" {
		t.Errorf("triggers missing from machine-readable output: %+v", infos)
	}

	if text, err := runCLI(t, "ability", "list"); err != nil || !strings.Contains(text, "ship the thing") {
		t.Errorf("humans need the triggers too: out=%q err=%v", text, err)
	}
}

// The command listing is how anyone learns what a tool does.
func TestEveryCommandHasHelp(t *testing.T) {
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		if c.Name() != "hydra" && c.Name() != "completion" && c.Name() != "help" {
			if c.Short == "" {
				t.Errorf("%s: no Short description", path)
			}
			if c.Long == "" {
				t.Errorf("%s: no Long help — users cannot learn what it does or what its flags mean", path)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub, path+" "+sub.Name())
		}
	}
	walk(newRootCmd(io.Discard, io.Discard), "hydra")
}

func TestHelpDoesNotDescribeAbilitiesAsLazyLoaded(t *testing.T) {
	root := newRootCmd(io.Discard, io.Discard)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, text := range []string{c.Short, c.Long} {
			if strings.Contains(text, "lazy-loaded abilit") {
				t.Errorf("%s: help still describes the pre-fix model: %q", c.Name(), text)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
