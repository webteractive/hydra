package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderAbilityIndex(t *testing.T) {
	abilities := []Ability{{
		Name:        "testing-notes",
		Description: "Generate QA notes | with coverage.",
		Path:        "/home/user/.hydra/abilities/testing-notes/ABILITY.md",
	}}
	got := RenderAbilityIndex(abilities)
	for _, want := range []string{"exact normalized ability-name match", "`testing-notes`", `Generate QA notes \| with coverage.`, abilities[0].Path} {
		if !strings.Contains(got, want) {
			t.Errorf("index missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(RenderAbilityIndex(nil), "| Ability |") {
		t.Error("empty index should use the empty-state message")
	}
}

func TestRenderAbilityBlockKeepsCatalogExternal(t *testing.T) {
	s := ResolveAbilityScope("/home/user")
	got := RenderAbilityBlock(s)
	if !strings.Contains(got, filepath.ToSlash(filepath.Join(s.AbilitiesDir, abilityIndexFile))) {
		t.Errorf("block missing catalog path:\n%s", got)
	}
	for _, want := range []string{
		"$ability <name>",
		"choose semantically",
		"Before selecting another workflow",
		"lowercase kebab-case",
		"treat it as an explicit invocation",
		"available workflow also matches",
		"semantic selection is optional",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing discovery contract %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "testing-notes") {
		t.Error("block must not inline catalog entries")
	}
	if strings.Contains(got, "Abilities are optional") {
		t.Error("block must not make exact-name matches optional")
	}
}

func TestRenderAbilityRouter(t *testing.T) {
	s := ResolveAbilityScope("/home/user")
	got := RenderAbilityRouter(s)
	for _, want := range []string{routerOwnedMarker, "name: ability", s.AbilitiesDir, "exact ability name", "path separators"} {
		if !strings.Contains(got, want) {
			t.Errorf("router missing %q:\n%s", want, got)
		}
	}
}
