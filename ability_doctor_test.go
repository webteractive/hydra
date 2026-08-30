package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAbilityDoctorHealthy(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	var out bytes.Buffer
	if err := AbilityInit(s, &out); err != nil {
		t.Fatal(err)
	}
	if rep := AbilityDoctor(s); !rep.OK {
		t.Errorf("doctor failed: %+v", rep.Checks)
	}
}

func TestAbilityDoctorInvalidBundleAndForeignRouter(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	if err := os.MkdirAll(s.AbilitiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(s.AbilitiesDir, "bad", abilityFilename), "---\nname: wrong\n---\n")
	if rep := AbilityDoctor(s); rep.OK {
		t.Error("invalid ability should fail doctor")
	}

	if err := os.RemoveAll(filepath.Join(s.AbilitiesDir, "bad")); err != nil {
		t.Fatal(err)
	}
	harness := defaultAbilityHarness(s)
	mustWrite(t, harness.InstructionPath, RenderAbilityBlock(s))
	mustWrite(t, harness.RouterPath, "foreign\n")
	if rep := AbilityDoctor(s); rep.OK {
		t.Error("foreign router should fail doctor")
	}
}
