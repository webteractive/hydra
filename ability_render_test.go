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

func TestRenderAbilityBlockInlinesTheCatalog(t *testing.T) {
	s := ResolveAbilityScope("/home/user")
	abilities := []Ability{{
		Name:        "testing-notes",
		Description: "Generate QA notes.",
		Triggers:    []string{"write test notes"},
		Path:        filepath.Join(s.AbilitiesDir, "testing-notes", abilityFilename),
	}}
	got := RenderAbilityBlock(s, abilities)
	if !strings.Contains(got, filepath.ToSlash(filepath.Join(s.AbilitiesDir, abilityIndexFile))) {
		t.Errorf("block missing catalog path:\n%s", got)
	}
	for _, want := range []string{
		"$ability <name>",
		"choose semantically",
		"Before selecting any workflow",
		"lowercase kebab-case",
		"treat it as an explicit invocation",
		"available workflow also matches",
		"`testing-notes`",
		"write test notes",
		"Generate QA notes.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing discovery contract %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Abilities are optional") {
		t.Error("block must not make exact-name matches optional")
	}
}

func TestRenderAbilityBlockDelegatesAmbiguityToTheAgent(t *testing.T) {
	got := RenderAbilityBlock(ResolveAbilityScope("/home/user"), nil)
	for _, want := range []string{"candidates, not a conflict", "name the one you picked", "is NOT an invocation"} {
		if !strings.Contains(got, want) {
			t.Errorf("block must tell the agent how to resolve several matches, missing %q:\n%s", want, got)
		}
	}
}

func TestRenderAbilityBlockOmitsDerivablePaths(t *testing.T) {
	s := ResolveAbilityScope("/home/user")
	abilities := []Ability{{
		Name:        "testing-notes",
		Description: "Generate QA notes.",
		Path:        filepath.Join(s.AbilitiesDir, "testing-notes", abilityFilename),
	}}
	got := RenderAbilityBlock(s, abilities)
	if strings.Contains(got, filepath.ToSlash(abilities[0].Path)) {
		t.Errorf("block should state the path pattern once, not per row:\n%s", got)
	}
	if !strings.Contains(got, "<name>/"+abilityFilename) {
		t.Errorf("block must tell the agent how to resolve an ability path:\n%s", got)
	}
	if !strings.Contains(RenderAbilityIndex(abilities), filepath.ToSlash(abilities[0].Path)) {
		t.Error("index.md should still carry full paths")
	}
}

func TestRenderAbilityBlockKeepsBodiesLazy(t *testing.T) {
	s := ResolveAbilityScope("/home/user")
	abilities := []Ability{{
		Name:        "testing-notes",
		Description: "Generate QA notes.",
		Path:        filepath.Join(s.AbilitiesDir, "testing-notes", abilityFilename),
		Body:        "# Testing Notes\n\nSecret authored steps that must stay on disk.\n",
	}}
	if got := RenderAbilityBlock(s, abilities); strings.Contains(got, "Secret authored steps") {
		t.Errorf("block must not inline ability bodies:\n%s", got)
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

func TestRenderAbilityIndexExposesTriggers(t *testing.T) {
	abilities := []Ability{
		{
			Name:        "prepare-for-production",
			Description: "Harden a change for production.",
			Triggers:    []string{"make it production ready", "primetime!"},
			Path:        "/home/user/.hydra/abilities/prepare-for-production/ABILITY.md",
		},
		{
			Name:        "explain-code",
			Description: "Explain unfamiliar code.",
			Path:        "/home/user/.hydra/abilities/explain-code/ABILITY.md",
		},
	}
	got := RenderAbilityIndex(abilities)
	for _, want := range []string{
		"| Ability | Triggers | Description | File |",
		"make it production ready · primetime!",
		noAbilityTriggers,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("index missing %q:\n%s", want, got)
		}
	}
}

func TestRenderAbilityBlockMakesTriggerMatchingMandatory(t *testing.T) {
	got := RenderAbilityBlock(ResolveAbilityScope("/home/user"), nil)
	for _, want := range []string{
		"MUST",
		"Triggers",
		"explicit invocation",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"semantic selection is optional",
		"does not rank or automatically enforce",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("block still licenses skipping the catalog: %q\n%s", unwanted, got)
		}
	}
}
