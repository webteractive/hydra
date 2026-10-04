package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(args, &out, &out)
	return out.String(), err
}

func TestRunVersionHelp(t *testing.T) {
	out, err := runCLI(t, "version")
	if err != nil || !strings.Contains(out, "hydra "+version()) {
		t.Errorf("version: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "help"); err != nil || !strings.Contains(out, "Usage:") {
		t.Errorf("help: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "bogus"); err == nil {
		t.Error("expected an error for an unknown command")
	}
}

func TestRunLifecycleProject(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	isolateHome(t, filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "add",
		"--glob", "app/Http/Controllers/**",
		"--title", "Extend BaseController",
		"--note", "Every controller extends BaseController.",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "sync"); err != nil {
		t.Fatal(err)
	}

	claude := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if !strings.Contains(claude, ".hydra/rules/controllers.md") {
		t.Errorf("CLAUDE.md missing the indexed rule:\n%s", claude)
	}

	out, err := runCLI(t, "list")
	if err != nil || !strings.Contains(out, "Rules in project scope (1)") || !strings.Contains(out, "Files: app/Http/Controllers/**") {
		t.Errorf("list: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "list", "--json"); err != nil || !strings.Contains(out, `"name": "controllers"`) {
		t.Errorf("list --json: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "doctor"); err != nil {
		t.Fatalf("doctor should pass on a fresh install: %v", err)
	}
	if out, err := runCLI(t, "doctor", "--json"); err != nil || !strings.Contains(out, `"severity"`) {
		t.Errorf("doctor --json: out=%q err=%v", out, err)
	}
}

func TestRunAddInitializesFromScratch(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	isolateHome(t, filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "add", "--always",
		"--title", "Never commit automatically",
		"--note", "Ask before git commit.",
	); err != nil {
		t.Fatal(err)
	}
	claude := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if !strings.Contains(claude, "Ask before git commit.") {
		t.Errorf("always-rule not inlined:\n%s", claude)
	}
}

func TestRunGlobalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	isolateHome(t, home)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".claude", "CLAUDE.md")
	got := readFile(t, target)
	if !strings.Contains(got, filepath.Join(home, ".hydra", "rules")) {
		t.Errorf("global block should reference absolute paths:\n%s", got)
	}
	if _, err := runCLI(t, "doctor", "--global"); err != nil {
		t.Fatal(err)
	}
}

func TestRunSyncUninitializedFails(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	isolateHome(t, filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "sync"); err == nil {
		t.Error("sync on an uninitialized project should fail")
	}
}

func TestRunAbilityLifecycle(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	isolateHome(t, home)

	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "ability", "new", "testing-notes"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "ability", "sync"); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, "ability", "list"); err != nil || !strings.Contains(out, "Invoke: $ability testing-notes") {
		t.Errorf("ability list: out=%q err=%v", out, err)
	}
	if out, err := runCLI(t, "ability", "list", "--json"); err != nil || !strings.Contains(out, `"description"`) {
		t.Errorf("ability list --json: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "ability", "doctor"); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, "ability", "doctor", "--json"); err != nil || !strings.Contains(out, `"global abilities"`) {
		t.Errorf("ability doctor --json: out=%q err=%v", out, err)
	}
}

// filepath.Join("", ".hydra") is ".hydra", so a swallowed UserHomeDir failure
// would point --global at the current directory and scaffold there while
// reporting success. It must fail loudly instead.
func TestRunGlobalFailsWithoutHome(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	isolateHome(t, "")

	out, err := runCLI(t, "init", "--global")
	if err == nil {
		t.Fatalf("expected an error when the home directory cannot be resolved; out=%q", out)
	}
	if exists(filepath.Join(tmp, ".hydra")) {
		t.Error("a failed --global must not scaffold into the current directory")
	}
}

// A CLI must answer "which build is this?" without a subcommand.
func TestRootReportsVersionViaFlag(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		out, err := runCLI(t, flag)
		if err != nil {
			t.Errorf("%s: %v", flag, err)
		}
		if !strings.Contains(out, version()) {
			t.Errorf("%s: output %q should contain %q", flag, out, version())
		}
	}
	// The subcommand must keep working alongside the flag.
	out, err := runCLI(t, "version")
	if err != nil || !strings.Contains(out, version()) {
		t.Errorf("version subcommand: out=%q err=%v", out, err)
	}
}

