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
	// GlobalHome is the global hydra library, whether or not this scope is the
	// global one. Abilities are always global, so a project scope still has to
	// carry it — see abilityScopeFromRuleScope.
	GlobalHome string `json:"global_home"`
	// GlobalHomeSource names what put GlobalHome where it is, so doctor can
	// report an override that has silently stopped applying.
	GlobalHomeSource string `json:"global_home_source,omitempty"`
}

func ResolveScope(global bool, cwd, home string) Scope {
	return ResolveScopeIn(global, cwd, home, "")
}

// ResolveScopeIn is ResolveScope with the global library relocated to
// globalHome (empty means the default, <home>/.hydra).
//
// Only the library moves. Base stays the home directory, because that is where
// ~/.claude/CLAUDE.md lives and detect.go finds instruction files through Base
// — relocating those too would hide the block from the agents that read it.
// Project scope ignores globalHome outright: a project rule travels with its
// repository, which is the whole reason the scope exists.
func ResolveScopeIn(global bool, cwd, home, globalHome string) Scope {
	// An empty home with no override leaves GlobalHome empty rather than
	// resolving to a bare ".hydra": a project command that never touches
	// abilities still works, and one that does fails in
	// abilityScopeFromRuleScope with a message about $HOME.
	if globalHome == "" && home != "" {
		globalHome = filepath.Join(home, ".hydra")
	}
	base, label := cwd, "project"
	hydraHome := filepath.Join(cwd, ".hydra")
	if global {
		base, label, hydraHome = home, "global", globalHome
	}
	return Scope{
		Global:     global,
		Label:      label,
		Base:       base,
		UserHome:   home,
		Home:       hydraHome,
		RulesDir:   filepath.Join(hydraHome, "rules"),
		GlobalHome: globalHome,
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

// hydraHomeOverrides reads the two overrides for this invocation, so the flag
// and the environment variable are never looked up from separate places.
func hydraHomeOverrides(cmd *cobra.Command) (flag, env string) {
	flag, _ = cmd.Flags().GetString(hydraHomeFlagName)
	return flag, os.Getenv(hydraHomeEnv)
}

// globalScopeFromCmd resolves the global Scope regardless of --global, for the
// commands that only ever operate on the global library.
//
// The home directory is required even when the library has moved: only the
// library moves, and ~/.claude/CLAUDE.md still has to be found and written.
func globalScopeFromCmd(cmd *cobra.Command) (Scope, error) {
	flagHome, envHome := hydraHomeOverrides(cmd)
	home, err := os.UserHomeDir()
	if err != nil {
		return Scope{}, fmt.Errorf("cannot resolve your home directory for the global scope (is $HOME set?): %w", err)
	}
	globalHome, source, err := resolveHydraHome(flagHome, envHome, home)
	if err != nil {
		return Scope{}, err
	}
	s := ResolveScopeIn(true, "", home, globalHome)
	s.GlobalHomeSource = source
	return s, nil
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
		return globalScopeFromCmd(cmd)
	}

	flagHome, envHome := hydraHomeOverrides(cmd)
	cwd, err := os.Getwd()
	if err != nil {
		return Scope{}, fmt.Errorf("cannot resolve the current directory: %w", err)
	}

	// A project scope needs no home directory for its own library, but it
	// carries the global one so init can wire abilities. A stated override is
	// resolved strictly — a bad one must be reported, never ignored — while a
	// missing $HOME is deferred to whoever actually needs abilities.
	home, _ := os.UserHomeDir()
	var globalHome, source string
	if hydraHomeOverridden(flagHome, envHome) || home != "" {
		if globalHome, source, err = resolveHydraHome(flagHome, envHome, home); err != nil {
			return Scope{}, err
		}
	}
	s := ResolveScopeIn(false, cwd, home, globalHome)
	s.GlobalHomeSource = source
	return s, nil
}
