package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sevError   = "error"
	sevWarning = "warning"
)

type DoctorCheck struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

type DoctorReport struct {
	Scope  string        `json:"scope"`
	Home   string        `json:"home"`
	OK     bool          `json:"ok"`
	Checks []DoctorCheck `json:"checks"`
}

// Doctor inspects a scope and returns a structured report. It performs no output
// I/O; rendering is the caller's job. Only error-severity failures clear OK, so a
// stale index reports honestly without turning the exit code red.
func Doctor(s Scope) DoctorReport {
	rep := DoctorReport{Scope: s.Label, Home: s.Home, OK: true}
	add := func(name string, ok bool, severity, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: ok, Severity: severity, Detail: detail})
		if !ok && severity == sevError {
			rep.OK = false
		}
	}

	if !isDir(s.RulesDir) {
		add("rules directory present", false, sevError, "run 'hydra init'")
		return rep
	}
	add("rules directory present", true, sevError, "")

	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		add("every rule parses", false, sevError, err.Error())
		return rep
	}
	add("every rule parses", true, sevError, "")

	seen := map[string]bool{}
	for _, r := range rules {
		add(r.Name+" has a matcher", r.HasMatcher(), sevError,
			"add paths, commands, or triggers, or set always: true")
		add(r.Name+" is uniquely named", !seen[r.Name], sevError, "")
		seen[r.Name] = true
	}

	indexPath := filepath.Join(s.RulesDir, indexFilename)
	current, _ := os.ReadFile(indexPath)
	add("index.md is current", string(current) == RenderIndex(s, rules), sevWarning, "run 'hydra sync'")

	targets := DetectTargets(s)
	add("at least one instruction file detected", len(targets) > 0, sevWarning, "run 'hydra init'")

	block := RenderBlock(s, rules)
	for _, t := range targets {
		add("block current in "+t, blockMatches(t, block), sevWarning, "run 'hydra sync'")
	}

	add("no v0.1 skill-curator artifacts", !hasV01Artifacts(s), sevWarning, "run 'hydra init' to clean up")

	return rep
}

// blockMatches reports whether the target's managed block is byte-identical to
// what a fresh render would produce.
func blockMatches(path, want string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	start := strings.Index(content, blockStart)
	if start < 0 {
		return false
	}
	end := strings.Index(content[start:], blockEnd)
	if end < 0 {
		return false
	}
	got := content[start : start+end+len(blockEnd)]
	return got+"\n" == want
}

func hasV01Artifacts(s Scope) bool {
	for _, p := range []string{
		filepath.Join(s.Home, curatorHookMarker),
		filepath.Join(s.Home, "curator.log"),
		filepath.Join(s.Home, "config"),
	} {
		if exists(p) {
			return true
		}
	}
	// A skills directory existing proves nothing — globally it is shared with
	// skillset, dotfiles, and plugins. Only links into our own library count.
	for _, dir := range skillFarms(s) {
		if stale, _ := staleLinks(dir, ownedSkillDirs(s)); len(stale) > 0 {
			return true
		}
	}
	if fileContains(filepath.Join(s.Base, ".claude", "settings.json"), curatorHookMarker) {
		return true
	}
	for _, t := range candidateTargets(s) {
		if fileContains(t, curatorBlockStart) {
			return true
		}
	}
	return false
}

func renderDoctorText(r DoctorReport, out io.Writer) {
	fmt.Fprintf(out, "hydra doctor (%s: %s)\n", r.Scope, r.Home)
	for _, c := range r.Checks {
		glyph := "✓"
		if !c.OK {
			glyph = "✗"
			if c.Severity == sevWarning {
				glyph = "!"
			}
		}
		line := fmt.Sprintf("  %s %s", glyph, c.Name)
		if !c.OK && c.Detail != "" {
			line += " — " + c.Detail
		}
		fmt.Fprintln(out, line)
	}
	if r.OK {
		fmt.Fprintln(out, "doctor: PASS")
	} else {
		fmt.Fprintln(out, "doctor: FAIL")
	}
}