func TestVersionFlagAndSubcommandAgree(t *testing.T) {
	flagOut, _ := runCLI(t, "--version")
	subOut, _ := runCLI(t, "version")
	if strings.TrimSpace(flagOut) != strings.TrimSpace(subOut) {
		t.Errorf("version surfaces disagree:\n  --version: %q\n  version:   %q", flagOut, subOut)
	}
}

// Triggers decide invocation, so a script reading the machine-readable output
// cannot reason about an ability without them.
func TestAbilityListJSONExposesTriggers(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(home, ".hydra", "abilities", "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers:\n  - ship the thing\n---\n\n# Shipper\n")
	if _, err := runCLI(t, "ability", "sync"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "ability", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var infos []AbilityInfo
	if err := json.Unmarshal([]byte(out), &infos); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(infos) != 1 || len(infos[0].Triggers) != 1 || infos[0].Triggers[0] != "ship the thing" {
		t.Errorf("triggers missing from machine-readable output: %+v", infos)
	}

	if text, err := runCLI(t, "ability", "list"); err != nil || !strings.Contains(text, "ship the thing") {
		t.Errorf("humans need the triggers too: out=%q err=%v", text, err)
	}
}

// The command listing is how anyone learns what a tool does.
func TestEveryCommandHasHelp(t *testing.T) {
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		if c.Name() != "hydra" && c.Name() != "completion" && c.Name() != "help" {
			if c.Short == "" {
				t.Errorf("%s: no Short description", path)
			}
			if c.Long == "" {
				t.Errorf("%s: no Long help — users cannot learn what it does or what its flags mean", path)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub, path+" "+sub.Name())
		}
	}
	walk(newRootCmd(io.Discard, io.Discard), "hydra")
}

func TestHelpDoesNotDescribeAbilitiesAsLazyLoaded(t *testing.T) {
	root := newRootCmd(io.Discard, io.Discard)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, text := range []string{c.Short, c.Long} {
			if strings.Contains(text, "lazy-loaded abilit") {
				t.Errorf("%s: help still describes the pre-fix model: %q", c.Name(), text)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// Rule: point each audience at the other in the output itself, so nobody has to
// guess the flag exists.
func TestHumanOutputPointsAtTheJSONFlag(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	project := t.TempDir()
	t.Chdir(project)

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(home, ".hydra", "abilities", "shipper", abilityFilename),
		"---\nname: shipper\ndescription: Ship it.\ntriggers:\n  - ship the thing\n---\n\n# Shipper\n")
	if _, err := runCLI(t, "ability", "sync"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"list"}, "hydra list --json"},
		{[]string{"list", "--global"}, "hydra list --global --json"},
		{[]string{"doctor"}, "hydra doctor --json"},
		{[]string{"doctor", "--global"}, "hydra doctor --global --json"},
		{[]string{"ability", "list"}, "hydra ability list --json"},
		{[]string{"ability", "doctor"}, "hydra ability doctor --json"},
		{[]string{"ability", "match", "ship the thing"}, "hydra ability match"},
	} {
		out, _ := runCLI(t, tc.args...)
		if !strings.Contains(out, tc.want) {
			t.Errorf("`hydra %s` never mentions %q:\n%s", strings.Join(tc.args, " "), tc.want, out)
		}
	}
}

// Every --json flag should describe itself the same way.
func TestJSONFlagsDescribeThemselvesConsistently(t *testing.T) {
	const want = "emit machine-readable JSON for agents and scripts"
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		if f := c.Flags().Lookup("json"); f != nil && f.Usage != want {
			t.Errorf("%s --json: usage %q, want %q", path, f.Usage, want)
		}
		for _, sub := range c.Commands() {
			walk(sub, path+" "+sub.Name())
		}
	}
	walk(newRootCmd(io.Discard, io.Discard), "hydra")
}

// HYDRA_HOME relocates the global library. The instruction file it wires must
// stay in the home directory, since that is where the agents look for it.
func TestRunGlobalScopeHonorsHydraHomeEnv(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	library := filepath.Join(tmp, "dotfiles", "hydra")
	t.Chdir(tmp)
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, library)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(library, "rules")) {
		t.Errorf("rules library not created at %s", library)
	}
	if exists(filepath.Join(home, ".hydra")) {
		t.Error("the default library must not be created when HYDRA_HOME is set")
	}
	got := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(got, filepath.Join(library, "rules")) {
		t.Errorf("global block should reference the relocated library:\n%s", got)
	}
	if strings.Contains(got, filepath.Join(home, ".hydra")) {
		t.Errorf("global block still references the default library:\n%s", got)
	}
}

