package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Doctor(s Scope, out io.Writer) bool {
	ok := true
	check := func(cond bool, label string) {
		if cond {
			fmt.Fprintf(out, "  ✓ %s\n", label)
		} else {
			fmt.Fprintf(out, "  ✗ %s\n", label)
			ok = false
		}
	}
	fmt.Fprintf(out, "hydra doctor (%s: %s)\n", s.Label, s.Home)
	check(isDir(filepath.Join(s.Home, "skills")), "skills dir present")
	check(exists(filepath.Join(s.Home, "skills", "skill-curator", "SKILL.md")), "skill-curator seeded")
	check(exists(filepath.Join(s.Home, "curator-reminder.sh")), "hook script present")

	for _, t := range s.RuntimeTargets(runtimes(s)) {
		entries, err := os.ReadDir(t)
		if err != nil {
			continue
		}
		for _, e := range entries {
			link := filepath.Join(t, e.Name())
			fi, err := os.Lstat(link)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			check(resolves(link), "symlink resolves: "+link)
		}
	}

	check(fileContains(s.Settings, "curator-reminder.sh"), "hook wired in "+s.Settings)
	for _, md := range s.ClaudeMDs {
		check(fileContains(md, "hydra:curator:start"), "curator block in "+md)
	}

	if ok {
		fmt.Fprintln(out, "doctor: PASS")
	} else {
		fmt.Fprintln(out, "doctor: FAIL")
	}
	return ok
}
