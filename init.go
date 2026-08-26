package main

import (
	"fmt"
	"io"
	"os"
)

// Init scaffolds the rules library and wires the managed block into the scope's
// instruction files. It is idempotent: existing rules and instruction files are
// left alone, only missing pieces are created. Every mutating command calls it,
// so recording a rule in a fresh project just works.
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

	fmt.Fprintf(out, "hydra init complete (%s: %s)\n", s.Label, s.RulesDir)
	return nil
}