func TestRunGlobalScopeHonorsHydraHomeFlag(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	library := filepath.Join(tmp, "elsewhere")
	t.Chdir(tmp)
	isolateHome(t, home)

	if _, err := runCLI(t, "init", "--global", "--hydra-home", library); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(library, "rules")) {
		t.Errorf("rules library not created at %s", library)
	}
}

func TestRunHydraHomeFlagBeatsTheEnvironment(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, filepath.Join(tmp, "from-env"))

	flagged := filepath.Join(tmp, "from-flag")
	if _, err := runCLI(t, "init", "--global", "--hydra-home", flagged); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(flagged, "rules")) {
		t.Errorf("library not created at the flagged path %s", flagged)
	}
	if exists(filepath.Join(tmp, "from-env")) {
		t.Error("the flag must win over HYDRA_HOME")
	}
}

// A relative override would scaffold a global library inside the current
// repository, so it fails loudly rather than guessing.
func TestRunRejectsRelativeHydraHome(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv(hydraHomeEnv, "library")

	out, err := runCLI(t, "init", "--global")
	if err == nil {
		t.Fatalf("expected an error for a relative HYDRA_HOME; out=%q", out)
	}
	if exists(filepath.Join(tmp, "library")) {
		t.Error("a rejected HYDRA_HOME must not scaffold into the current directory")
	}
}

// A project rule travels with its repository; nothing in the environment
// should be able to move it out.
func TestRunProjectScopeIgnoresHydraHome(t *testing.T) {
	tmp := t.TempDir()
	project := filepath.Join(tmp, "app")
	mustWrite(t, filepath.Join(project, "keep"), "")
	t.Chdir(project)
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv(hydraHomeEnv, filepath.Join(tmp, "dotfiles", "hydra"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(project, ".hydra", "rules")) {
		t.Error("project rules must stay in the project")
	}
	if isDir(filepath.Join(tmp, "dotfiles", "hydra", "rules")) {
		t.Error("HYDRA_HOME must not move the project rules library")
	}
}

// Abilities live in the same library, so they follow it — including from a
// project scope, which is where `hydra init` wires them from.
func TestRunAbilitiesFollowHydraHome(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	library := filepath.Join(tmp, "dotfiles", "hydra")
	t.Chdir(tmp)
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, library)

	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "ability", "new", "testing-notes"); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(library, "abilities", "testing-notes", abilityFilename)) {
		t.Errorf("ability not scaffolded under %s", library)
	}
	if exists(filepath.Join(home, ".hydra")) {
		t.Error("the default library must not be created when HYDRA_HOME is set")
	}
	if out, err := runCLI(t, "ability", "list"); err != nil || !strings.Contains(out, "testing-notes") {
		t.Errorf("ability list: out=%q err=%v", out, err)
	}
}

// An override that silently stopped applying would rewrite every managed block
// back to the default library, so doctor has to say where it is reading from.
func TestDoctorReportsWhereTheLibraryCameFrom(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	library := filepath.Join(tmp, "dotfiles", "hydra")
	t.Chdir(tmp)
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, library)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "doctor", "--global")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, library) || !strings.Contains(out, hydraHomeEnv) {
		t.Errorf("doctor should name the library and its source:\n%s", out)
	}

	out, err = runCLI(t, "doctor", "--global", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rep DoctorReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.HomeSource != hydraHomeSourceEnv {
		t.Errorf("home_source = %q want %q", rep.HomeSource, hydraHomeSourceEnv)
	}
	if rep.Home != library {
		t.Errorf("home = %q want %q", rep.Home, library)
	}
}

func TestDoctorReportsTheDefaultLibrarySource(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Chdir(tmp)
	isolateHome(t, home)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "doctor", "--global", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rep DoctorReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.HomeSource != hydraHomeSourceDefault {
		t.Errorf("home_source = %q want %q", rep.HomeSource, hydraHomeSourceDefault)
	}
}

