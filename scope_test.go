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
