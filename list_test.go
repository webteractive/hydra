package main

import (
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
