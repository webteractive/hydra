package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The global managed blocks embed absolute paths into the library that wrote
// them, so the blocks themselves record which library the agents are reading.
// A write resolved against a different library — a shell that lost
// HYDRA_HOME after a relocate is the usual way — would quietly repoint every
// agent at it and report success. These guards compare the two first.

var (
	// rulesBlockLibraryRe reads the grep hint every global rules block carries:
	// `grep -rin '<keyword>' <library>/rules`.
	rulesBlockLibraryRe = regexp.MustCompile("`grep -rin '<keyword>' ([^`]+)`")
	// abilityBlockLibraryRe reads the abilities block's
	// "Each ability lives at `<library>/abilities/<name>/ABILITY.md`".
	abilityBlockLibraryRe = regexp.MustCompile("Each ability lives at `([^`]+)/<name>/" + regexp.QuoteMeta(abilityFilename) + "`")
)

// managedBlockText returns the text between a file's sentinels, or "" when
// the file or the block is not there.
func managedBlockText(path, start, end string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	i := strings.Index(content, start)
	if i < 0 {
		return ""
	}
	j := strings.Index(content[i:], end)
	if j < 0 {
		return ""
	}
	return content[i : i+j]
}

// namedLibrary returns the library a block names: the directory above the
// rules or abilities directory its embedded path points into. An empty block,
// or one written before the path was embedded, names nothing.
func namedLibrary(block string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(block)
	if m == nil {
		return ""
	}
	return filepath.Dir(filepath.Clean(filepath.FromSlash(m[1])))
}

// sameLibrary compares two library paths, resolving symlinks where they exist.
func sameLibrary(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

// libraryConflict is one block that names a library other than the resolved one.
type libraryConflict struct {
	File    string // the instruction file holding the block
	Names   string // the library the block names
	Library string // the library this run resolved
	Source  string // what put Library where it is
}

func rulesConflicts(s Scope) []libraryConflict {
	if !s.Global {
		return nil // project blocks embed relative paths and name no global library
	}
	var out []libraryConflict
	for _, t := range DetectTargets(s) {
		named := namedLibrary(managedBlockText(t, blockStart, blockEnd), rulesBlockLibraryRe)
		if named != "" && !sameLibrary(named, s.Home) {
			out = append(out, libraryConflict{File: t, Names: named, Library: s.Home, Source: s.GlobalHomeSource})
		}
	}
	return out
}

func abilityConflicts(s AbilityScope) []libraryConflict {
	var out []libraryConflict
	for _, h := range abilityHarnesses(s) {
		named := namedLibrary(managedBlockText(h.InstructionPath, abilityBlockStart, abilityBlockEnd), abilityBlockLibraryRe)
		if named != "" && !sameLibrary(named, s.HydraHome) {
			out = append(out, libraryConflict{File: h.InstructionPath, Names: named, Library: s.HydraHome, Source: s.HydraHomeSource})
		}
	}
	return out
}

// refuseConflict turns the first conflict into the refusal a write returns:
// exit 2, both paths named, and the two ways forward.
func refuseConflict(conflicts []libraryConflict) error {
	if len(conflicts) == 0 {
		return nil
	}
	c := conflicts[0]
	source := c.Source
	if source == "" {
		source = hydraHomeSourceDefault
	}
	return &exitCodeError{code: 2, msg: fmt.Sprintf(
		"refusing to rewrite the managed block in %s:\n"+
			"  the block names the library at %s\n"+
			"  this run resolved              %s (%s)\n"+
			"Writing now would point every agent at the wrong library. If the block is right, run\n"+
			"  export %s=%q\n"+
			"and try again; to point the block at %s instead, pass --force.",
		c.File, c.Names, c.Library, source, hydraHomeEnv, c.Names, c.Library)}
}

// guardRules refuses a global rules write whose library the blocks do not name.
func guardRules(s Scope) error {
	if s.Force {
		return nil
	}
	return refuseConflict(rulesConflicts(s))
}

// guardAbilities is guardRules for the abilities block, which every scope's
// init writes: a project init wires the global abilities too.
func guardAbilities(s AbilityScope) error {
	if s.Force {
		return nil
	}
	return refuseConflict(abilityConflicts(s))
}

// conflictAdvice is what doctor says instead of a sync the guard would refuse.
func conflictAdvice(c libraryConflict, forceCommand string) string {
	return fmt.Sprintf("the managed block in %s names %s: set %s=%s, or run '%s' to point it at %s",
		c.File, c.Names, hydraHomeEnv, c.Names, forceCommand, c.Library)
}
