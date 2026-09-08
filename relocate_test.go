package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// globalScopeAt builds the resolved global scope hydra would hand Relocate for
// a library living at src, with home as the user's home directory.
func globalScopeAt(home, src string) Scope {
	return ResolveScopeIn(true, "", home, src)
}

// seedLibrary writes a minimal but realistic global library at src: an authored
// rule, an authored ability, and the wired instruction file that references it.
func seedLibrary(t *testing.T, home, src string) {
	t.Helper()
	mustWrite(t, filepath.Join(src, "rules", "rust.md"),
		"---\ntitle: Rust\npaths:\n  - '**/Cargo.toml'\n---\n\n- Pin versions.\n")
	mustWrite(t, filepath.Join(src, "abilities", "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers:\n  - ship the thing\n---\n\n# Shipper\n")
	s := globalScopeAt(home, src)
	if err := Init(s, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestRelocateMovesRulesAndAbilities(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	seedLibrary(t, home, src)

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "rules", "rust.md")) {
		t.Error("rules did not move")
	}
	if !exists(filepath.Join(dest, "abilities", "shipper", abilityFilename)) {
		t.Error("abilities did not move")
	}
	if exists(src) {
		t.Error("the source library should be gone after a move")
	}
}

// A block still naming the old directory sends every agent to a path that no
// longer exists, so rewriting it is part of the move, not a follow-up.
func TestRelocateRewritesTheGlobalBlock(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	seedLibrary(t, home, src)

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(got, filepath.Join(dest, "rules")) {
		t.Errorf("block does not reference the new rules library:\n%s", got)
	}
	if !strings.Contains(got, filepath.Join(dest, "abilities")) {
		t.Errorf("block does not reference the new abilities library:\n%s", got)
	}
	if strings.Contains(got, src) {
		t.Errorf("block still references the old library:\n%s", got)
	}
}

func TestRelocateCreatesMissingParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "a", "b", "c")
	seedLibrary(t, home, src)

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "rules", "rust.md")) {
		t.Error("library did not move into the nested destination")
	}
}

// Merging into an occupied directory would interleave two libraries with no way
// to tell them apart afterwards.
func TestRelocateRefusesANonEmptyDestination(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	seedLibrary(t, home, src)
	mustWrite(t, filepath.Join(dest, "occupied.md"), "mine\n")

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err == nil {
		t.Fatal("expected an error for a non-empty destination")
	}
	if !exists(filepath.Join(src, "rules", "rust.md")) {
		t.Error("a refused move must leave the source untouched")
	}
	if readFile(t, filepath.Join(dest, "occupied.md")) != "mine\n" {
		t.Error("a refused move must leave the destination untouched")
	}
}

func TestRelocateAcceptsAnEmptyDestinationDirectory(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	seedLibrary(t, home, src)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dest, "rules", "rust.md")) {
		t.Error("library did not move into the empty destination")
	}
}

// Moving a directory into itself either loops or destroys it, depending on how
// the copy is written. Neither is acceptable, so it is refused up front.
func TestRelocateRefusesADestinationInsideTheSource(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	seedLibrary(t, home, src)

	if err := Relocate(globalScopeAt(home, src), filepath.Join(src, "nested"), io.Discard); err == nil {
		t.Fatal("expected an error for a destination inside the source")
	}
	if !exists(filepath.Join(src, "rules", "rust.md")) {
		t.Error("a refused move must leave the source untouched")
	}
}

func TestRelocateIsANoOpWhenTheLibraryIsAlreadyThere(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	seedLibrary(t, home, src)

	var out bytes.Buffer
	if err := Relocate(globalScopeAt(home, src), src, &out); err != nil {
		t.Fatalf("relocating to the current location should succeed: %v", err)
	}
	if !exists(filepath.Join(src, "rules", "rust.md")) {
		t.Fatal("the library was disturbed by a no-op relocate")
	}
	if !strings.Contains(out.String(), "already") {
		t.Errorf("a no-op should say so, got: %q", out.String())
	}
}

