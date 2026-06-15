package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	if err := New(s, "my-skill", &out); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(tmp, ".hydra", "skills", "my-skill", "SKILL.md")
	if !fileContains(skill, "name: my-skill") || !fileContains(skill, "description:") {
		t.Error("scaffold missing frontmatter")
	}
	if !resolves(filepath.Join(tmp, ".claude", "skills", "my-skill")) {
		t.Error("new skill not synced")
	}

	if err := New(s, "BadName", &out); err == nil {
		t.Error("expected error for non-kebab name")
	}
	if err := New(s, "has space", &out); err == nil {
		t.Error("expected error for spaced name")
	}
	if err := New(s, "my-skill", &out); err == nil {
		t.Error("expected error for existing skill")
	}
	if err := New(s, "", &out); err == nil {
		t.Error("expected error for empty name")
	}
}
