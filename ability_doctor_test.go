package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	mustWrite(t, harness.InstructionPath, RenderAbilityBlock(s, nil))
	mustWrite(t, harness.RouterPath, "foreign\n")
	if rep := AbilityDoctor(s); rep.OK {
		t.Error("foreign router should fail doctor")
	}
}

func TestAbilityDoctorFlagsTriggersShadowedByAnAbilityName(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "explain-code", abilityFilename),
		"---\nname: explain-code\ndescription: Explain code.\n---\n")
	mustWrite(t, filepath.Join(s.AbilitiesDir, "review-code", abilityFilename),
		"---\nname: review-code\ndescription: Review code.\ntriggers:\n  - Explain Code\n  - review this\n---\n")

	var check *DoctorCheck
	for i, c := range AbilityDoctor(s).Checks {
		if c.Name == "no trigger is shadowed by an ability name" {
			check = &AbilityDoctor(s).Checks[i]
		}
	}
	if check == nil {
		t.Fatal("expected the shadowed-trigger check to run")
	}
	if check.OK {
		t.Error("a trigger equal to another ability's name can never fire")
	}
	if !strings.Contains(check.Detail, "explain-code") {
		t.Errorf("detail should name the shadowing ability: %q", check.Detail)
	}
}

func TestAbilityDoctorAllowsOverlappingTriggers(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "ship-library", abilityFilename),
		"---\nname: ship-library\ndescription: Ship a library.\ntriggers:\n  - ship it\n---\n")
	mustWrite(t, filepath.Join(s.AbilitiesDir, "ship-mobile", abilityFilename),
		"---\nname: ship-mobile\ndescription: Ship a mobile build.\ntriggers:\n  - ship it\n---\n")

	for _, c := range AbilityDoctor(s).Checks {
		if c.Name == "no trigger is shadowed by an ability name" && !c.OK {
			t.Errorf("overlapping triggers are delegated to the agent, not a defect: %q", c.Detail)
		}
	}
}

// A trigger is shadowed if either form of it resolves to another ability's
// name, since MatchAbilities tries the raw and the filler-stripped wording.
func TestAbilityDoctorFlagsTriggersShadowedAfterFillerStripping(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "deploy", abilityFilename),
		"---\nname: deploy\ndescription: Deploy.\n---\n")
	mustWrite(t, filepath.Join(s.AbilitiesDir, "alpha", abilityFilename),
		"---\nname: alpha\ndescription: Alpha.\ntriggers:\n  - just deploy\n---\n")

	// Confirm the runtime really does swallow it, so the check is not theoretical.
	abilities, err := LoadAbilities(s.AbilitiesDir)
	if err != nil {
		t.Fatal(err)
	}
	got := MatchAbilities(abilities, "just deploy")
	if len(got) != 1 || got[0].Ability != "deploy" || got[0].Kind != MatchName {
		t.Fatalf("expected a decisive name match on deploy: %+v", got)
	}

	for _, c := range AbilityDoctor(s).Checks {
		if c.Name == "no trigger is shadowed by an ability name" {
			if c.OK {
				t.Error("alpha's \"just deploy\" trigger can never fire and must be flagged")
			}
			return
		}
	}
	t.Fatal("shadow check did not run")
}
