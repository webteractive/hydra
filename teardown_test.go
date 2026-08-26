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
