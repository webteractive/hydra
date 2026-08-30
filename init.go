package main

import (
	"fmt"
	"io"
	"os"
)

// Init scaffolds the rules library, wires its managed block, and ensures the
// global ability system is ready. It is idempotent and never rewrites authored
// rules or abilities. Every mutating rule command calls it when its library is
// absent, so a fresh project gets the complete Hydra setup.
func Init(s Scope, out io.Writer) error {
	if found, err := Teardown(s, out); err != nil {
		return err
	} else if found {
		fmt.Fprintln(out, "cleaned up a v0.1 skill-curator install")
	}

	if !isDir(s.RulesDir) {
		if err := os.MkdirAll(s.RulesDir, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "created %s\n", s.RulesDir)
	}

	if len(DetectTargets(s)) == 0 {
		for _, t := range DefaultTargets(s) {
			if err := SpliceBlock(t, blockStart+"\n"+blockEnd+"\n"); err != nil {
				return err
			}
			fmt.Fprintf(out, "created %s\n", t)
		}
	}

	if err := Sync(s, out); err != nil {
		return err
	}
	abilityScope, err := abilityScopeFromRuleScope(s)
	if err != nil {
		return err
	}
	if err := AbilityInit(abilityScope, out); err != nil {
		return err
	}

	fmt.Fprintf(out, "hydra init complete (%s: %s)\n", s.Label, s.RulesDir)
	return nil
}
