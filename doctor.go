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
	// Fix is the command that clears the check, as an argument vector, scoped
	// the way the report was. Scripts run or show it without parsing Detail.
	Fix []string `json:"fix,omitempty"`
}

type DoctorReport struct {
	Scope string `json:"scope"`
	Home  string `json:"home"`
	// HomeSource names what put the library where it is — only meaningful for
	// the global scope, where an override can move it.
	HomeSource string `json:"home_source,omitempty"`
	// Initialized says whether the library directory exists at all, so a
	// caller can tell "this is not a hydra project" from "this one is broken".
	Initialized bool          `json:"initialized"`
	OK          bool          `json:"ok"`
	Checks      []DoctorCheck `json:"checks"`
}

// runPhrase renders a check's fix for people, as "run '...'". The detail and
// the Fix argv come from the same slice, so they cannot disagree.
func runPhrase(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t'\"") {
			a = "\"" + a + "\""
		}
		quoted[i] = a
	}
	return "run '" + strings.Join(quoted, " ") + "'"
}

// scopedCommand builds a hydra command that acts on the same library the
// report read. A global report needs --global, and a library moved for one run
// with --hydra-home needs that flag again — the environment variable carries
// over on its own, so it needs nothing.
func scopedCommand(global bool, home, homeSource string, verb ...string) []string {
	argv := append([]string{"hydra"}, verb...)
	if global {
		argv = append(argv, "--global")
	}
	if homeSource == hydraHomeSourceFlag {
		argv = append(argv, hydraHomeFlag, home)
	}
	return argv
}

// Doctor inspects a scope and returns a structured report. It performs no output
// I/O; rendering is the caller's job. Only error-severity failures clear OK, so a
// stale index reports honestly without turning the exit code red.
func Doctor(s Scope) DoctorReport {
	rep := DoctorReport{Scope: s.Label, Home: s.Home, OK: true}
	// Only the global library can move, so only it has a source worth naming.
	if s.Global {
		rep.HomeSource = s.GlobalHomeSource
	}
	add := func(name string, ok bool, severity, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: ok, Severity: severity, Detail: detail})
		if !ok && severity == sevError {
			rep.OK = false
		}
	}
	// While a block names another library, every sync or init this report could
	// advise would be refused (guard.go): the advice is the variable instead.
	conflicts := rulesConflicts(s)
	// fixable adds a check whose remedy is a hydra command for this scope;
	// detail is a format whose %s is the "run '...'" phrase.
	fixable := func(name string, ok bool, severity, detail string, verb ...string) {
		fix := scopedCommand(s.Global, s.Home, rep.HomeSource, verb...)
		if len(conflicts) > 0 {
			add(name, ok, severity, conflictAdvice(conflicts[0], strings.Join(append(fix, "--force"), " ")))
			return
		}
		add(name, ok, severity, fmt.Sprintf(detail, runPhrase(fix)))
		rep.Checks[len(rep.Checks)-1].Fix = fix
	}

	if !isDir(s.RulesDir) {
		fixable("rules directory present", false, sevError, "%s", "init")
		return rep
	}
	rep.Initialized = true
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
	// A missing index reads as "", which never equals a render — so the check
	// fails and reports the sync command, which is the right advice either way.
	current, _ := os.ReadFile(indexPath)
	fixable("index.md is current", string(current) == RenderIndex(s, rules), sevWarning, "%s", "sync")

	targets := DetectTargets(s)
	fixable("at least one instruction file detected", len(targets) > 0, sevWarning, "%s", "init")

	block := RenderBlock(s, rules)
	conflicting := map[string]bool{}
	for _, c := range conflicts {
		conflicting[c.File] = true
		add("block names this library in "+c.File, false, sevError,
			conflictAdvice(c, strings.Join(scopedCommand(s.Global, s.Home, rep.HomeSource, "sync", "--force"), " ")))
	}
	for _, t := range targets {
		if !conflicting[t] {
			fixable("block current in "+t, blockMatches(t, block), sevWarning, "%s", "sync")
		}
	}

	fixable("no v0.1 skill-curator artifacts", !hasV01Artifacts(s), sevWarning, "%s to clean up", "init")
	fixable("no gemini artifacts", !hasGeminiArtifacts(s), sevWarning, "gemini support was removed — %s to clean up", "init")

	return rep
}

// blockMatches reports whether the target's managed block is byte-identical to
// what a fresh render would produce.
func blockMatches(path, want string) bool {
	return managedBlockMatches(path, want, blockStart, blockEnd)
}

func managedBlockMatches(path, want, startSentinel, endSentinel string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	start := strings.Index(content, startSentinel)
	if start < 0 {
		return false
	}
	end := strings.Index(content[start:], endSentinel)
	if end < 0 {
		return false
	}
	got := content[start : start+end+len(endSentinel)]
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

// doctorHome renders the library path, naming what put it there when that was
// something other than the default. An override that has silently stopped
// applying is the failure this makes visible.
func doctorHome(r DoctorReport) string {
	if r.HomeSource == "" || r.HomeSource == hydraHomeSourceDefault {
		return r.Home
	}
	return r.Home + " via " + r.HomeSource
}

// renderDoctorText is the one doctor renderer. Both doctors print the same
// shape, so the JSON pointer and the PASS/FAIL line cannot drift apart.
func renderDoctorText(out io.Writer, r DoctorReport, header, jsonCommand string) {
	fmt.Fprintln(out, header)
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
	fmt.Fprintf(out, "\nFor agents and scripts: %s\n", jsonCommand)
}
