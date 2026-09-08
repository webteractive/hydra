package main

import "testing"

func TestResolveScopeProject(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	if s.Home != "/work/app/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.RulesDir != "/work/app/.hydra/rules" {
		t.Errorf("RulesDir = %s", s.RulesDir)
	}
	if s.UserHome != "/home/u" {
		t.Errorf("UserHome = %s", s.UserHome)
	}
	if s.Label != "project" {
		t.Errorf("Label = %s", s.Label)
	}
	if s.Global {
		t.Error("Global = true for a project scope")
	}
}

func TestResolveScopeGlobal(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	if s.Home != "/home/u/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.RulesDir != "/home/u/.hydra/rules" {
		t.Errorf("RulesDir = %s", s.RulesDir)
	}
	if s.Label != "global" {
		t.Errorf("Label = %s", s.Label)
	}
}

func TestRuleRefRelativeInProject(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	r := Rule{Name: "rust", Path: "/work/app/.hydra/rules/rust.md"}
	if got := s.RuleRef(r); got != ".hydra/rules/rust.md" {
		t.Errorf("RuleRef = %s want .hydra/rules/rust.md", got)
	}
	if got := s.RulesDirRef(); got != ".hydra/rules" {
		t.Errorf("RulesDirRef = %s want .hydra/rules", got)
	}
}

func TestRuleRefAbsoluteInGlobal(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	r := Rule{Name: "rust", Path: "/home/u/.hydra/rules/rust.md"}
	if got := s.RuleRef(r); got != "/home/u/.hydra/rules/rust.md" {
		t.Errorf("RuleRef = %s", got)
	}
	if got := s.RulesDirRef(); got != "/home/u/.hydra/rules" {
		t.Errorf("RulesDirRef = %s", got)
	}
}

func TestResolveScopeInMovesTheGlobalLibrary(t *testing.T) {
	s := ResolveScopeIn(true, "", "/home/u", "/srv/library")
	if s.Home != "/srv/library" {
		t.Errorf("Home = %s want /srv/library", s.Home)
	}
	if s.RulesDir != "/srv/library/rules" {
		t.Errorf("RulesDir = %s want /srv/library/rules", s.RulesDir)
	}
}

// The library moves; the agent instruction files do not. Base still points at
// the real home directory, because that is where ~/.claude/CLAUDE.md lives and
// detect.go finds it through Base.
func TestResolveScopeInLeavesInstructionFilesInTheHomeDirectory(t *testing.T) {
	s := ResolveScopeIn(true, "", "/home/u", "/srv/library")
	if s.Base != "/home/u" {
		t.Errorf("Base = %s want /home/u", s.Base)
	}
	if s.UserHome != "/home/u" {
		t.Errorf("UserHome = %s want /home/u", s.UserHome)
	}
	got := DefaultTargets(s)
	if len(got) != 1 || got[0] != "/home/u/.claude/CLAUDE.md" {
		t.Errorf("DefaultTargets = %v want [/home/u/.claude/CLAUDE.md]", got)
	}
}

// A project rule travels with its repository, so an override that moved
// .hydra out of the project would defeat the point of the scope.
func TestResolveScopeInIgnoresTheOverrideForProjectScope(t *testing.T) {
	s := ResolveScopeIn(false, "/work/app", "/home/u", "/srv/library")
	if s.Home != "/work/app/.hydra" {
		t.Errorf("Home = %s want /work/app/.hydra", s.Home)
	}
	if s.RulesDir != "/work/app/.hydra/rules" {
		t.Errorf("RulesDir = %s want /work/app/.hydra/rules", s.RulesDir)
	}
}

// Abilities are always global, so a project scope still has to carry the
// global home for abilityScopeFromRuleScope to find.
func TestResolveScopeInRecordsTheGlobalHomeInEitherScope(t *testing.T) {
	for _, global := range []bool{true, false} {
		s := ResolveScopeIn(global, "/work/app", "/home/u", "/srv/library")
		if s.GlobalHome != "/srv/library" {
			t.Errorf("global=%v: GlobalHome = %s want /srv/library", global, s.GlobalHome)
		}
	}
}

func TestResolveScopeDefaultsTheGlobalHomeToDotHydra(t *testing.T) {
	if got := ResolveScope(false, "/work/app", "/home/u").GlobalHome; got != "/home/u/.hydra" {
		t.Errorf("GlobalHome = %s want /home/u/.hydra", got)
	}
}

func TestRuleRefFollowsTheOverriddenGlobalLibrary(t *testing.T) {
	s := ResolveScopeIn(true, "", "/home/u", "/srv/library")
	r := Rule{Name: "rust", Path: "/srv/library/rules/rust.md"}
	if got := s.RuleRef(r); got != "/srv/library/rules/rust.md" {
		t.Errorf("RuleRef = %s", got)
	}
	if got := s.RulesDirRef(); got != "/srv/library/rules" {
		t.Errorf("RulesDirRef = %s", got)
	}
}
