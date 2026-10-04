package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Relocate moves the global library from s.Home to dest and rewrites the
// managed blocks so they name the new location.
//
// Only the library moves. The instruction files stay under s.Base, which is why
// the blocks have to be rewritten rather than moved: they hold absolute paths
// into a directory that is about to stop existing.
//
// It cannot finish the job on its own. HYDRA_HOME lives in a shell profile hydra
// has no business editing, so the last thing it does is print the line to add.
func Relocate(s Scope, dest string, out io.Writer) error {
	// Relocating a library the blocks do not name would rewire them away from
	// the one the agents are reading and hide it.
	if err := guardRules(s); err != nil {
		return err
	}
	if err := guardAbilities(abilityScopeOf(s)); err != nil {
		return err
	}

	src := s.Home
	if !isDir(src) {
		return fmt.Errorf("no hydra library at %s — run 'hydra init --global' first", src)
	}

	src, dest = filepath.Clean(src), filepath.Clean(dest)
	if src == dest {
		fmt.Fprintf(out, "the hydra library is already at %s — nothing to do\n", dest)
		return nil
	}
	if within(dest, src) {
		return fmt.Errorf("cannot relocate %s into itself (%s)", src, dest)
	}
	if err := destinationIsFree(dest); err != nil {
		return err
	}

	if err := move(src, dest); err != nil {
		return err
	}
	fmt.Fprintf(out, "moved %s -> %s\n", src, dest)

	// From here the files are safely at dest; only the wiring can still fail,
	// and a stale block is fixable with one command, so the error names it.
	// Each half is synced only if it is there: `hydra ability init` alone
	// produces an abilities-only library, and --global on its own a rules-only
	// one, and syncing the missing half would fail a move that succeeded.
	// The blocks still name the old place — that is what is being fixed — so
	// the rewire must not trip the guard.
	moved := ResolveScopeIn(true, "", s.UserHome, dest)
	moved.Force = true
	abilities := ResolveAbilityScopeIn(s.UserHome, dest)
	abilities.Force = true
	if isDir(moved.RulesDir) {
		if err := Sync(moved, out); err != nil {
			return rewireFailed(dest, err)
		}
	}
	if isDir(abilities.AbilitiesDir) {
		if err := AbilitySync(abilities, out); err != nil {
			return rewireFailed(dest, err)
		}
	}

	fmt.Fprintf(out, "\nAdd this to your shell profile to make it stick:\n  export %s=%q\n", hydraHomeEnv, dest)
	return nil
}

// abilityScopeOf is the abilities half of a global scope, for the guard.
func abilityScopeOf(s Scope) AbilityScope {
	as := ResolveAbilityScopeIn(s.UserHome, s.Home)
	as.HydraHomeSource = s.GlobalHomeSource
	as.Force = s.Force
	return as
}

func rewireFailed(dest string, err error) error {
	return fmt.Errorf("the library moved to %s but its managed blocks were not rewritten: %w\n"+
		"finish with: hydra sync --global --hydra-home %s", dest, err, dest)
}

// destinationIsFree accepts a path that does not exist or is an empty
// directory. Merging into an occupied one would interleave two libraries with
// no way to tell them apart afterwards.
func destinationIsFree(dest string) error {
	entries, err := os.ReadDir(dest)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s is not a usable destination: %w", dest, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty — move or remove it first", dest)
	}
	return nil
}

// within reports whether path sits inside dir.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// move renames src to dest, falling back to copy-then-remove when the two are
// on different filesystems — the case this feature invites, with a home
// directory on one volume and a dotfiles repository on another.
//
// The fallback copies in full before removing anything: a copy that fails
// partway cleans up after itself and leaves the source where it was.
func move(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// destinationIsFree has already proved dest is empty if it exists at all.
	// Clear it, because rename onto an existing directory is an error on macOS
	// even when it is empty.
	os.Remove(dest)
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	// EXDEV ("cross-device link") is the one failure worth retrying as a copy;
	// anything else is a real problem and is reported as one.
	if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("cannot move %s to %s: %w", src, dest, err)
	}

	if err := copyTree(src, dest); err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("cannot copy %s to %s: %w", src, dest, err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("copied to %s but could not remove %s: %w", dest, src, err)
	}
	return nil
}

// copyTree recreates the tree rooted at src under dest, preserving file modes.
func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			// Only reachable on the cross-device path; a same-filesystem
			// rename moves anything. Say what to do rather than just what
			// went wrong.
			return fmt.Errorf("cannot copy %s across filesystems: not a regular file — move the library by hand, then run 'hydra sync --global --hydra-home <path>'", path)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dest string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// resolveRelocateDest turns the destination argument into an absolute path.
// Unlike HYDRA_HOME, a relative one is fine here: it was typed at a prompt from
// a known directory, which is how mv and cp behave.
func resolveRelocateDest(arg, cwd, userHome string) (string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", fmt.Errorf("relocate needs a destination path")
	}
	expanded, err := expandTilde(arg, userHome)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	return filepath.Join(cwd, expanded), nil
}
