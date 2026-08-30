package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AbilityHarness struct {
	Name            string `json:"name"`
	InstructionPath string `json:"instruction_path"`
	RouterPath      string `json:"router_path"`
}

// abilityHarnesses is the adapter registry. The Codex adapter uses the shared
// Agent Skills directory that Codex discovers; system-owned Codex skills remain
// separate under .codex/skills.
func abilityHarnesses(s AbilityScope) []AbilityHarness {
	return []AbilityHarness{
		{
			Name:            "claude",
			InstructionPath: filepath.Join(s.UserHome, ".claude", "CLAUDE.md"),
			RouterPath:      filepath.Join(s.UserHome, ".claude", "skills", "ability", "SKILL.md"),
		},
		{
			Name:            "codex",
			InstructionPath: filepath.Join(s.UserHome, ".codex", "AGENTS.md"),
			RouterPath:      filepath.Join(s.UserHome, ".agents", "skills", "ability", "SKILL.md"),
		},
	}
}

func detectAbilityHarnesses(s AbilityScope) []AbilityHarness {
	var found []AbilityHarness
	for _, harness := range abilityHarnesses(s) {
		if exists(harness.InstructionPath) {
			found = append(found, harness)
		}
	}
	return found
}

// initialAbilityHarnesses includes harnesses whose config directory already
// exists even when their global instruction file does not. This lets a fresh
// project init wire the agent the user actually has installed instead of
// blindly creating Claude configuration. If no harness is detectable, Hydra
// retains its existing Claude-first default.
func initialAbilityHarnesses(s AbilityScope) []AbilityHarness {
	var found []AbilityHarness
	for _, harness := range abilityHarnesses(s) {
		if exists(harness.InstructionPath) || isDir(filepath.Dir(harness.InstructionPath)) {
			found = append(found, harness)
		}
	}
	if len(found) == 0 {
		return []AbilityHarness{defaultAbilityHarness(s)}
	}
	return found
}

func defaultAbilityHarness(s AbilityScope) AbilityHarness {
	return abilityHarnesses(s)[0]
}

func preflightAbilityRouters(harnesses []AbilityHarness) error {
	var collisions []string
	for _, harness := range harnesses {
		routerDir := filepath.Dir(harness.RouterPath)
		if !exists(routerDir) {
			continue
		}
		data, err := os.ReadFile(harness.RouterPath)
		if err == nil && strings.Contains(string(data), routerOwnedMarker) {
			continue
		}
		collisions = append(collisions, fmt.Sprintf("%s router path is not Hydra-owned: %s", harness.Name, routerDir))
	}
	if len(collisions) > 0 {
		return fmt.Errorf("cannot install ability router:\n  %s", strings.Join(collisions, "\n  "))
	}
	return nil
}

func writeAbilityRouter(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
