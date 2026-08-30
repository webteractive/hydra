package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// AbilityScope is intentionally global-only. It is separate from Scope so a
// future project ability feature cannot accidentally inherit rule precedence.
type AbilityScope struct {
	UserHome     string `json:"user_home"`
	HydraHome    string `json:"hydra_home"`
	AbilitiesDir string `json:"abilities_dir"`
}

func ResolveAbilityScope(home string) AbilityScope {
	hydraHome := filepath.Join(home, ".hydra")
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
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return AbilityScope{}, fmt.Errorf("cannot resolve your home directory for abilities (is $HOME set?): %w", err)
		}
	}
	return ResolveAbilityScope(home), nil
}

func abilityScopeFromCmd() (AbilityScope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return AbilityScope{}, fmt.Errorf("cannot resolve your home directory for abilities (is $HOME set?): %w", err)
	}
	return ResolveAbilityScope(home), nil
}
