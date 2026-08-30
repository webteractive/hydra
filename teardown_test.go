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

	if !exists(filepath.Join(farm, "mine")) {
		t.Error("a link to a skill that still exists must survive — removing it unexposes a working skill")
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

// The curator's own link always goes, even though its skill still exists — it
// is the machinery being removed, not content worth keeping exposed.
func TestTeardownRemovesCuratorLinkButKeepsOtherLiveSkills(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "skill-curator", "SKILL.md"), "curator\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "bugsnag-comment", "SKILL.md"), "authored\n")

	farm := filepath.Join(tmp, ".claude", "skills")
	if err := os.MkdirAll(farm, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"skill-curator", "bugsnag-comment"} {
		if err := os.Symlink("../../.hydra/skills/"+name, filepath.Join(farm, name)); err != nil {
			t.Fatal(err)
		}
	}

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if _, err := Teardown(s, &out); err != nil {
		t.Fatal(err)
	}

	if exists(filepath.Join(farm, "skill-curator")) {
		t.Error("the curator link must be removed")
	}
	if !exists(filepath.Join(farm, "bugsnag-comment")) {
		t.Error("an authored skill's link must survive")
	}
	if !exists(filepath.Join(tmp, ".hydra", "skills", "bugsnag-comment", "SKILL.md")) {
		t.Error("the authored skill itself must survive")
	}
	if !strings.Contains(out.String(), "kept") {
		t.Errorf("teardown should report what it kept: %s", out.String())
	}
}

// Doctor must not nag forever about a farm that only holds live authored links.
func TestDoctorIgnoresLiveAuthoredLinks(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "bugsnag-comment", "SKILL.md"), "authored\n")
	farm := filepath.Join(tmp, ".claude", "skills")
	if err := os.MkdirAll(farm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.hydra/skills/bugsnag-comment", filepath.Join(farm, "bugsnag-comment")); err != nil {
		t.Fatal(err)
	}

	if hasV01Artifacts(s) {
		t.Error("a live authored skill link is not v0.1 wreckage")
	}
}

func TestTeardownRemovesGeminiArtifacts(t *testing.T) {
	home := t.TempDir()
	s := ResolveScope(true, home, home)
	as := ResolveAbilityScope(home)

	gemini := filepath.Join(home, ".gemini", "GEMINI.md")
	mustWrite(t, gemini, "# My Gemini notes\n\nKeep this prose.\n\n"+
		blockStart+"\nrules block\n"+blockEnd+"\n\n"+
		abilityBlockStart+"\nabilities block\n"+abilityBlockEnd+"\n")
	router := filepath.Join(home, ".gemini", "skills", "ability", "SKILL.md")
	mustWrite(t, router, routerOwnedMarker+"\nrouter\n")

	if !hasGeminiArtifacts(s) || !hasGeminiAbilityArtifacts(as) {
		t.Fatal("expected artifacts to be detected before teardown")
	}

	var out bytes.Buffer
	if _, err := Teardown(s, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := removeGeminiAbilityArtifacts(as, &out); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(gemini)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Keep this prose.") {
		t.Errorf("authored prose must survive teardown:\n%s", body)
	}
	for _, sentinel := range []string{blockStart, abilityBlockStart} {
		if strings.Contains(string(body), sentinel) {
			t.Errorf("managed block %q should be gone:\n%s", sentinel, body)
		}
	}
	if exists(router) {
		t.Error("hydra-owned gemini router should be removed")
	}
	if hasGeminiArtifacts(s) || hasGeminiAbilityArtifacts(as) {
		t.Error("doctor should report clean after teardown")
	}
}

// Abilities are always global, so the global Gemini wiring must be cleaned even
// when init runs inside a project — the common upgrade path.
func TestAbilityInitCleansGlobalGeminiArtifactsFromProjectScope(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	s := ResolveScope(false, project, home)

	gemini := filepath.Join(home, ".gemini", "GEMINI.md")
	mustWrite(t, gemini, "# Notes\n\n"+abilityBlockStart+"\nstale\n"+abilityBlockEnd+"\n")
	router := filepath.Join(home, ".gemini", "skills", "ability", "SKILL.md")
	mustWrite(t, router, routerOwnedMarker+"\nrouter\n")

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	as := ResolveAbilityScope(home)
	if hasGeminiAbilityArtifacts(as) {
		body, _ := os.ReadFile(gemini)
		t.Errorf("project-scope init must clean the global gemini ability wiring:\n%s\nrouter exists=%v", body, exists(router))
	}
}

// A read error must not be mistaken for "already gone" — that would report a
// clean teardown while doctor keeps flagging the artifact.
func TestRemoveGeminiAbilityArtifactsReportsReadFailures(t *testing.T) {
	home := t.TempDir()
	as := ResolveAbilityScope(home)
	// A directory where the router file is expected: ReadFile fails with EISDIR.
	if err := os.MkdirAll(filepath.Join(home, ".gemini", "skills", "ability", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := removeGeminiAbilityArtifacts(as, &out); err == nil {
		t.Error("an unreadable router must surface an error, not be treated as absent")
	}
}

func TestTeardownKeepsForeignGeminiRouter(t *testing.T) {
	home := t.TempDir()
	as := ResolveAbilityScope(home)
	router := filepath.Join(home, ".gemini", "skills", "ability", "SKILL.md")
	mustWrite(t, router, "# hand-authored, not hydra's\n")

	var out bytes.Buffer
	if _, err := removeGeminiAbilityArtifacts(as, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(router) {
		t.Error("a skill hydra does not own must never be deleted")
	}
}
