package main

import (
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
func scopeFromCmd(cmd *cobra.Command) Scope {
	global, _ := cmd.Flags().GetBool("global")
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return ResolveScope(global, cwd, home)
}
