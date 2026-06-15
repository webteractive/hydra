package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	s := ResolveScope(false, tmp, home)

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(tmp, ".hydra", "skills", "skill-curator", "SKILL.md"),
		filepath.Join(tmp, ".hydra", "curator-reminder.sh"),
		filepath.Join(tmp, ".hydra", "config"),
		filepath.Join(tmp, ".hydra", "curator.log"),
	} {
		if !exists(p) {
			t.Errorf("missing %s", p)
		}
	}
	if fi, _ := os.Stat(filepath.Join(tmp, ".hydra", "curator-reminder.sh")); fi == nil || fi.Mode()&0o100 == 0 {
		t.Error("hook not executable")
	}
	if !resolves(filepath.Join(tmp, ".claude", "skills", "skill-curator")) {
		t.Error("skill-curator not symlinked")
	}
	if !fileContains(filepath.Join(tmp, "CLAUDE.md"), "hydra:curator:start") {
		t.Error("CLAUDE.md missing block")
	}
	if !fileContains(filepath.Join(tmp, "AGENTS.md"), "hydra:curator:start") {
		t.Error("AGENTS.md missing block")
	}
	if !fileContains(filepath.Join(tmp, ".claude", "settings.json"), "curator-reminder.sh") {
		t.Error("settings.json not wired")
	}

	// idempotent: re-run, no dup block / hook
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if n := strings.Count(string(md), "hydra:curator:start"); n != 1 {
		t.Errorf("curator block count = %d", n)
	}
	settings, _ := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if n := strings.Count(string(settings), "curator-reminder.sh"); n != 1 {
		t.Errorf("hook count = %d", n)
	}
}

func TestInitMergesExistingSettings(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"), `{"permissions":{"allow":["Bash"]}}`)
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	b, _ := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("settings.json not valid JSON after merge: %v", err)
	}
	if _, ok := data["permissions"]; !ok {
		t.Error("permissions clobbered")
	}
	if _, ok := data["hooks"]; !ok {
		t.Error("hooks not added")
	}
}

func TestInitInvalidSettingsDoesNotAbort(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"), `{not valid json`)
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatalf("init aborted on invalid settings.json: %v", err)
	}
	if !strings.Contains(out.String(), "not valid JSON") {
		t.Errorf("expected manual-merge notice, got: %s", out.String())
	}
	// the rest of init still happened
	if !exists(filepath.Join(tmp, ".hydra", "skills", "skill-curator", "SKILL.md")) {
		t.Error("init did not complete after invalid settings.json")
	}
}
