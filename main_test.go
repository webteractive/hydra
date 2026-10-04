package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain walls the suite off from the developer's real libraries before any
// test runs, and refuses to run at all if the wall did not hold.
//
// hydra resolves everything it touches from two variables: HOME (through
// os.UserHomeDir — ~/.claude, ~/.codex, ~/.agents and the default ~/.hydra all
// hang off it) and HYDRA_HOME. A test that sets neither inherits the shell's,
// and `relocate` moves the library it resolves into a t.TempDir that is
// deleted when the test ends. With HYDRA_HOME pointing at a real library, one
// forgotten t.Setenv deleted that library. So the suite starts from a
// throwaway HOME and no HYDRA_HOME, and every test that touches either still
// sets both (isolateHome).
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "hydra-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "hydra tests: cannot create an isolated HOME:", err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Unsetenv(hydraHomeEnv)

	if err := checkIsolated(); err != nil {
		fmt.Fprintln(os.Stderr, "hydra tests: refusing to run:", err)
		os.RemoveAll(home)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// checkIsolated reports a HOME or HYDRA_HOME that resolves anywhere but the
// temp directory. Paths are compared after resolving symlinks, so a temp dir
// reached through /var → /private/var on macOS still counts as temp.
func checkIsolated() error {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return fmt.Errorf("cannot resolve the temp directory: %w", err)
	}
	inTemp := func(path string) bool {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = filepath.Clean(path)
		}
		return strings.HasPrefix(real, tmp+string(filepath.Separator))
	}

	home, err := os.UserHomeDir()
	if err != nil || !inTemp(home) {
		return fmt.Errorf("HOME resolves to %q, outside %s", home, tmp)
	}
	if v, ok := os.LookupEnv(hydraHomeEnv); ok && strings.TrimSpace(v) != "" && !inTemp(v) {
		return fmt.Errorf("%s resolves to %q, outside %s", hydraHomeEnv, v, tmp)
	}
	return nil
}

// isolateHome points HOME at home and clears HYDRA_HOME for one test, so the
// global library resolves to <home>/.hydra and nowhere else. Every test that
// sets HOME goes through it; a test that wants a moved library sets HYDRA_HOME
// again afterwards.
func isolateHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, "")
}

// The wall is the only thing between a careless test and real data, so it is
// tested like anything else.
func TestSuiteNeverSeesTheDevelopersLibrary(t *testing.T) {
	if v, ok := os.LookupEnv(hydraHomeEnv); ok {
		t.Errorf("%s leaked into the suite: %q", hydraHomeEnv, v)
	}
	if err := checkIsolated(); err != nil {
		t.Error(err)
	}
}

func TestCheckIsolatedRefusesARealLibrary(t *testing.T) {
	t.Setenv(hydraHomeEnv, "/Users/someone/AI/dotfiles/hydra")
	if err := checkIsolated(); err == nil {
		t.Error("a HYDRA_HOME outside the temp directory must be refused")
	}
	t.Setenv(hydraHomeEnv, "")
	t.Setenv("HOME", "/Users/someone")
	if err := checkIsolated(); err == nil {
		t.Error("a HOME outside the temp directory must be refused")
	}
}
