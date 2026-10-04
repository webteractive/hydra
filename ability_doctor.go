package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func AbilityDoctor(s AbilityScope) DoctorReport {
	rep := DoctorReport{Scope: "global abilities", Home: s.HydraHome, HomeSource: s.HydraHomeSource, OK: true}
	add := func(name string, ok bool, severity, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: ok, Severity: severity, Detail: detail})
		if !ok && severity == sevError {
			rep.OK = false
		}
	}
	// While the block names another library, every ability sync or init this
	// report could advise would be refused (guard.go): the advice is the variable.
	conflicts := abilityConflicts(s)
	// fixable adds a check whose remedy is a hydra ability command; detail is
	// a format whose %s is the "run '...'" phrase. Abilities are always global,
	// so the command never needs --global.
	fixable := func(name string, ok bool, severity, detail string, verb ...string) {
		fix := scopedCommand(false, s.HydraHome, s.HydraHomeSource, append([]string{"ability"}, verb...)...)
		if len(conflicts) > 0 {
			add(name, ok, severity, conflictAdvice(conflicts[0], strings.Join(append(fix, "--force"), " ")))
			return
		}
		add(name, ok, severity, fmt.Sprintf(detail, runPhrase(fix)))
		rep.Checks[len(rep.Checks)-1].Fix = fix
	}

	if !isDir(s.AbilitiesDir) {
		fixable("abilities directory present", false, sevError, "%s", "init")
		return rep
	}
	rep.Initialized = true
	add("abilities directory present", true, sevError, "")

	abilities, err := LoadAbilities(s.AbilitiesDir)
	if err != nil {
		add("every ability is valid", false, sevError, err.Error())
		return rep
	}
	add("every ability is valid", true, sevError, "")

	indexPath := filepath.Join(s.AbilitiesDir, abilityIndexFile)
	currentIndex, _ := os.ReadFile(indexPath)
	fixable("abilities index.md is current", string(currentIndex) == RenderAbilityIndex(abilities), sevWarning, "%s", "sync")

	var untriggered []string
	for _, ability := range abilities {
		if len(ability.Triggers) == 0 {
			untriggered = append(untriggered, ability.Name)
		}
	}
	add("every ability has triggers", len(untriggered) == 0, sevWarning,
		"these rely on description matching alone and may never fire: "+strings.Join(untriggered, ", "))

	// A name match is decisive, so a trigger that normalizes to some other
	// ability's name can never fire. Overlapping triggers between abilities are
	// deliberately not flagged: the agent resolves those from context.
	names := map[string]bool{}
	for _, ability := range abilities {
		names[ability.Name] = true
	}
	var shadowed []string
	for _, ability := range abilities {
		for _, trigger := range ability.Triggers {
			// Mirror MatchAbilities, which tries both the raw and the
			// filler-stripped wording before falling through to triggers.
			tokens := matchTokens(trigger)
			for _, owner := range []string{kebabForm(tokens), kebabForm(stripFiller(tokens))} {
				if owner != ability.Name && names[owner] {
					shadowed = append(shadowed, fmt.Sprintf("%q on %s is always claimed by %s", trigger, ability.Name, owner))
					break
				}
			}
		}
	}
	sort.Strings(shadowed)
	add("no trigger is shadowed by an ability name", len(shadowed) == 0, sevWarning,
		"these can never fire: "+strings.Join(shadowed, "; "))

	fixable("no gemini artifacts", !hasGeminiAbilityArtifacts(s), sevWarning,
		"gemini support was removed — %s to clean up", "init")

	harnesses := detectAbilityHarnesses(s)
	fixable("at least one global instruction file detected", len(harnesses) > 0, sevWarning, "%s", "init")
	block := RenderAbilityBlock(s, abilities)
	conflicting := map[string]bool{}
	for _, c := range conflicts {
		conflicting[c.File] = true
		add("block names this library in "+c.File, false, sevError,
			conflictAdvice(c, strings.Join(scopedCommand(false, s.HydraHome, s.HydraHomeSource, "ability", "sync", "--force"), " ")))
	}
	for _, harness := range harnesses {
		if !conflicting[harness.InstructionPath] {
			fixable("abilities block current in "+harness.InstructionPath,
				managedBlockMatches(harness.InstructionPath, block, abilityBlockStart, abilityBlockEnd),
				sevWarning, "%s", "sync")
		}

		data, readErr := os.ReadFile(harness.RouterPath)
		if readErr != nil {
			if exists(filepath.Dir(harness.RouterPath)) {
				add(harness.Name+" ability router is Hydra-owned", false, sevError,
					"router directory exists but is not a Hydra-managed router: "+filepath.Dir(harness.RouterPath))
			} else {
				fixable(harness.Name+" ability router is installed", false, sevWarning, "%s", "sync")
			}
			continue
		}
		owned := strings.Contains(string(data), routerOwnedMarker)
		add(harness.Name+" ability router is Hydra-owned", owned, sevError,
			"refusing to overwrite user-authored skill at "+harness.RouterPath)
		if owned {
			fixable(harness.Name+" ability router is current", string(data) == RenderAbilityRouter(s, harness), sevWarning, "%s", "sync")
		}
	}

	return rep
}

func abilityDoctorSummary(rep DoctorReport) string {
	return fmt.Sprintf("hydra ability doctor (%s)", doctorHome(rep))
}
