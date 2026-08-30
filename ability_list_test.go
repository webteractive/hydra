package main

import (
	"path/filepath"
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