// The rules library is not what "home source" describes in a project scope —
// reporting HYDRA_HOME there would be a lie.
func TestDoctorOmitsTheLibrarySourceForProjectScope(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv(hydraHomeEnv, filepath.Join(tmp, "dotfiles", "hydra"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "doctor", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "home_source") {
		t.Errorf("project doctor should not report a library source:\n%s", out)
	}
}

func TestAbilityDoctorReportsWhereTheLibraryCameFrom(t *testing.T) {
	tmp := t.TempDir()
	library := filepath.Join(tmp, "dotfiles", "hydra")
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv(hydraHomeEnv, library)

	if _, err := runCLI(t, "ability", "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "ability", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, library) || !strings.Contains(out, hydraHomeEnv) {
		t.Errorf("ability doctor should name the library and its source:\n%s", out)
	}
}

func TestRunRelocateMovesTheGlobalLibrary(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	src := filepath.Join(tmp, "src")
	dest := filepath.Join(tmp, "dotfiles", "hydra")
	t.Chdir(tmp)
	t.Setenv("HOME", home)
	t.Setenv(hydraHomeEnv, src)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "relocate", dest)
	if err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(dest, "rules")) {
		t.Errorf("library did not move to %s", dest)
	}
	if exists(src) {
		t.Error("the source library should be gone")
	}
	if !strings.Contains(out, `export `+hydraHomeEnv+`="`+dest+`"`) {
		t.Errorf("missing the export line:\n%s", out)
	}
	block := readFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(block, filepath.Join(dest, "rules")) || strings.Contains(block, src) {
		t.Errorf("block not rewritten to the new library:\n%s", block)
	}

	// The moved library is now reachable by pointing the resolver at it.
	if _, err := runCLI(t, "doctor", "--global", "--hydra-home", dest); err != nil {
		t.Fatalf("doctor should pass against the relocated library: %v", err)
	}
}

// relocate is inherently global; the default source is ~/.hydra with nothing set.
func TestRunRelocateDefaultsToTheDefaultLibrary(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	dest := filepath.Join(tmp, "elsewhere")
	t.Chdir(tmp)
	isolateHome(t, home)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "relocate", dest); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(dest, "rules")) {
		t.Errorf("library did not move to %s", dest)
	}
	if exists(filepath.Join(home, ".hydra")) {
		t.Error("the default library should be gone after relocating")
	}
}

// A path typed at a prompt resolves against the working directory, the way mv
// and cp behave — unlike HYDRA_HOME, which is refused when relative.
func TestRunRelocateResolvesARelativeDestination(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	work := filepath.Join(tmp, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	isolateHome(t, home)

	if _, err := runCLI(t, "init", "--global"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "relocate", "library"); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(work, "library", "rules")) {
		t.Error("relative destination did not resolve against the working directory")
	}
}

func TestRunRelocateRequiresADestination(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	isolateHome(t, filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "relocate"); err == nil {
		t.Error("relocate without a destination should fail")
	}
}

// Ability commands are always global, so --hydra-home applies to them without
// --global. Each subcommand is checked because they resolve their scope
// independently.
func TestRunAbilityCommandsHonorTheHydraHomeFlag(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	library := filepath.Join(tmp, "library")
	t.Chdir(tmp)
	t.Setenv("HOME", home)

	if _, err := runCLI(t, "ability", "init", "--hydra-home", library); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(library, "abilities")) {
		t.Fatalf("ability init ignored --hydra-home; nothing at %s", library)
	}
	if exists(filepath.Join(home, ".hydra")) {
		t.Error("ability init fell back to the default library")
	}

	if _, err := runCLI(t, "ability", "new", "shipper", "--hydra-home", library); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(library, "abilities", "shipper", abilityFilename)) {
		t.Error("ability new ignored --hydra-home")
	}
	if _, err := runCLI(t, "ability", "sync", "--hydra-home", library); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "ability", "list", "--hydra-home", library)
	if err != nil || !strings.Contains(out, "shipper") {
		t.Errorf("ability list ignored --hydra-home: out=%q err=%v", out, err)
	}
	out, err = runCLI(t, "ability", "doctor", "--hydra-home", library)
	if err != nil || !strings.Contains(out, library) {
		t.Errorf("ability doctor ignored --hydra-home: out=%q err=%v", out, err)
	}
	if _, err := runCLI(t, "ability", "match", "shipper", "--hydra-home", library); err != nil {
		t.Errorf("ability match ignored --hydra-home: %v", err)
	}
}
