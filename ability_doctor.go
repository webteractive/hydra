package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	harnesses := detectAbilityHarnesses(s)
	add("at least one global instruction file detected", len(harnesses) > 0, sevWarning, "run 'hydra ability init'")
	block := RenderAbilityBlock(s)
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
