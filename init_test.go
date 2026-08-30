package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFreshProject(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(tmp, ".hydra", "rules")) {
		t.Error("rules dir not created")
	}
	if !exists(filepath.Join(tmp, ".hydra", "rules", "index.md")) {
		t.Error("index.md not created")
	}
	abilityScope := ResolveAbilityScope(filepath.Join(tmp, "home"))
	if !exists(filepath.Join(abilityScope.AbilitiesDir, abilityIndexFile)) {
		t.Error("global abilities were not bootstrapped")
	}
	abilityHarness := defaultAbilityHarness(abilityScope)
	if !exists(abilityHarness.InstructionPath) || !exists(abilityHarness.RouterPath) {
		t.Errorf("global ability harness was not bootstrapped: %+v", abilityHarness)
	}
	for _, target := range []string{"CLAUDE.md", "AGENTS.md"} {
		p := filepath.Join(tmp, target)
		if !exists(p) {
			t.Fatalf("%s not created", target)
		}
		if !strings.Contains(readFile(t, p), blockStart) {
			t.Errorf("%s missing the block", target)
		}
	}
}

func TestInitUsesExistingTargetsOnly(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), "# App\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, "CLAUDE.md")) {
		t.Error("init created CLAUDE.md even though AGENTS.md was detected")
	}
	if !strings.Contains(readFile(t, filepath.Join(tmp, "AGENTS.md")), blockStart) {
		t.Error("AGENTS.md missing the block")
	}
}

func TestInitIdempotent(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := Init(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if n := strings.Count(got, blockStart); n != 1 {
		t.Errorf("block count = %d want 1", n)
	}
}

func TestInitPreservesExistingRules(t *testing.T) {
	tmp := t.TempDir()
	rule := filepath.Join(tmp, ".hydra", "rules", "keep.md")
	mustWrite(t, rule, "---\npaths: [\"a/**\"]\n---\n\n# Keep\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, rule), "# Keep") {
		t.Error("existing rule was clobbered")
	}
}

func TestInitRunsTeardown(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator-reminder.sh"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"),
		"# App\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, ".hydra", "curator-reminder.sh")) {
		t.Error("hook script survived init")
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if strings.Contains(got, "hydra:curator") {
		t.Error("curator block survived init")
	}
	if !strings.Contains(got, blockStart) {
		t.Error("rules block not written")
	}
}

func TestInitGlobalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	s := ResolveScope(true, tmp, home)

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".claude", "CLAUDE.md")
	if !exists(target) {
		t.Fatal("global CLAUDE.md not created")
	}
	got := readFile(t, target)
	if !strings.Contains(got, filepath.Join(home, ".hydra", "rules")) {
		t.Errorf("global block should reference an absolute rules dir: %s", got)
	}
}
