package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSync(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, ".hydra", "skills", "foo"))
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "foo", "SKILL.md"), "x")
	mustWrite(t, filepath.Join(tmp, ".hydra", "config"), `HYDRA_RUNTIMES="claude agents"`)
	s := ResolveScope(false, tmp, tmp)

	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !resolves(filepath.Join(tmp, ".claude", "skills", "foo")) {
		t.Error("claude link does not resolve")
	}
	if !resolves(filepath.Join(tmp, ".agents", "skills", "foo")) {
		t.Error("agents link does not resolve")
	}
	link, _ := os.Readlink(filepath.Join(tmp, ".claude", "skills", "foo"))
	if link != "../../.hydra/skills/foo" {
		t.Errorf("link target = %s", link)
	}

	// idempotent
	out.Reset()
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0 link(s)") {
		t.Errorf("second sync not idempotent: %s", out.String())
	}

	// prune dangling
	os.RemoveAll(filepath.Join(tmp, ".hydra", "skills", "foo"))
	out.Reset()
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, ".claude", "skills", "foo")) {
		t.Error("dangling link not pruned")
	}

	// collision: a non-hydra symlink is warned + left alone
	mustMkdir(t, filepath.Join(tmp, ".hydra", "skills", "bar"))
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "bar", "SKILL.md"), "y")
	mustMkdir(t, filepath.Join(tmp, ".claude", "skills"))
	os.Symlink(os.TempDir(), filepath.Join(tmp, ".claude", "skills", "bar"))
	out.Reset()
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out.String()), "collision") {
		t.Errorf("expected collision warning: %s", out.String())
	}
	got, _ := os.Readlink(filepath.Join(tmp, ".claude", "skills", "bar"))
	if got != os.TempDir() {
		t.Errorf("collision link changed to %s", got)
	}
}

// test helpers (shared across tests in package main)
func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}
func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
