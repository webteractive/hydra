package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Sync reparses the library and rewrites both generated artifacts: index.md and
// the managed block in every detected instruction file. It deliberately does not
// create the library — that is init's job, so a typo'd directory is reported
// rather than silently scaffolded.
func Sync(s Scope, out io.Writer) error {
	if !isDir(s.RulesDir) {
		return fmt.Errorf("no rules library at %s — run 'hydra init' first", s.RulesDir)
	}

	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		return err
	}

	for _, r := range rules {
		if !r.HasMatcher() {
			fmt.Fprintf(out, "warning: %s has no paths, commands, or triggers and is not always:true — it can never fire\n", r.Name)
		}
	}

	indexPath := filepath.Join(s.RulesDir, indexFilename)
	if err := os.WriteFile(indexPath, []byte(RenderIndex(s, rules)), 0o644); err != nil {
		return err
	}

	targets := DetectTargets(s)
	if len(targets) == 0 {
		fmt.Fprintf(out, "warning: no agent instruction files found for the %s scope — run 'hydra init' to create them\n", s.Label)
	}
	block := RenderBlock(s, rules)
	for _, t := range targets {
		if err := SpliceBlock(t, block); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "indexed %d rule(s) → %d target(s)\n", len(rules), len(targets))
	return nil
}
