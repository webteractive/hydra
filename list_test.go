package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestListReturnsRules(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "secrets.md"),
		"---\nalways: true\n---\n\n# Secrets\n\nNever read.\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\ncommands: [\"cargo add\"]\n---\n\n# Rust\n")

	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d want 2", len(got))
	}
	if got[0].Name != "rust" || got[1].Name != "secrets" {
		t.Errorf("order = %s, %s", got[0].Name, got[1].Name)
	}
	if !got[1].Always {
		t.Error("secrets should be always")
	}
	if got[0].Path != ".hydra/rules/rust.md" {
		t.Errorf("Path = %s want a scope-relative ref", got[0].Path)
	}
}

func TestListUninitialized(t *testing.T) {
	tmp := t.TempDir()
	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatalf("uninitialized scope should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d want 0", len(got))
	}
}

func TestListJSONShape(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\n---\n\n# Rust\n")
	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name":"rust"`, `"always":false`, `"paths":["**/Cargo.toml"]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON missing %s: %s", want, b)
		}
	}
}

func TestRenderRuleListTextLabelsEveryMatcher(t *testing.T) {
	s := ResolveScope(false, "/work/project", "/work/home")
	rules := []RuleInfo{{
		Name:     "rust",
		Title:    "Rust dependencies",
		Path:     ".hydra/rules/rust.md",
		Paths:    []string{"**/Cargo.toml"},
		Commands: []string{"cargo add"},
		Triggers: []string{"auditing a dependency"},
	}}

	var out bytes.Buffer
	renderRuleListText(&out, s, rules)
	got := out.String()
	for _, want := range []string{
		"Rules in project scope (1)",
		"Rust dependencies (rust)",
		"Files: **/Cargo.toml",
		"Commands: cargo add",
		"When: auditing a dependency",
		"Source: .hydra/rules/rust.md",
		"For agents and scripts: hydra list --json",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderRuleListTextExplainsEmptyScope(t *testing.T) {
	var out bytes.Buffer
	renderRuleListText(&out, ResolveScope(false, "/work/project", "/work/home"), nil)
	if got := out.String(); !strings.Contains(got, "No rules found") || !strings.Contains(got, "hydra add") {
		t.Errorf("empty output should explain what to do next:\n%s", got)
	}
}

func TestRenderRuleListTextPreservesGlobalScopeInJSONHint(t *testing.T) {
	var out bytes.Buffer
	renderRuleListText(&out, ResolveScope(true, "/work/project", "/work/home"), nil)
	if got := out.String(); !strings.Contains(got, "hydra list --global --json") {
		t.Errorf("global output should preserve scope in the JSON hint:\n%s", got)
	}
}
