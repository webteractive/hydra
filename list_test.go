package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, home, name, desc string) {
	t.Helper()
	dir := filepath.Join(home, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListSortedWithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, tmp)

	// beta written first to confirm sorting, alpha has a colon in description.
	writeSkill(t, s.Home, "beta", "second skill")
	writeSkill(t, s.Home, "alpha", "first skill: with a colon")

	skills, err := List(s)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("want 2 skills, got %d: %+v", len(skills), skills)
	}
	if skills[0].Name != "alpha" || skills[1].Name != "beta" {
		t.Fatalf("not sorted by name: %+v", skills)
	}
	if skills[0].Description != "first skill: with a colon" {
		t.Errorf("colon description mangled: %q", skills[0].Description)
	}
	if skills[1].Description != "second skill" {
		t.Errorf("beta description wrong: %q", skills[1].Description)
	}
	wantPath := filepath.Join(s.Home, "skills", "alpha", "SKILL.md")
	if skills[0].Path != wantPath {
		t.Errorf("path: want %q got %q", wantPath, skills[0].Path)
	}
}

func TestListFallbackNoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, tmp)
	dir := filepath.Join(s.Home, "skills", "naked")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(s)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "naked" || skills[0].Description != "" {
		t.Fatalf("fallback wrong: %+v", skills)
	}
}

func TestListUninitialized(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, tmp)
	skills, err := List(s)
	if err != nil {
		t.Fatalf("uninitialized should not error: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("want empty, got %+v", skills)
	}
}

func TestRunListJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(tmp, ".hydra"), "zeta", "a zeta skill")

	out, err := runCLI(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var got []SkillInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	names := map[string]bool{}
	for _, sk := range got {
		names[sk.Name] = true
	}
	if !names["zeta"] || !names["skill-curator"] {
		t.Errorf("missing expected skills in JSON: %v", names)
	}

	textOut, err := runCLI(t, "list")
	if err != nil {
		t.Fatalf("list text: %v", err)
	}
	if !strings.Contains(textOut, "zeta") || !strings.Contains(textOut, "skill-curator") {
		t.Errorf("text output missing skills:\n%s", textOut)
	}
}

func TestRunListJSONEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	out, err := runCLI(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json empty: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty library should print []: %q", out)
	}
}
