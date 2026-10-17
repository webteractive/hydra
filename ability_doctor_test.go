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

// An untriggered auto ability can only be found by description; a direct-only
// one never is, so its missing triggers are not a defect.
func TestAbilityDoctorWarnsOnlyAutoAbilitiesWithoutTriggers(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	mustWrite(t, filepath.Join(s.AbilitiesDir, "deploy-prod", abilityFilename),
		"---\nname: deploy-prod\ndescription: Deploy to production.\ninvocation: explicit\n---\n")
	mustWrite(t, filepath.Join(s.AbilitiesDir, "explain-code", abilityFilename),
		"---\nname: explain-code\ndescription: Explain code.\n---\n")

	var check *DoctorCheck
	checks := AbilityDoctor(s).Checks
	for i, c := range checks {
		if c.Name == "every auto ability has triggers" {
			check = &checks[i]
		}
	}
	if check == nil {
		t.Fatal("expected the trigger check to run")
	}
	if check.OK || !strings.Contains(check.Detail, "explain-code") {
		t.Errorf("an untriggered auto ability should be named: %+v", *check)
	}
	if strings.Contains(check.Detail, "deploy-prod") {
		t.Errorf("a direct-only ability does not rely on description matching: %q", check.Detail)
	}

	mustWrite(t, filepath.Join(s.AbilitiesDir, "explain-code", abilityFilename),
		"---\nname: explain-code\ndescription: Explain code.\ntriggers:\n  - explain this\n---\n")
	for _, c := range AbilityDoctor(s).Checks {
		if c.Name == "every auto ability has triggers" && !c.OK {
			t.Errorf("only a direct-only ability lacks triggers, which is fine: %q", c.Detail)
		}
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

func TestAbilityDoctorFixCommands(t *testing.T) {
	home := t.TempDir()
	s := ResolveAbilityScope(home)
	rep := AbilityDoctor(s)
	c, _ := checkByPrefix(rep, "abilities directory present")
	if strings.Join(c.Fix, " ") != "hydra ability init" || rep.Initialized {
		t.Errorf("missing catalog: fix = %v, initialized = %v", c.Fix, rep.Initialized)
	}

	var out bytes.Buffer
	if err := AbilityInit(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(s.AbilitiesDir, "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers: [ship it]\n---\n")
	rep = AbilityDoctor(s)
	if !rep.Initialized {
		t.Error("an initialized catalog should say so")
	}
	c, ok := checkByPrefix(rep, "abilities index.md is current")
	if !ok || c.OK || strings.Join(c.Fix, " ") != "hydra ability sync" {
		t.Errorf("stale index: %+v", c)
	}
}
