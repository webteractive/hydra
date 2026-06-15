package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestResolveScopeProject(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	if s.Home != "/work/app/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.Settings != "/work/app/.claude/settings.json" {
		t.Errorf("Settings = %s", s.Settings)
	}
	want := []string{"/work/app/CLAUDE.md", "/work/app/AGENTS.md"}
	if !reflect.DeepEqual(s.ClaudeMDs, want) {
		t.Errorf("ClaudeMDs = %v", s.ClaudeMDs)
	}
	if s.HookCmd != "$CLAUDE_PROJECT_DIR/.hydra/curator-reminder.sh" {
		t.Errorf("HookCmd = %s", s.HookCmd)
	}
	if s.Label != "project" {
		t.Errorf("Label = %s", s.Label)
	}
}

func TestResolveScopeGlobal(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	if s.Home != "/home/u/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.Settings != "/home/u/.claude/settings.json" {
		t.Errorf("Settings = %s", s.Settings)
	}
	if s.HookCmd != "/home/u/.hydra/curator-reminder.sh" {
		t.Errorf("HookCmd = %s", s.HookCmd) // absolute in global scope
	}
}

func TestRuntimeTargets(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	got := s.RuntimeTargets([]string{"claude", "agents"})
	sort.Strings(got)
	want := []string{filepath.Join("/work/app/.agents", "skills"), filepath.Join("/work/app/.claude", "skills")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("targets = %v want %v", got, want)
	}
}
