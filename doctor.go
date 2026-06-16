package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type DoctorCheck struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

type DoctorReport struct {
	Scope  string        `json:"scope"`
	Home   string        `json:"home"`
	OK     bool          `json:"ok"`
	Checks []DoctorCheck `json:"checks"`
}

// Doctor inspects an install and returns a structured report. It performs no I/O
// on the output; rendering is the caller's job (see renderDoctorText).
func Doctor(s Scope) DoctorReport {
	rep := DoctorReport{Scope: s.Label, Home: s.Home, OK: true}
	add := func(name string, cond bool) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: cond})
		if !cond {
			rep.OK = false
		}
	}

	add("skills dir present", isDir(filepath.Join(s.Home, "skills")))
	add("skill-curator seeded", exists(filepath.Join(s.Home, "skills", "skill-curator", "SKILL.md")))
	add("hook script present", exists(filepath.Join(s.Home, "curator-reminder.sh")))

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
			add("symlink resolves: "+link, resolves(link))
		}
	}

	add("hook wired in "+s.Settings, fileContains(s.Settings, "curator-reminder.sh"))
	for _, md := range s.ClaudeMDs {
		add("curator block in "+md, fileContains(md, "hydra:curator:start"))
	}

	return rep
}

// renderDoctorText reproduces the legacy human-readable doctor output.
func renderDoctorText(r DoctorReport, out io.Writer) {
	fmt.Fprintf(out, "hydra doctor (%s: %s)\n", r.Scope, r.Home)
	for _, c := range r.Checks {
		if c.OK {
			fmt.Fprintf(out, "  ✓ %s\n", c.Name)
		} else {
			fmt.Fprintf(out, "  ✗ %s\n", c.Name)
		}
	}
	if r.OK {
		fmt.Fprintln(out, "doctor: PASS")
	} else {
		fmt.Fprintln(out, "doctor: FAIL")
	}
}
