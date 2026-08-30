package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbilityInitAndSyncAllDetectedHarnesses(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	for _, path := range []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".codex", "AGENTS.md"),
		filepath.Join(home, ".gemini", "GEMINI.md"),
	} {
		mustWrite(t, path, "# Existing\n")
	}

	var out bytes.Buffer
	for range 3 {
		if err := AbilityInit(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	if !exists(filepath.Join(s.AbilitiesDir, abilityIndexFile)) {
		t.Error("ability index not created")
	}
	for _, harness := range detectAbilityHarnesses(s) {
		instructions := readFile(t, harness.InstructionPath)
		if strings.Count(instructions, abilityBlockStart) != 1 {
			t.Errorf("%s abilities block count is not one:\n%s", harness.Name, instructions)
		}
		if got := readFile(t, harness.RouterPath); !strings.Contains(got, routerOwnedMarker) {
			t.Errorf("%s router is not marked as owned", harness.Name)
		}
	}
}

func TestAbilityInitCreatesDefaultHarness(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	var out bytes.Buffer
	if err := AbilityInit(s, &out); err != nil {
		t.Fatal(err)
	}
	harness := defaultAbilityHarness(s)
	if !exists(harness.InstructionPath) || !exists(harness.RouterPath) {
		t.Errorf("default harness was not wired: %+v", harness)
	}
}

func TestAbilityInitCreatesInstructionsForInstalledHarness(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	if err := os.Mkdir(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AbilityInit(s, &out); err != nil {
		t.Fatal(err)
	}
	harness := abilityHarnesses(s)[1]
	if !exists(harness.InstructionPath) || !exists(harness.RouterPath) {
		t.Errorf("installed Codex harness was not wired: %+v", harness)
	}
	if exists(defaultAbilityHarness(s).InstructionPath) {
		t.Error("Claude default was created despite detecting Codex")
	}
}

func TestAbilityInitPreservesAuthoredAbility(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	path := filepath.Join(s.AbilitiesDir, "keep-me", abilityFilename)
	want := validAbility("keep-me", "Keep this authored workflow.")
	mustWrite(t, path, want)

	var out bytes.Buffer
	for range 2 {
		if err := AbilityInit(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFile(t, path); got != want {
		t.Error("ability init modified authored content")
	}
}

func TestAbilityInitInvalidLibraryDoesNotCreateHarnessWiring(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "broken", abilityFilename), "---\nname: wrong\n---\n")
	harness := defaultAbilityHarness(s)

	var out bytes.Buffer
	if err := AbilityInit(s, &out); err == nil {
		t.Fatal("expected invalid library to fail init")
	}
	if exists(harness.InstructionPath) || exists(harness.RouterPath) {
		t.Error("invalid init created harness wiring")
	}
}

func TestAbilitySyncValidationDoesNotRewriteGeneratedFiles(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	var out bytes.Buffer
	if err := AbilityInit(s, &out); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(s.AbilitiesDir, abilityIndexFile)
	harness := defaultAbilityHarness(s)
	beforeIndex := readFile(t, indexPath)
	beforeInstructions := readFile(t, harness.InstructionPath)
	beforeRouter := readFile(t, harness.RouterPath)

	mustWrite(t, filepath.Join(s.AbilitiesDir, "broken", abilityFilename), "---\nname: wrong\n---\n")
	if err := AbilitySync(s, &out); err == nil {
		t.Fatal("expected invalid library to fail")
	}
	if got := readFile(t, indexPath); got != beforeIndex {
		t.Error("invalid sync rewrote index")
	}
	if got := readFile(t, harness.InstructionPath); got != beforeInstructions {
		t.Error("invalid sync rewrote instructions")
	}
	if got := readFile(t, harness.RouterPath); got != beforeRouter {
		t.Error("invalid sync rewrote router")
	}
}

func TestAbilitySyncRouterCollisionDoesNotRewriteIndex(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	if err := os.MkdirAll(s.AbilitiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	harness := defaultAbilityHarness(s)
	mustWrite(t, harness.InstructionPath, "# Existing\n")
	mustWrite(t, harness.RouterPath, "user-authored\n")
	indexPath := filepath.Join(s.AbilitiesDir, abilityIndexFile)
	mustWrite(t, indexPath, "sentinel\n")

	var out bytes.Buffer
	if err := AbilitySync(s, &out); err == nil {
		t.Fatal("expected router collision")
	}
	if got := readFile(t, indexPath); got != "sentinel\n" {
		t.Error("collision rewrote index")
	}
	if got := readFile(t, harness.RouterPath); got != "user-authored\n" {
		t.Error("collision overwrote foreign router")
	}
}

func TestAbilityNew(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	var out bytes.Buffer
	if err := AbilityNew(s, "testing-notes", &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.AbilitiesDir, "testing-notes", abilityFilename)
	if got := readFile(t, path); !strings.Contains(got, "name: testing-notes") {
		t.Errorf("bad scaffold:\n%s", got)
	}
	if err := AbilityNew(s, "testing-notes", &out); err == nil {
		t.Error("expected duplicate to fail")
	}
	for _, name := range []string{"../escape", "UPPER", "trailing-"} {
		if err := AbilityNew(s, name, &out); err == nil {
			t.Errorf("expected invalid name %q to fail", name)
		}
	}
}
