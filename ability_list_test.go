package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbilityList(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "testing-notes", abilityFilename), validAbility("testing-notes", "Generate QA notes."))
	got, err := AbilityList(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "testing-notes" || got[0].Path == "" {
		t.Errorf("unexpected list: %+v", got)
	}
}

func TestRenderAbilityListTextIsHumanReadable(t *testing.T) {
	abilities := []AbilityInfo{{
		Name:        "testing-notes",
		Description: "Generate focused QA notes.",
		Path:        "/home/.hydra/abilities/testing-notes/ABILITY.md",
	}}

	var out bytes.Buffer
	renderAbilityListText(&out, abilities)
	got := out.String()
	for _, want := range []string{
		"Global abilities (1)",
		"testing-notes",
		"Generate focused QA notes.",
		"Invoke: $ability testing-notes",
		"Source: /home/.hydra/abilities/testing-notes/ABILITY.md",
		"For agents and scripts: hydra ability list --json",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderAbilityListTextExplainsEmptyCatalog(t *testing.T) {
	var out bytes.Buffer
	renderAbilityListText(&out, nil)
	if got := out.String(); !strings.Contains(got, "No abilities found") || !strings.Contains(got, "hydra ability new") {
		t.Errorf("empty output should explain what to do next:\n%s", got)
	}
}
