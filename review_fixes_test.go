package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPreservesForeignSymlinks(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, ".hydra", "skills", "foo"))
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "foo", "SKILL.md"), "x")
	s := ResolveScope(false, tmp, tmp)
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	// a foreign DANGLING symlink another tool placed in .claude/skills
	foreign := filepath.Join(tmp, ".claude", "skills", "other-tool")
	if err := os.Symlink("../../somewhere/does-not-exist", foreign); err != nil {
		t.Fatal(err)
	}
	// make hydra's own foo link dangling by removing its source
	os.RemoveAll(filepath.Join(tmp, ".hydra", "skills", "foo"))
	out.Reset()
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, ".claude", "skills", "foo")) {
		t.Error("hydra's own dangling link should have been pruned")
	}
	if _, err := os.Lstat(foreign); err != nil {
		t.Error("foreign dangling symlink was wrongly pruned")
	}
}

func TestInitWrongTypedHooksNotClobbered(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"), `{"hooks":"oops"}`)
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if !strings.Contains(string(b), "oops") {
		t.Error("wrong-typed hooks value was clobbered")
	}
	if !exists(filepath.Join(tmp, ".hydra", "skills", "skill-curator", "SKILL.md")) {
		t.Error("init did not complete after unexpected settings shape")
	}
}

func TestInitWrongTypedUPSNotClobbered(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"), `{"hooks":{"UserPromptSubmit":{"not":"an array"}}}`)
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if !strings.Contains(string(b), "not") {
		t.Error("wrong-typed UserPromptSubmit was clobbered")
	}
}

func TestInitPreservesSettingsMode(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, ".claude", "settings.json")
	mustWrite(t, p, `{"permissions":{"allow":["Bash"]}}`)
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("settings.json mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestInitSeparatesCuratorBlock(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# Existing\nstuff\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if !strings.Contains(string(b), "stuff\n\n<!-- hydra:curator:start") {
		t.Errorf("curator block not separated by a blank line:\n%s", b)
	}
}
