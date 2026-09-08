package main

import (
	"strings"
	"testing"
)

func TestResolveHydraHomeDefaultsToDotHydraInUserHome(t *testing.T) {
	got, src, err := resolveHydraHome("", "", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/u/.hydra" {
		t.Errorf("home = %s want /home/u/.hydra", got)
	}
	if src != hydraHomeSourceDefault {
		t.Errorf("source = %s want %s", src, hydraHomeSourceDefault)
	}
}

func TestResolveHydraHomeHonorsEnv(t *testing.T) {
	got, src, err := resolveHydraHome("", "/srv/library", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/srv/library" {
		t.Errorf("home = %s want /srv/library", got)
	}
	if src != hydraHomeSourceEnv {
		t.Errorf("source = %s want %s", src, hydraHomeSourceEnv)
	}
}

func TestResolveHydraHomeFlagBeatsEnv(t *testing.T) {
	got, src, err := resolveHydraHome("/tmp/probe", "/srv/library", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/probe" {
		t.Errorf("home = %s want /tmp/probe", got)
	}
	if src != hydraHomeSourceFlag {
		t.Errorf("source = %s want %s", src, hydraHomeSourceFlag)
	}
}

// A HYDRA_HOME set outside a shell — a launchd plist, a systemd unit, a CI
// config — never gets tilde expansion, so hydra does it rather than scaffolding
// a directory literally named "~".
func TestResolveHydraHomeExpandsLeadingTilde(t *testing.T) {
	got, _, err := resolveHydraHome("", "~/dotfiles/hydra", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/u/dotfiles/hydra" {
		t.Errorf("home = %s want /home/u/dotfiles/hydra", got)
	}
}

func TestResolveHydraHomeTreatsBareTildeAsUserHome(t *testing.T) {
	got, _, err := resolveHydraHome("", "~", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/u" {
		t.Errorf("home = %s want /home/u", got)
	}
}

func TestResolveHydraHomeIgnoresBlankEnv(t *testing.T) {
	got, src, err := resolveHydraHome("", "   ", "/home/u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/u/.hydra" {
		t.Errorf("home = %s want /home/u/.hydra", got)
	}
	if src != hydraHomeSourceDefault {
		t.Errorf("source = %s want %s", src, hydraHomeSourceDefault)
	}
}

// Resolving a relative override against the working directory would scaffold a
// global library inside whatever repository happened to be current, so it is an
// error rather than a silent guess.
func TestResolveHydraHomeRejectsRelativeEnv(t *testing.T) {
	_, _, err := resolveHydraHome("", "library", "/home/u")
	if err == nil {
		t.Fatal("expected an error for a relative HYDRA_HOME")
	}
	if !strings.Contains(err.Error(), "HYDRA_HOME") || !strings.Contains(err.Error(), "library") {
		t.Errorf("error should name the variable and the value, got: %v", err)
	}
}

func TestResolveHydraHomeRejectsRelativeFlag(t *testing.T) {
	_, _, err := resolveHydraHome("../library", "", "/home/u")
	if err == nil {
		t.Fatal("expected an error for a relative --hydra-home")
	}
	if !strings.Contains(err.Error(), "--hydra-home") {
		t.Errorf("error should name the flag, got: %v", err)
	}
}

func TestResolveHydraHomeRequiresUserHomeForTheDefault(t *testing.T) {
	if _, _, err := resolveHydraHome("", "", ""); err == nil {
		t.Fatal("expected an error when there is no home directory to default into")
	}
}

// An override means hydra no longer needs a home directory to find its library.
func TestResolveHydraHomeWithoutUserHomeAcceptsAnAbsoluteOverride(t *testing.T) {
	got, _, err := resolveHydraHome("", "/srv/library", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/srv/library" {
		t.Errorf("home = %s want /srv/library", got)
	}
}

func TestResolveHydraHomeCannotExpandTildeWithoutUserHome(t *testing.T) {
	if _, _, err := resolveHydraHome("", "~/library", ""); err == nil {
		t.Fatal("expected an error expanding ~ with no home directory")
	}
}
