package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncWritesIndexAndBlocks(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\n---\n\n# Rust\n\nPin versions.\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}

	index := readFile(t, filepath.Join(tmp, ".hydra", "rules", "index.md"))
	if !strings.Contains(index, ".hydra/rules/rust.md") {
		t.Errorf("index missing the rule: %s", index)
	}
	for _, target := range []string{"CLAUDE.md", "AGENTS.md"} {
		got := readFile(t, filepath.Join(tmp, target))
		if !strings.Contains(got, blockStart) {
			t.Errorf("%s missing block: %s", target, got)
		}
		if !strings.Contains(got, "# App") {
			t.Errorf("%s lost its own content: %s", target, got)
		}
	}
	if !strings.Contains(out.String(), "2 target") {
		t.Errorf("output should name the target count: %s", out.String())
	}
}

func TestSyncIdempotent(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := Sync(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if n := strings.Count(got, blockStart); n != 1 {
		t.Errorf("block count = %d want 1: %s", n, got)
	}
}

func TestSyncRefusesUninitialized(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err == nil {
		t.Fatal("expected an error for an uninitialized scope")
	}
	if exists(filepath.Join(tmp, ".hydra")) {
		t.Error("sync must not create the library")
	}
}

func TestSyncWarnsWhenNoTargets(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no agent instruction files") {
		t.Errorf("expected a no-targets warning: %s", out.String())
	}
}

func TestSyncReportsMatcherlessRule(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "orphan.md"), "# Orphan\n\nno matchers\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "orphan") || !strings.Contains(out.String(), "can never fire") {
		t.Errorf("expected a matcher-less warning: %s", out.String())
	}
}
