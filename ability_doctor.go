package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func AbilityDoctor(s AbilityScope) DoctorReport {
	rep := DoctorReport{Scope: "global abilities", Home: s.HydraHome, OK: true}
	add := func(name string, ok bool, severity, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: ok, Severity: severity, Detail: detail})
		if !ok && severity == sevError {
			rep.OK = false
		}
	}

	if !isDir(s.AbilitiesDir) {
		add("abilities directory present", false, sevError, "run 'hydra ability init'")
		return rep
	}
	add("abilities directory present", true, sevError, "")

	abilities, err := LoadAbilities(s.AbilitiesDir)
	if err != nil {
		add("every ability is valid", false, sevError, err.Error())
		return rep
	}
	add("every ability is valid", true, sevError, "")

	indexPath := filepath.Join(s.AbilitiesDir, abilityIndexFile)
	currentIndex, _ := os.ReadFile(indexPath)
	add("abilities index.md is current", string(currentIndex) == RenderAbilityIndex(abilities), sevWarning, "run 'hydra ability sync'")

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

	add("no gemini artifacts", !hasGeminiAbilityArtifacts(s), sevWarning,
		"gemini support was removed — run 'hydra ability init' to clean up")

	harnesses := detectAbilityHarnesses(s)
	add("at least one global instruction file detected", len(harnesses) > 0, sevWarning, "run 'hydra ability init'")
	block := RenderAbilityBlock(s, abilities)
	router := RenderAbilityRouter(s)
	for _, harness := range harnesses {
		add("abilities block current in "+harness.InstructionPath,
			managedBlockMatches(harness.InstructionPath, block, abilityBlockStart, abilityBlockEnd),
			sevWarning, "run 'hydra ability sync'")

		data, readErr := os.ReadFile(harness.RouterPath)
		if readErr != nil {
			if exists(filepath.Dir(harness.RouterPath)) {
				add(harness.Name+" ability router is Hydra-owned", false, sevError,
					"router directory exists but is not a Hydra-managed router: "+filepath.Dir(harness.RouterPath))
			} else {
				add(harness.Name+" ability router is installed", false, sevWarning, "run 'hydra ability sync'")
			}
			continue
		}
		owned := strings.Contains(string(data), routerOwnedMarker)
		add(harness.Name+" ability router is Hydra-owned", owned, sevError,
			"refusing to overwrite user-authored skill at "+harness.RouterPath)
		if owned {
			add(harness.Name+" ability router is current", string(data) == router, sevWarning, "run 'hydra ability sync'")
		}
	}

	return rep
}

func abilityDoctorSummary(rep DoctorReport) string {
	return fmt.Sprintf("hydra ability doctor (%s)", rep.Home)
}
