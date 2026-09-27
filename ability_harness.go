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
	// Picker is the harness's native selection prompt, which the router hands a
	// bare `$ability` to instead of answering with a list the user has to retype.
	Picker AbilityPicker `json:"-"`
}

// AbilityPicker names a harness's structured question tool and how many choices
// one question may carry. Both tools append their own free-text answer, which is
// how a user who already knows the name can type it without paging.
type AbilityPicker struct {
	Tool       string
	MaxOptions int
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
			Picker:          AbilityPicker{Tool: "AskUserQuestion", MaxOptions: 4},
		},
		{
			Name:            "codex",
			InstructionPath: filepath.Join(s.UserHome, ".codex", "AGENTS.md"),
			RouterPath:      filepath.Join(s.UserHome, ".agents", "skills", "ability", "SKILL.md"),
			Picker:          AbilityPicker{Tool: "request_user_input", MaxOptions: 3},
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
