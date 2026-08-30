package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validAbility(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\nextra: retained-by-author\n---\n\n# " + headline(name) + "\n\nDo the work.\n"
}

func TestParseAbility(t *testing.T) {
	path := "/tmp/testing-notes/ABILITY.md"
	ability, err := ParseAbility("testing-notes", path, validAbility("testing-notes", "Generate manual QA notes."))
	if err != nil {
		t.Fatal(err)
	}
	if ability.Name != "testing-notes" || ability.Description != "Generate manual QA notes." {
		t.Errorf("unexpected metadata: %+v", ability)
	}
	if ability.Path != path || !strings.Contains(ability.Body, "Do the work.") {
		t.Errorf("unexpected authored content: %+v", ability)
	}
}

func TestParseAbilityValidation(t *testing.T) {
	cases := map[string]string{
		"missing frontmatter":   "# Plain\n",
		"missing name":          "---\ndescription: Useful.\n---\n",
		"bad name":              "---\nname: Not-Kebab\ndescription: Useful.\n---\n",
		"directory mismatch":    "---\nname: other\ndescription: Useful.\n---\n",
		"missing description":   "---\nname: example\n---\n",
		"multiline description": "---\nname: example\ndescription: |\n  Line one.\n  Line two.\n---\n",
	}
	for label, content := range cases {
		if _, err := ParseAbility("example", "/tmp/example/ABILITY.md", content); err == nil {
			t.Errorf("%s: expected validation error", label)
		}
	}
}

func TestLoadAbilitiesSortsAndAggregatesErrors(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "zebra", abilityFilename), validAbility("zebra", "Last."))
	mustWrite(t, filepath.Join(dir, "alpha", abilityFilename), validAbility("alpha", "First."))
	mustWrite(t, filepath.Join(dir, ".git", "config"), "ignored")
	mustWrite(t, filepath.Join(dir, abilityIndexFile), "generated")

	abilities, err := LoadAbilities(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(abilities) != 2 || abilities[0].Name != "alpha" || abilities[1].Name != "zebra" {
		t.Errorf("unexpected abilities: %+v", abilities)
	}

	mustWrite(t, filepath.Join(dir, "missing-name", abilityFilename), "---\ndescription: Nope.\n---\n")
	if err := os.Mkdir(filepath.Join(dir, "missing-file"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = LoadAbilities(dir)
	if err == nil {
		t.Fatal("expected aggregate validation error")
	}
	for _, want := range []string{"missing-name", "missing-file"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregate error missing %q: %v", want, err)
		}
	}
}

func TestLoadAbilitiesMissingLibraryIsEmpty(t *testing.T) {
	abilities, err := LoadAbilities(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(abilities) != 0 {
		t.Errorf("abilities=%v err=%v, want empty nil", abilities, err)
	}
}

func TestRenderAbilityFileParses(t *testing.T) {
	content := RenderAbilityFile("release-review")
	if _, err := ParseAbility("release-review", "/tmp/release-review/ABILITY.md", content); err != nil {
		t.Fatal(err)
	}
}

func TestParseAbilityTriggers(t *testing.T) {
	content := "---\nname: example\ndescription: Useful.\ntriggers:\n  - Prepare For Production\n  - \"Primetime!\"\n  - prepare for production\n---\n\n# Example\n"
	ability, err := ParseAbility("example", "/tmp/example/ABILITY.md", content)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"prepare for production", "primetime!"}
	if len(ability.Triggers) != len(want) {
		t.Fatalf("triggers=%q, want %q", ability.Triggers, want)
	}
	for i, trigger := range want {
		if ability.Triggers[i] != trigger {
			t.Errorf("triggers=%q, want %q", ability.Triggers, want)
		}
	}
}

func TestParseAbilityTriggersAreOptional(t *testing.T) {
	ability, err := ParseAbility("example", "/tmp/example/ABILITY.md", validAbility("example", "Useful."))
	if err != nil {
		t.Fatal(err)
	}
	if len(ability.Triggers) != 0 {
		t.Errorf("expected no triggers, got %q", ability.Triggers)
	}
}

func TestParseAbilityRejectsUnusableTriggers(t *testing.T) {
	cases := map[string]string{
		"empty trigger":     "---\nname: example\ndescription: Useful.\ntriggers:\n  - \"\"\n---\n",
		"blank trigger":     "---\nname: example\ndescription: Useful.\ntriggers:\n  - \"   \"\n---\n",
		"multiline trigger": "---\nname: example\ndescription: Useful.\ntriggers:\n  - |\n    one\n    two\n---\n",
	}
	for label, content := range cases {
		if _, err := ParseAbility("example", "/tmp/example/ABILITY.md", content); err == nil {
			t.Errorf("%s: expected validation error", label)
		}
	}
}
