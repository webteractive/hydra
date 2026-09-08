package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// AbilityScope is intentionally global-only. It is separate from Scope so a
// future project ability feature cannot accidentally inherit rule precedence.
type AbilityScope struct {
	UserHome     string `json:"user_home"`
	HydraHome    string `json:"hydra_home"`
	AbilitiesDir string `json:"abilities_dir"`
	// HydraHomeSource names what put HydraHome where it is; see
	// Scope.GlobalHomeSource.
	HydraHomeSource string `json:"hydra_home_source,omitempty"`
}

func ResolveAbilityScope(home string) AbilityScope {
	return ResolveAbilityScopeIn(home, "")
}

// ResolveAbilityScopeIn is ResolveAbilityScope with the library relocated to
// hydraHome (empty means the default, <home>/.hydra).
func ResolveAbilityScopeIn(home, hydraHome string) AbilityScope {
	if hydraHome == "" {
		hydraHome = filepath.Join(home, ".hydra")
	}
	return AbilityScope{
		UserHome:     home,
		HydraHome:    hydraHome,
		AbilitiesDir: filepath.Join(hydraHome, "abilities"),
	}
}

func abilityScopeFromRuleScope(s Scope) (AbilityScope, error) {
	home := s.UserHome
	if s.Global {
		home = s.Base
	}
	// A relocated library does not excuse a missing home directory: UserHome
	// still drives the harness paths (~/.claude/CLAUDE.md and the routers), so
	// an empty one would wire abilities into /.claude.
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return AbilityScope{}, fmt.Errorf("cannot resolve your home directory for abilities (is $HOME set?): %w", err)
		}
	}
	as := ResolveAbilityScopeIn(home, s.GlobalHome)
	as.HydraHomeSource = s.GlobalHomeSource
	return as, nil
}

func abilityScopeFromCmd(cmd *cobra.Command) (AbilityScope, error) {
	// Abilities are always global, so --hydra-home moves them whether or not
	// --global was passed. The home directory stays required: the harness
	// wiring lives under it even when the catalog does not.
	home, err := os.UserHomeDir()
	if err != nil {
		return AbilityScope{}, fmt.Errorf("cannot resolve your home directory for abilities (is $HOME set?): %w", err)
	}
	flagHome, envHome := hydraHomeOverrides(cmd)
	hydraHome, source, err := resolveHydraHome(flagHome, envHome, home)
	if err != nil {
		return AbilityScope{}, err
	}
	s := ResolveAbilityScopeIn(home, hydraHome)
	s.HydraHomeSource = source
	return s, nil
}
