package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Scope is one rules library plus how its paths should be written into agent
// instruction files. Project and global scopes are fully independent: neither
// reads the other, and hydra never merges them.
type Scope struct {
	Global   bool   `json:"global"`
	Label    string `json:"label"`     // "project" | "global"
	Base     string `json:"base"`      // cwd for project, home for global
	UserHome string `json:"user_home"` // user home, retained for global abilities
	Home     string `json:"home"`      // <Base>/.hydra
	RulesDir string `json:"rules_dir"` // <Base>/.hydra/rules
}

func ResolveScope(global bool, cwd, home string) Scope {
	base, label := cwd, "project"
	if global {
		base, label = home, "global"
	}
	hydraHome := filepath.Join(base, ".hydra")
	return Scope{
		Global:   global,
		Label:    label,
		Base:     base,
		UserHome: home,
		Home:     hydraHome,
		RulesDir: filepath.Join(hydraHome, "rules"),
	}
}

// RuleRef renders a rule's path as it should appear in an instruction file.
// Global scope must be absolute: ~/.claude/CLAUDE.md is loaded from whatever
// working directory the agent is in, so a relative path resolves against the
// wrong repo. Project scope stays relative so it survives a clone elsewhere.
func (s Scope) RuleRef(r Rule) string {
	return s.ref(r.Path)
}

// RulesDirRef renders the library directory for the grep hint.
func (s Scope) RulesDirRef() string {
	return s.ref(s.RulesDir)
}

func (s Scope) ref(path string) string {
	if s.Global {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(s.Base, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return strings.TrimPrefix(filepath.ToSlash(rel), "./")
}

// scopeFromCmd resolves the active Scope honoring the persistent --global flag.
// It lives here rather than in main.go so every command file can reach it
// without depending on the order commands get wired up.
//
// The directory lookup is only fatal for the scope that actually needs it, but
// it must be fatal: filepath.Join("", ".hydra") is ".hydra", so swallowing a
// failed UserHomeDir would silently point --global at the current repository
// and scaffold there while reporting that it worked on the global scope.
func scopeFromCmd(cmd *cobra.Command) (Scope, error) {
	global, _ := cmd.Flags().GetBool("global")

	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return Scope{}, fmt.Errorf("cannot resolve your home directory for --global (is $HOME set?): %w", err)
		}
		return ResolveScope(true, "", home), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Scope{}, fmt.Errorf("cannot resolve the current directory: %w", err)
	}
	return ResolveScope(false, cwd, ""), nil
}
