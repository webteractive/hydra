package main

import "testing"

func matchFixture() []Ability {
	return []Ability{
		{
			Name:        "prepare-for-production",
			Description: "Harden a change for production.",
			Triggers:    []string{"make it production ready", "primetime"},
			Path:        "/home/user/.hydra/abilities/prepare-for-production/ABILITY.md",
		},
		{
			Name:        "explain-code",
			Description: "Explain unfamiliar code.",
			Triggers:    []string{"explain this code"},
			Path:        "/home/user/.hydra/abilities/explain-code/ABILITY.md",
		},
	}
}

func TestMatchAbilitiesByName(t *testing.T) {
	got := MatchAbilities(matchFixture(), "Prepare For Production")
	if len(got) != 1 || got[0].Ability != "prepare-for-production" || got[0].Kind != MatchName {
		t.Fatalf("unexpected matches: %+v", got)
	}
}

func TestMatchAbilitiesByTriggerIgnoringPunctuationAndCase(t *testing.T) {
	for _, phrase := range []string{"Primetime!", "primetime", "  PRIMETIME  "} {
		got := MatchAbilities(matchFixture(), phrase)
		if len(got) != 1 || got[0].Kind != MatchTrigger || got[0].Trigger != "primetime" {
			t.Errorf("%q: unexpected matches: %+v", phrase, got)
		}
	}
}

func TestMatchAbilitiesFindsTriggerInsideLongerWording(t *testing.T) {
	got := MatchAbilities(matchFixture(), "ok now make it production ready please")
	if len(got) != 1 || got[0].Ability != "prepare-for-production" || got[0].Kind != MatchTrigger {
		t.Fatalf("unexpected matches: %+v", got)
	}
}

func TestMatchAbilitiesRequiresWholeWords(t *testing.T) {
	if got := MatchAbilities(matchFixture(), "primetimely"); len(got) != 0 {
		t.Errorf("partial word should not match: %+v", got)
	}
}

func TestMatchAbilitiesReportsNoMatch(t *testing.T) {
	if got := MatchAbilities(matchFixture(), "prep for prod"); len(got) != 0 {
		t.Errorf("expected no match: %+v", got)
	}
}

func TestMatchAbilitiesPrefersNameOverTriggerAndDeduplicates(t *testing.T) {
	abilities := []Ability{{
		Name:     "explain-code",
		Triggers: []string{"explain code"},
		Path:     "/tmp/explain-code/ABILITY.md",
	}}
	got := MatchAbilities(abilities, "explain code")
	if len(got) != 1 || got[0].Kind != MatchName {
		t.Fatalf("name match should win and not duplicate: %+v", got)
	}
}

func TestMatchAbilitiesReturnsEveryCandidateForTheAgentToChoose(t *testing.T) {
	abilities := []Ability{
		{Name: "one", Description: "Ship a library release.", Triggers: []string{"ship it"}, Path: "/tmp/one/ABILITY.md"},
		{Name: "two", Description: "Ship a mobile build.", Triggers: []string{"ship it"}, Path: "/tmp/two/ABILITY.md"},
	}
	got := MatchAbilities(abilities, "ship it")
	if len(got) != 2 {
		t.Fatalf("both abilities should match: %+v", got)
	}
	// A candidate set is only useful if each entry says enough to choose between
	// them without opening both bundles.
	for _, match := range got {
		if match.Description == "" {
			t.Errorf("candidate %q must carry its description: %+v", match.Ability, match)
		}
	}
}

func TestMatchAbilitiesNameMatchIsDecisive(t *testing.T) {
	abilities := []Ability{
		{Name: "explain-code", Description: "Explain code.", Path: "/tmp/explain-code/ABILITY.md"},
		{Name: "review-code", Description: "Review code.", Triggers: []string{"explain code"}, Path: "/tmp/review-code/ABILITY.md"},
	}
	got := MatchAbilities(abilities, "explain code")
	if len(got) != 1 || got[0].Ability != "explain-code" || got[0].Kind != MatchName {
		t.Fatalf("an exact name is explicit intent and must not be diluted by trigger candidates: %+v", got)
	}
}

