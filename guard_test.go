package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// guardFixture wires a global library at lib through HYDRA_HOME, so the managed
// blocks in home name it, then clears HYDRA_HOME — the shell that lost the
// export, where every global write used to repoint the blocks at ~/.hydra.
func guardFixture(t *testing.T) (home, lib string) {
	t.Helper()
	tmp := t.TempDir()
	home = filepath.Join(tmp, "home")
	lib = filepath.Join(tmp, "dotfiles", "hydra")
	isolateHome(t, home)
	t.Chdir(tmp)
	t.Setenv(hydraHomeEnv, lib)
	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(hydraHomeEnv, "")
	return home, lib
}

func wantRefusal(t *testing.T, out string, err error, home, lib string) {
	t.Helper()
	if exitCode(err) != 2 {
		t.Fatalf("want exit 2, got %v\n%s", err, out)
	}
	msg := err.Error()
	for _, want := range []string{lib, filepath.Join(home, ".hydra"), `export HYDRA_HOME="` + lib + `"`, "--force"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal should mention %q:\n%s", want, msg)
		}
	}
}

func TestGlobalWritesRefuseABlockNamingAnotherLibrary(t *testing.T) {
	for _, args := range [][]string{
		{"sync", "--global"},
		{"init", "--global"},
		{"add", "--global", "--command", "make", "--title", "Make", "--note", "Use make."},
		{"new", "--global", "make"},
		{"ability", "sync"},
		{"ability", "init"},
		{"ability", "new", "shipper"},
		{"init"}, // a project init wires the global abilities block too
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			home, lib := guardFixture(t)
			claude := filepath.Join(home, ".claude", "CLAUDE.md")
			before := readFile(t, claude)

			out, err := runCLI(t, args...)
			wantRefusal(t, out, err, home, lib)

			if readFile(t, claude) != before {
				t.Error("a refused run must leave the managed blocks alone")
			}
			if exists(filepath.Join(home, ".hydra")) {
				t.Error("a refused run must not scaffold a library at the default location")
			}
		})
	}
}

func TestForceRewritesTheBlockToTheResolvedLibrary(t *testing.T) {
	home, lib := guardFixture(t)
	if _, err := runCLI(t, "init", "--global", "--force"); err != nil {
		t.Fatal(err)
	}
	claude := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(claude, filepath.Join(home, ".hydra", "rules")) || strings.Contains(claude, lib) {
		t.Errorf("--force should point both blocks at the resolved library:\n%s", claude)
	}
}

func TestGlobalWritesProceedWhenTheLibrariesAgree(t *testing.T) {
	_, lib := guardFixture(t)
	t.Setenv(hydraHomeEnv, lib)
	for _, args := range [][]string{{"sync", "--global"}, {"ability", "sync"}, {"init"}} {
		if out, err := runCLI(t, args...); err != nil {
			t.Errorf("%v: %v\n%s", args, err, out)
		}
	}
}

// relocate rewrites the blocks away from the old place by design, so its own
// rewire must not trip the guard; but relocating a library the blocks do not
// name would hide the one they do.
func TestRelocateRewiresButRefusesTheWrongSource(t *testing.T) {
	home, lib := guardFixture(t)
	t.Setenv(hydraHomeEnv, lib)
	dest := filepath.Join(filepath.Dir(lib), "moved")
	if out, err := runCLI(t, "relocate", dest); err != nil {
		t.Fatalf("relocate: %v\n%s", err, out)
	}
	if claude := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md")); !strings.Contains(claude, dest) {
		t.Errorf("relocate should rewire the blocks to %s", dest)
	}

	stale := filepath.Join(filepath.Dir(lib), "stale")
	mustWrite(t, filepath.Join(stale, "rules", "index.md"), "#\n")
	t.Setenv(hydraHomeEnv, stale)
	_, err := runCLI(t, "relocate", filepath.Join(filepath.Dir(lib), "elsewhere"))
	if exitCode(err) != 2 || !strings.Contains(err.Error(), dest) {
		t.Errorf("relocating a library the blocks do not name should be refused naming %s, got %v", dest, err)
	}
	if !exists(filepath.Join(stale, "rules", "index.md")) {
		t.Error("a refused relocate must not move anything")
	}
}

func TestDoctorAdvisesHydraHomeWhenTheBlockNamesAnotherLibrary(t *testing.T) {
	home, lib := guardFixture(t)

	// Nothing at the default location: the advice is the variable, never init.
	rep := Doctor(globalScopeForTest(t, home))
	c, _ := checkByPrefix(rep, "rules directory present")
	if c.OK || !strings.Contains(c.Detail, "HYDRA_HOME="+lib) || len(c.Fix) != 0 {
		t.Errorf("missing library: %+v, want HYDRA_HOME=%s advice and no fix argv", c, lib)
	}
	ab := AbilityDoctor(ResolveAbilityScope(home))
	c, _ = checkByPrefix(ab, "abilities directory present")
	if c.OK || !strings.Contains(c.Detail, "HYDRA_HOME="+lib) || len(c.Fix) != 0 {
		t.Errorf("missing abilities: %+v", c)
	}

	// A stale library at the default location: the blocks still name the other.
	mustWrite(t, filepath.Join(home, ".hydra", "rules", "index.md"), "#\n")
	mustWrite(t, filepath.Join(home, ".hydra", "abilities", "index.md"), "#\n")
	for _, r := range []DoctorReport{Doctor(globalScopeForTest(t, home)), AbilityDoctor(ResolveAbilityScope(home))} {
		named, ok := checkByPrefix(r, "block names this library in")
		if !ok || named.OK || !strings.Contains(named.Detail, "HYDRA_HOME="+lib) || !strings.Contains(named.Detail, "--force") {
			t.Errorf("%s: want a failing names-this-library check, got %+v", r.Scope, named)
		}
		for _, check := range r.Checks {
			if strings.Join(check.Fix, " ") == "hydra sync --global" || strings.Join(check.Fix, " ") == "hydra ability sync" {
				t.Errorf("%s: %q advises a sync the guard would refuse", r.Scope, check.Name)
			}
		}
	}
}

func globalScopeForTest(t *testing.T, home string) Scope {
	t.Helper()
	s := ResolveScope(true, "", home)
	s.GlobalHomeSource = hydraHomeSourceDefault
	return s
}
