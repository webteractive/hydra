package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveHookPreservesOtherSettings(t *testing.T) {
	raw := `{
	  "model": "opus",
	  "hooks": {
	    "UserPromptSubmit": [
	      {"hooks": [{"type": "command", "command": "$CLAUDE_PROJECT_DIR/.hydra/curator-reminder.sh"}]},
	      {"hooks": [{"type": "command", "command": "other.sh"}]}
	    ],
	    "Stop": [{"hooks": [{"type": "command", "command": "keep.sh"}]}]
	  }
	}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	if !removeHook(data, "curator-reminder.sh") {
		t.Fatal("removeHook = false, want true")
	}
	out, _ := json.Marshal(data)
	s := string(out)
	if strings.Contains(s, "curator-reminder") {
		t.Errorf("curator hook survived: %s", s)
	}
	for _, keep := range []string{"other.sh", "keep.sh", "opus"} {
		if !strings.Contains(s, keep) {
			t.Errorf("%s was removed: %s", keep, s)
		}
	}
}

func TestRemoveHookDropsEmptyKeys(t *testing.T) {
	var data map[string]any
	raw := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"x/curator-reminder.sh"}]}]}}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	removeHook(data, "curator-reminder.sh")
	if _, present := data["hooks"]; present {
		t.Errorf("empty hooks key should be dropped: %+v", data)
	}
}

func TestTeardownRemovesEverythingButSkills(t *testing.T) {
	tmp := t.TempDir()

	// v0.1 wreckage
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator-reminder.sh"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator.log"), "log\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "config"), "HYDRA_RUNTIMES=\"claude\"\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "keep-me", "SKILL.md"), "authored\n")
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"),
		`{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":".hydra/curator-reminder.sh"}]}]}}`)
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"),
		"# App\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n")

	// a dangling symlink, exactly the state this repo is in
	if err := os.MkdirAll(filepath.Join(tmp, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../skills/gone", filepath.Join(tmp, ".claude", "skills", "gone")); err != nil {
		t.Fatal(err)
	}

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	found, err := Teardown(s, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("found = false, want true")
	}

	for _, gone := range []string{
		filepath.Join(tmp, ".hydra", "curator-reminder.sh"),
		filepath.Join(tmp, ".hydra", "curator.log"),
		filepath.Join(tmp, ".hydra", "config"),
		filepath.Join(tmp, ".claude", "skills"),
	} {
		if exists(gone) {
			t.Errorf("%s should have been removed", gone)
		}
	}
	if !exists(filepath.Join(tmp, ".hydra", "skills", "keep-me", "SKILL.md")) {
		t.Error("authored skills must survive teardown")
	}
	if strings.Contains(readFile(t, filepath.Join(tmp, "CLAUDE.md")), "curator") {
		t.Error("curator block survived")
	}
	if strings.Contains(readFile(t, filepath.Join(tmp, ".claude", "settings.json")), "curator-reminder") {
		t.Error("hook survived")
	}
	if !strings.Contains(out.String(), "kept") {
		t.Errorf("teardown should say what it kept: %s", out.String())
	}
}

func TestTeardownNoopOnCleanScope(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	found, err := Teardown(s, &out)
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}

// A shared skills directory — the shape of ~/.claude/skills, which holds links
// from skillset and dotfiles alongside hydra's. Teardown must remove only the
// links pointing into hydra's own library.
func TestTeardownOnlyRemovesItsOwnSymlinks(t *testing.T) {
	tmp := t.TempDir()
	foreign := filepath.Join(tmp, "elsewhere", "skills")
	mustWrite(t, filepath.Join(foreign, "other-tool", "SKILL.md"), "not ours\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "mine", "SKILL.md"), "ours\n")

	farm := filepath.Join(tmp, ".claude", "skills")
	if err := os.MkdirAll(farm, 0o755); err != nil {
		t.Fatal(err)
	}
	// ours, relative — exactly what v0.1 sync wrote
	if err := os.Symlink("../../.hydra/skills/mine", filepath.Join(farm, "mine")); err != nil {
		t.Fatal(err)
	}
	// ours, dangling — the stale-farm case
	if err := os.Symlink("../../.hydra/skills/gone", filepath.Join(farm, "gone")); err != nil {
		t.Fatal(err)
	}
	// another tool's, absolute
	if err := os.Symlink(filepath.Join(foreign, "other-tool"), filepath.Join(farm, "other-tool")); err != nil {
		t.Fatal(err)
	}
	// a real directory someone dropped in by hand
	mustWrite(t, filepath.Join(farm, "handwritten", "SKILL.md"), "by hand\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if _, err := Teardown(s, &out); err != nil {
		t.Fatal(err)
	}

	if exists(filepath.Join(farm, "mine")) {
		t.Error("hydra's own link should have been removed")
	}
	if exists(filepath.Join(farm, "gone")) {
		t.Error("hydra's dangling link should have been removed")
	}
	if !exists(filepath.Join(farm, "other-tool")) {
		t.Error("another tool's symlink must survive")
	}
	if !exists(filepath.Join(foreign, "other-tool", "SKILL.md")) {
		t.Error("another tool's actual skill must survive")
	}
	if !exists(filepath.Join(farm, "handwritten", "SKILL.md")) {
		t.Error("a hand-written skill directory must survive")
	}
	if !isDir(farm) {
		t.Error("a farm still holding other content must not be removed")
	}
}

func TestDoctorIgnoresForeignSymlinkFarm(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	farm := filepath.Join(tmp, ".claude", "skills")
	if err := os.MkdirAll(farm, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(tmp, "elsewhere", "some-skill")
	mustWrite(t, filepath.Join(other, "SKILL.md"), "not ours\n")
	if err := os.Symlink(other, filepath.Join(farm, "some-skill")); err != nil {
		t.Fatal(err)
	}

	if hasV01Artifacts(s) {
		t.Error("a skills dir holding only other tools' links is not v0.1 wreckage")
	}
}