// Triggers are an explicit-invocation mechanism, so a trigger fires only when it
// is what the user actually said — not when it happens to appear inside a
// sentence about something else. Looser phrasings fall through to semantic
// description matching, which is the recall tier.
func TestMatchAbilitiesAnchorsTriggersToTheWholeRequest(t *testing.T) {
	fires := []string{
		"primetime",
		"Primetime!",
		"ok primetime",
		"primetime, thanks",
		"make it production ready",
		"please make it production ready now",
		"can you make it production ready",
	}
	for _, phrase := range fires {
		got := MatchAbilities(matchFixture(), phrase)
		if len(got) != 1 || got[0].Ability != "prepare-for-production" {
			t.Errorf("%q should fire prepare-for-production, got %+v", phrase, got)
		}
	}
}

func TestMatchAbilitiesIgnoresTriggersBuriedInOtherWork(t *testing.T) {
	abilities := []Ability{
		{Name: "summarize-git-changes", Description: "Summarize changes.", Triggers: []string{"what changed"}, Path: "/tmp/a/ABILITY.md"},
		{Name: "explain-code", Description: "Explain code.", Triggers: []string{"explain this"}, Path: "/tmp/b/ABILITY.md"},
		{Name: "greet-someone", Description: "Greet someone.", Triggers: []string{"say hi"}, Path: "/tmp/c/ABILITY.md"},
		{Name: "deploy-web", Description: "Deploy web.", Triggers: []string{"ship it"}, Path: "/tmp/d/ABILITY.md"},
		{Name: "brainstorm-test-cases", Description: "Test ideas.", Triggers: []string{"what should i test"}, Path: "/tmp/e/ABILITY.md"},
	}
	// Every one of these fired before anchoring.
	for _, phrase := range []string{
		"what changed in the nginx config?",
		"can you explain this to me like I'm five",
		"the deploy failed, what changed on the server",
		"say hi to the API and check it responds",
		"I need to ship it by Friday, can you estimate the work",
		"explain this stack trace",
		"what should I test for in code review",
	} {
		if got := MatchAbilities(abilities, phrase); len(got) != 0 {
			t.Errorf("%q must not be an explicit invocation, matched %+v", phrase, got)
		}
	}
}

func TestMatchAbilitiesStripsPolitenessAroundNames(t *testing.T) {
	got := MatchAbilities(matchFixture(), "can you prepare for production please")
	if len(got) != 1 || got[0].Kind != MatchName {
		t.Fatalf("politeness must not defeat an exact name: %+v", got)
	}
}

// stripFiller exists so politeness cannot defeat a match. It must not make a
// name or trigger that legitimately contains one of those words unmatchable.
func TestMatchAbilitiesMatchesNamesContainingFillerWords(t *testing.T) {
	abilities := []Ability{
		{Name: "ship-it-now", Description: "Ship now.", Path: "/tmp/a/ABILITY.md"},
		{Name: "just-fix-it", Description: "Fix it.", Path: "/tmp/b/ABILITY.md"},
	}
	for _, phrase := range []string{"ship it now", "ship-it-now", "just fix it", "just-fix-it"} {
		got := MatchAbilities(abilities, phrase)
		if len(got) != 1 || got[0].Kind != MatchName {
			t.Errorf("%q names an ability verbatim and must match: %+v", phrase, got)
		}
	}
}

func TestMatchAbilitiesMatchesTriggersContainingFillerWords(t *testing.T) {
	abilities := []Ability{{
		Name:        "alpha",
		Description: "Alpha.",
		Triggers:    []string{"just do it", "ok go", "wrap it up now", "review this please"},
		Path:        "/tmp/a/ABILITY.md",
	}}
	for _, phrase := range []string{"just do it", "ok go", "wrap it up now", "review this please"} {
		if got := MatchAbilities(abilities, phrase); len(got) != 1 {
			t.Errorf("%q is a trigger typed verbatim and must fire: %+v", phrase, got)
		}
	}
}

// Politeness stripping must still work in the other direction.
func TestMatchAbilitiesStillStripsPolitenessAroundPlainTriggers(t *testing.T) {
	abilities := []Ability{{Name: "alpha", Triggers: []string{"do it"}, Path: "/tmp/a/ABILITY.md"}}
	for _, phrase := range []string{"do it", "just do it", "please do it now"} {
		if got := MatchAbilities(abilities, phrase); len(got) != 1 {
			t.Errorf("%q should reach trigger \"do it\": %+v", phrase, got)
		}
	}
}
