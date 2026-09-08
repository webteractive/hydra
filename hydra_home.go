package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// hydraHomeEnv names the environment variable that relocates the global hydra
// home. It follows the CARGO_HOME / RUSTUP_HOME convention: the variable names
// the library directory itself, not the parent it would be created in.
const hydraHomeEnv = "HYDRA_HOME"

// hydraHomeFlagName is the per-invocation escape hatch, mainly for probing
// another library without touching the environment. hydraHomeFlag is the same
// flag as it appears in error messages.
const (
	hydraHomeFlagName = "hydra-home"
	hydraHomeFlag     = "--" + hydraHomeFlagName
)

// Where a resolved home came from, reported by doctor so an override that
// silently stopped applying is visible rather than mysterious.
const (
	hydraHomeSourceDefault = "default"
	hydraHomeSourceEnv     = hydraHomeEnv
	hydraHomeSourceFlag    = hydraHomeFlag
)

// resolveHydraHome picks the global hydra home: the flag wins over the
// environment, which wins over <userHome>/.hydra. It returns the path and the
// source that produced it.
//
// An override must be absolute. Resolving a relative one against the working
// directory would scaffold a *global* library inside whatever repository
// happened to be current and then report success — the same failure mode
// scopeFromCmd refuses to swallow when the home directory cannot be resolved.
func resolveHydraHome(flag, env, userHome string) (string, string, error) {
	if v := strings.TrimSpace(flag); v != "" {
		return absoluteHydraHome(v, userHome, hydraHomeSourceFlag)
	}
	if v := strings.TrimSpace(env); v != "" {
		return absoluteHydraHome(v, userHome, hydraHomeSourceEnv)
	}
	if userHome == "" {
		return "", "", fmt.Errorf("cannot resolve your home directory (is $HOME set?); set %s to an absolute path to say where the hydra library lives", hydraHomeEnv)
	}
	return filepath.Join(userHome, ".hydra"), hydraHomeSourceDefault, nil
}

func absoluteHydraHome(path, userHome, source string) (string, string, error) {
	expanded, err := expandTilde(path, userHome)
	if err != nil {
		return "", "", fmt.Errorf("%s %q: %w", source, path, err)
	}
	if !filepath.IsAbs(expanded) {
		return "", "", fmt.Errorf("%s %q must be an absolute path", source, path)
	}
	return filepath.Clean(expanded), source, nil
}

// expandTilde resolves a leading ~ against userHome. A HYDRA_HOME set anywhere
// but an interactive shell — a launchd plist, a systemd unit, a CI config — is
// never expanded for us, and a directory literally named "~" is never what the
// author meant.
func expandTilde(path, userHome string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	if userHome == "" {
		return "", fmt.Errorf("cannot expand ~ without a home directory (is $HOME set?)")
	}
	if path == "~" {
		return userHome, nil
	}
	return filepath.Join(userHome, strings.TrimPrefix(path, "~/")), nil
}

// hydraHomeOverridden reports whether either override was actually stated.
func hydraHomeOverridden(flag, env string) bool {
	return strings.TrimSpace(flag) != "" || strings.TrimSpace(env) != ""
}
