package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectTargetsProject(t *testing.T) {
	tmp := t.TempDir()
	touch(t, filepath.Join(tmp, "CLAUDE.md"))

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	want := []string{filepath.Join(tmp, "CLAUDE.md")}
	if got := DetectTargets(s); !reflect.DeepEqual(got, want) {
		t.Errorf("DetectTargets = %v want %v", got, want)
	}
}

func TestDetectTargetsGlobal(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	touch(t, filepath.Join(home, ".codex", "AGENTS.md"))

	s := ResolveScope(true, tmp, home)
	want := []string{filepath.Join(home, ".codex", "AGENTS.md")}
	if got := DetectTargets(s); !reflect.DeepEqual(got, want) {
		t.Errorf("DetectTargets = %v want %v", got, want)
	}
}

func TestDetectTargetsNoneFound(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	if got := DetectTargets(s); len(got) != 0 {
		t.Errorf("DetectTargets = %v want empty", got)
	}
}

func TestDefaultTargets(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")

	proj := DefaultTargets(ResolveScope(false, tmp, home))
	wantProj := []string{filepath.Join(tmp, "CLAUDE.md"), filepath.Join(tmp, "AGENTS.md")}
	if !reflect.DeepEqual(proj, wantProj) {
		t.Errorf("project defaults = %v want %v", proj, wantProj)
	}

	glob := DefaultTargets(ResolveScope(true, tmp, home))
	wantGlob := []string{filepath.Join(home, ".claude", "CLAUDE.md")}
	if !reflect.DeepEqual(glob, wantGlob) {
		t.Errorf("global defaults = %v want %v", glob, wantGlob)
	}
}
