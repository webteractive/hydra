package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetectAbilityHarnesses(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(home, ".codex", "AGENTS.md"), "# Codex\n")
	mustWrite(t, filepath.Join(home, ".gemini", "GEMINI.md"), "# Gemini\n")

	got := detectAbilityHarnesses(s)
	var names []string
	for _, harness := range got {
		names = append(names, harness.Name)
	}
	if want := []string{"codex", "gemini"}; !reflect.DeepEqual(names, want) {
		t.Errorf("harnesses = %v want %v", names, want)
	}
	if got[0].RouterPath != filepath.Join(home, ".agents", "skills", "ability", "SKILL.md") {
		t.Errorf("Codex router path = %s", got[0].RouterPath)
	}
}

func TestInitialAbilityHarnessesDetectsInstalledConfigDirectory(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	if err := os.Mkdir(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := initialAbilityHarnesses(s)
	if len(got) != 1 || got[0].Name != "codex" {
		t.Errorf("initial harnesses = %+v, want codex", got)
	}
}

func TestPreflightAbilityRoutersPreservesForeignSkill(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	harness := abilityHarnesses(s)[0]
	mustWrite(t, harness.RouterPath, "---\nname: ability\n---\n\nuser-authored\n")

	err := preflightAbilityRouters([]AbilityHarness{harness})
	if err == nil || !strings.Contains(err.Error(), "not Hydra-owned") {
		t.Fatalf("expected ownership collision, got %v", err)
	}
	if got := readFile(t, harness.RouterPath); !strings.Contains(got, "user-authored") {
		t.Error("foreign router was modified")
	}

	if err := os.WriteFile(harness.RouterPath, []byte(RenderAbilityRouter(s)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := preflightAbilityRouters([]AbilityHarness{harness}); err != nil {
		t.Errorf("owned router should pass preflight: %v", err)
	}
}
