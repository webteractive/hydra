package main

import "path/filepath"

// candidateTargets lists the agent instruction files hydra knows how to write,
// in the order they should be reported. Targets are detected, never configured.
func candidateTargets(s Scope) []string {
	if s.Global {
		return []string{
			filepath.Join(s.Base, ".claude", "CLAUDE.md"),
			filepath.Join(s.Base, ".codex", "AGENTS.md"),
		}
	}
	return []string{
		filepath.Join(s.Base, "CLAUDE.md"),
		filepath.Join(s.Base, "AGENTS.md"),
	}
}

// DetectTargets returns the candidates that already exist.
func DetectTargets(s Scope) []string {
	found := []string{}
	for _, c := range candidateTargets(s) {
		if exists(c) {
			found = append(found, c)
		}
	}
	return found
}

// DefaultTargets is what init creates when detection finds nothing. A project
// gets the CLAUDE.md/AGENTS.md mirror pair; a home directory gets Claude Code's
// file, since creating config directories for agents that aren't installed
// would be presumptuous.
func DefaultTargets(s Scope) []string {
	if s.Global {
		return []string{filepath.Join(s.Base, ".claude", "CLAUDE.md")}
	}
	return []string{
		filepath.Join(s.Base, "CLAUDE.md"),
		filepath.Join(s.Base, "AGENTS.md"),
	}
}
