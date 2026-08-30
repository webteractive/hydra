package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AbilityInit creates the global ability library and all generated wiring. It
// never creates or rewrites an authored ability bundle.
func AbilityInit(s AbilityScope, out io.Writer) error {
	if _, err := removeGeminiAbilityArtifacts(s, out); err != nil {
		return err
	}

	if !isDir(s.AbilitiesDir) {
		if err := os.MkdirAll(s.AbilitiesDir, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "created %s\n", s.AbilitiesDir)
	}

	if _, err := LoadAbilities(s.AbilitiesDir); err != nil {
		return fmt.Errorf("invalid abilities:\n%w", err)
	}
	harnesses := initialAbilityHarnesses(s)
	if err := preflightAbilityRouters(harnesses); err != nil {
		return err
	}
	for _, harness := range harnesses {
		if exists(harness.InstructionPath) {
			continue
		}
		empty := abilityBlockStart + "\n" + abilityBlockEnd + "\n"
		if err := SpliceManagedBlock(harness.InstructionPath, empty, abilityBlockStart, abilityBlockEnd); err != nil {
			return err
		}
		fmt.Fprintf(out, "created %s\n", harness.InstructionPath)
	}

	if err := AbilitySync(s, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "hydra ability init complete (global: %s)\n", s.AbilitiesDir)
	return nil
}

// AbilitySync validates the complete library before touching generated files,
// then refreshes the external catalog, instruction blocks, and owned routers.
func AbilitySync(s AbilityScope, out io.Writer) error {
	if !isDir(s.AbilitiesDir) {
		return fmt.Errorf("no abilities library at %s — run 'hydra ability init' first", s.AbilitiesDir)
	}

	abilities, err := LoadAbilities(s.AbilitiesDir)
	if err != nil {
		return fmt.Errorf("invalid abilities:\n%w", err)
	}
	harnesses := detectAbilityHarnesses(s)
	if err := preflightAbilityRouters(harnesses); err != nil {
		return err
	}

	indexPath := filepath.Join(s.AbilitiesDir, abilityIndexFile)
	if err := os.WriteFile(indexPath, []byte(RenderAbilityIndex(abilities)), 0o644); err != nil {
		return err
	}

	if len(harnesses) == 0 {
		fmt.Fprintln(out, "warning: no global agent instruction files found — run 'hydra ability init' to create one")
	}
	block := RenderAbilityBlock(s, abilities)
	router := RenderAbilityRouter(s)
	for _, harness := range harnesses {
		if err := SpliceManagedBlock(harness.InstructionPath, block, abilityBlockStart, abilityBlockEnd); err != nil {
			return err
		}
		if err := writeAbilityRouter(harness.RouterPath, router); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "indexed %d ability(s) → %d harness(es)\n", len(abilities), len(harnesses))
	return nil
}

// AbilityNew scaffolds one authored bundle. Existing directories are never
// reused, even when they do not yet contain ABILITY.md.
func AbilityNew(s AbilityScope, name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("usage: hydra ability new <name>")
	}
	if !kebab.MatchString(name) {
		return fmt.Errorf("name must be kebab-case (lowercase letters, digits, hyphens): %s", name)
	}
	if !isDir(s.AbilitiesDir) {
		if err := AbilityInit(s, out); err != nil {
			return err
		}
	}

	bundleDir := filepath.Join(s.AbilitiesDir, name)
	if exists(bundleDir) {
		return fmt.Errorf("ability already exists: %s", bundleDir)
	}
	if err := os.Mkdir(bundleDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(bundleDir, abilityFilename)
	if err := os.WriteFile(path, []byte(RenderAbilityFile(name)), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s\n", path)
	return AbilitySync(s, out)
}