func TestRelocateErrorsWhenThereIsNoLibrary(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "missing")

	err := Relocate(globalScopeAt(home, src), filepath.Join(tmp, "dest"), io.Discard)
	if err == nil {
		t.Fatal("expected an error when there is no library to relocate")
	}
	if !strings.Contains(err.Error(), "hydra init") {
		t.Errorf("the error should point at init, got: %v", err)
	}
}

// The export line is the half of the job the command cannot do itself, so it
// has to be unmissable and correct.
func TestRelocatePrintsTheExportLine(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	seedLibrary(t, home, src)

	var out bytes.Buffer
	if err := Relocate(globalScopeAt(home, src), dest, &out); err != nil {
		t.Fatal(err)
	}
	want := `export ` + hydraHomeEnv + `="` + dest + `"`
	if !strings.Contains(out.String(), want) {
		t.Errorf("output should contain %q, got:\n%s", want, out.String())
	}
	if !strings.Contains(out.String(), src) {
		t.Errorf("output should name where the library came from, got:\n%s", out.String())
	}
}

// os.Rename fails across filesystems, which is exactly the case this feature
// invites: ~ on one volume, a dotfiles repo on another.
func TestCopyTreeReproducesFilesDirectoriesAndModes(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")
	mustWrite(t, filepath.Join(src, "top.md"), "top\n")
	mustWrite(t, filepath.Join(src, "nested", "deep", "leaf.md"), "leaf\n")
	if err := os.Chmod(filepath.Join(src, "top.md"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyTree(src, dest); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dest, "top.md")); got != "top\n" {
		t.Errorf("top.md = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "nested", "deep", "leaf.md")); got != "leaf\n" {
		t.Errorf("leaf.md = %q", got)
	}
	fi, err := os.Stat(filepath.Join(dest, "top.md"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v want 0600", fi.Mode().Perm())
	}
}

// A destination typed at a prompt is unambiguous in a way an inherited
// environment variable is not, so a relative one resolves against the cwd.
func TestResolveRelocateDestination(t *testing.T) {
	got, err := resolveRelocateDest("library", "/work/app", "/home/u")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/work/app/library" {
		t.Errorf("dest = %s want /work/app/library", got)
	}

	if got, err = resolveRelocateDest("~/dotfiles/hydra", "/work/app", "/home/u"); err != nil {
		t.Fatal(err)
	}
	if got != "/home/u/dotfiles/hydra" {
		t.Errorf("dest = %s want /home/u/dotfiles/hydra", got)
	}

	if got, err = resolveRelocateDest("/srv/library", "/work/app", "/home/u"); err != nil {
		t.Fatal(err)
	}
	if got != "/srv/library" {
		t.Errorf("dest = %s want /srv/library", got)
	}
}

// `hydra ability init` alone creates abilities/ with no rules/ — the documented
// path for an existing installation opting into abilities. Relocating that
// library must not fail on the rules sync it does not need.
func TestRelocateHandlesAnAbilitiesOnlyLibrary(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")

	mustWrite(t, filepath.Join(src, "abilities", "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers:\n  - ship the thing\n---\n\n# Shipper\n")
	if err := AbilityInit(ResolveAbilityScopeIn(home, src), io.Discard); err != nil {
		t.Fatal(err)
	}

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatalf("relocating an abilities-only library should succeed: %v", err)
	}
	if !exists(filepath.Join(dest, "abilities", "shipper", abilityFilename)) {
		t.Error("abilities did not move")
	}
	got := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(got, filepath.Join(dest, "abilities")) {
		t.Errorf("abilities block not rewritten to the new library:\n%s", got)
	}
}

// The mirror case: a rules-only library, which is what --global has always
// produced on its own.
func TestRelocateHandlesARulesOnlyLibrary(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dest")

	mustWrite(t, filepath.Join(src, "rules", "rust.md"),
		"---\ntitle: Rust\npaths:\n  - '**/Cargo.toml'\n---\n\n- Pin versions.\n")
	if err := Sync(globalScopeAt(home, src), io.Discard); err != nil {
		t.Fatal(err)
	}

	if err := Relocate(globalScopeAt(home, src), dest, io.Discard); err != nil {
		t.Fatalf("relocating a rules-only library should succeed: %v", err)
	}
	if !exists(filepath.Join(dest, "rules", "rust.md")) {
		t.Error("rules did not move")
	}
}
