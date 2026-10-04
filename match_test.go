package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		// ** spans zero, one, and many segments.
		{"**/Cargo.toml", "Cargo.toml", true},
		{"**/Cargo.toml", "crates/core/Cargo.toml", true},
		{"app/Jobs/**", "app/Jobs/Send.php", true},
		{"app/Jobs/**", "app/Jobs/Mail/Send.php", true},
		{"app/Jobs/**", "app/Jobs", true},
		{"app/Jobs/**", "app/Models/User.php", false},
		{"**/.hydra/**", "Users/x/.hydra/rules/a.md", true},
		{"**/.hydra/**", ".hydra/rules/a.md", true},
		{"**/cmd/**", "tools/cmd/hydra/main.go", true},
		{"src/**/test.ts", "src/test.ts", true},
		{"src/**/test.ts", "src/a/b/test.ts", true},
		// * and ? stay within one segment.
		{"app/*.php", "app/User.php", true},
		{"app/*.php", "app/Models/User.php", false},
		{"**/.env*", "project/.env.testing", true},
		{"**/.env*", "project/.environment/x", false},
		{"v?.go", "v1.go", true},
		// A pattern with no slash matches the basename anywhere, as gitignore does.
		{"*.php", "app/Models/User.php", true},
		{"Makefile", "build/Makefile", true},
		{"*.php", "app/User.go", false},
		// Literal segments must match exactly.
		{"**/main.go", "main.go.bak", false},
		{"**/main.go", "cmd/notmain.go", false},
	} {
		if got := GlobMatch(tc.pattern, tc.path); got != tc.want {
			t.Errorf("GlobMatch(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestCommandMatches(t *testing.T) {
	for _, tc := range []struct {
		pattern, command string
		want             bool
	}{
		{"artisan test", "php artisan test --filter=Foo", true},
		{"vendor/bin/pest", "./vendor/bin/pest tests/Unit", true},
		{"git tag", "git tag v1.2.3", true},
		{"git tag", "cd repo && git tag -l", true},
		{"warden ", "warden list", true},
		{"warden", "wardenx list", false},
		{"warden", "mywarden list", false},
		{"npm test", "npm testing", false},
		{"gh release", "gh   release\n  create v1", true},
		{"cargo add", "cargo addx", false},
		{"hydra ", "hydra", true},
		{"git commit", "git status", false},
		// Known, accepted false positive: the boundary rule cannot see quoting.
		{"git tag", `echo "git tag"`, true},
		{"", "anything", false},
		{"   ", "anything", false},
	} {
		if got := CommandMatches(tc.pattern, tc.command); got != tc.want {
			t.Errorf("CommandMatches(%q, %q) = %v, want %v", tc.pattern, tc.command, got, tc.want)
		}
	}
}

// ruleMatchFixture lays out a project library, a global library, and a dotfiles-
// dialect library under one temp dir.
func ruleMatchFixture(t *testing.T) (root, global, extra string) {
	t.Helper()
	tmp := t.TempDir()
	root = filepath.Join(tmp, "project")
	global = filepath.Join(tmp, "home", ".hydra", "rules")
	extra = filepath.Join(tmp, "dotfiles", "rules")

	mustWrite(t, filepath.Join(root, ".hydra", "rules", "queue.md"),
		"---\npaths: [\"app/Jobs/**\"]\ncommands: [\"php artisan queue\"]\n---\n\n# Queue conventions\n\nJobs are idempotent.\n")
	mustWrite(t, filepath.Join(root, ".hydra", "rules", "guard.md"),
		"---\nalways: true\npaths: [\"**\"]\n---\n\n# Guard\n")
	mustWrite(t, filepath.Join(global, "go-cli.md"),
		"---\npaths: [\"**/main.go\"]\n---\n\n# Go CLIs\n")
	mustWrite(t, filepath.Join(extra, "warden.md"),
		"---\ntitle: warden (env & secrets CLI)\nwhen:\n  - checking an env key\ncommands:\n  - \"warden \"\npaths:\n  - \"**/.env*\"\n---\n\n- Use warden.\n")
	mustWrite(t, filepath.Join(extra, "README.md"), "# Rules\n\nNot a rule.\n")
	return root, global, extra
}

func libs(root, global, extra string) []RuleLibrary {
	out := []RuleLibrary{
		{Kind: LibraryProject, Dir: filepath.Join(root, ".hydra", "rules")},
		{Kind: LibraryGlobal, Dir: global},
	}
	if extra != "" {
		out = append(out, RuleLibrary{Kind: LibraryExtra, Dir: extra})
	}
	return out
}

func matchNames(rep RuleMatchReport) []string {
	var names []string
	for _, m := range rep.Matches {
		names = append(names, m.Rule)
	}
	return names
}

func TestMatchRulesAcrossLibraries(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)

	rep := MatchRules(MatchRequest{
		Root:      root,
		Libraries: libs(root, global, extra),
		Paths:     []string{filepath.Join(root, "app", "Jobs", "Send.php"), filepath.Join(root, "cmd", "main.go"), filepath.Join(root, ".env.example")},
	})
	if got := strings.Join(matchNames(rep), ","); got != "queue,go-cli,warden" {
		t.Fatalf("matches = %s, want queue,go-cli,warden (project, global, extra order)", got)
	}
	if len(rep.Errors) != 0 {
		t.Errorf("unexpected errors: %+v", rep.Errors)
	}

	queue := rep.Matches[0]
	if queue.Title != "Queue conventions" || queue.Library.Kind != LibraryProject {
		t.Errorf("queue = %+v", queue)
	}
	if len(queue.Matched) != 1 || queue.Matched[0].Kind != "path" || queue.Matched[0].Pattern != "app/Jobs/**" {
		t.Errorf("queue.Matched = %+v", queue.Matched)
	}
	if warden := rep.Matches[2]; warden.Title != "warden (env & secrets CLI)" {
		t.Errorf("a title: frontmatter key should name the rule, got %q", warden.Title)
	}
}

func TestMatchRulesCommands(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	rep := MatchRules(MatchRequest{
		Root:      root,
		Libraries: libs(root, global, extra),
		Commands:  []string{"php artisan queue:work", "warden has STRIPE_KEY"},
	})
	if got := strings.Join(matchNames(rep), ","); got != "queue,warden" {
		t.Fatalf("matches = %s, want queue,warden", got)
	}
	if hit := rep.Matches[1].Matched[0]; hit.Kind != "command" || hit.Subject != "warden has STRIPE_KEY" {
		t.Errorf("warden hit = %+v", hit)
	}
}

func TestMatchRulesExcludesAlwaysRulesUnlessAsked(t *testing.T) {
	root, global, _ := ruleMatchFixture(t)
	path := filepath.Join(root, "README.md")
	if rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, ""), Paths: []string{path}}); len(rep.Matches) != 0 {
		t.Errorf("always rules are already in context and must not fire: %v", matchNames(rep))
	}
	rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, ""), Paths: []string{path}, IncludeAlways: true})
	if got := strings.Join(matchNames(rep), ","); got != "guard" || !rep.Matches[0].Always {
		t.Errorf("--include-always: matches = %s", got)
	}
}

// A project rule is anchored at the project root; a global or extra rule also
// sees the absolute path, so a file outside the project can still match it.
func TestMatchRulesProjectRulesAreRootRelative(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	outside := filepath.Join(filepath.Dir(root), "elsewhere", "app", "Jobs", "Send.php")
	if rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, extra), Paths: []string{outside}}); len(rep.Matches) != 0 {
		t.Errorf("a project rule must not match outside its project: %v", matchNames(rep))
	}

	env := filepath.Join(filepath.Dir(root), "other", ".env")
	rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, extra), Paths: []string{env}})
	if got := strings.Join(matchNames(rep), ","); got != "warden" {
		t.Errorf("an extra library's **/ glob should match an absolute path anywhere, got %s", got)
	}
}

func TestMatchRulesResolvesRelativeSubjectsAgainstTheRoot(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, extra), Paths: []string{"app/Jobs/Send.php"}})
	if got := strings.Join(matchNames(rep), ","); got != "queue" {
		t.Errorf("matches = %s, want queue", got)
	}
	if rep.Paths[0] != filepath.Join(root, "app", "Jobs", "Send.php") {
		t.Errorf("subjects are reported absolute, got %q", rep.Paths[0])
	}
}

func TestMatchRulesExpandsTildePatterns(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	extra := filepath.Join(tmp, "rules")
	mustWrite(t, filepath.Join(extra, "secrets.md"), "---\npaths: [\"~/.secrets\"]\n---\n")

	rep := MatchRules(MatchRequest{
		Root:      filepath.Join(tmp, "project"),
		Home:      home,
		Libraries: []RuleLibrary{{Kind: LibraryExtra, Dir: extra}},
		Paths:     []string{filepath.Join(home, ".secrets")},
	})
	if got := strings.Join(matchNames(rep), ","); got != "secrets" {
		t.Errorf("matches = %s, want secrets", got)
	}
}

// One broken file must not stop the rest of its library, or the other
// libraries, from matching.
func TestMatchRulesToleratesABrokenRule(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	mustWrite(t, filepath.Join(global, "broken.md"), "---\npaths: [unclosed\n---\n")

	rep := MatchRules(MatchRequest{Root: root, Libraries: libs(root, global, extra), Paths: []string{filepath.Join(root, "main.go")}})
	if got := strings.Join(matchNames(rep), ","); got != "go-cli" {
		t.Errorf("matches = %s, want go-cli despite the broken sibling", got)
	}
	if len(rep.Errors) != 1 || !strings.HasSuffix(rep.Errors[0].File, "broken.md") || rep.Errors[0].Library.Kind != LibraryGlobal {
		t.Errorf("errors = %+v, want the one broken file named", rep.Errors)
	}
}

func TestMatchRulesSkipsMissingLibraries(t *testing.T) {
	tmp := t.TempDir()
	rep := MatchRules(MatchRequest{
		Root:      tmp,
		Libraries: []RuleLibrary{{Kind: LibraryProject, Dir: filepath.Join(tmp, ".hydra", "rules")}},
		Paths:     []string{filepath.Join(tmp, "x.go")},
	})
	if len(rep.Matches) != 0 || len(rep.Errors) != 0 {
		t.Errorf("a missing library is simply empty: %+v", rep)
	}
	if rep.Libraries[0].Present {
		t.Error("a missing library should report present: false")
	}
}

func TestRunMatchJSONAndExitCodes(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	t.Chdir(root)
	isolateHome(t, filepath.Dir(filepath.Dir(global)))

	out, err := runCLI(t, "match", "--path", "cmd/main.go", "--library", extra, "--json")
	if err != nil {
		t.Fatalf("a match exits 0: %v\n%s", err, out)
	}
	var rep RuleMatchReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got := strings.Join(matchNames(rep), ","); got != "go-cli" {
		t.Errorf("matches = %s", got)
	}

	out, err = runCLI(t, "match", "--path", "docs/x.md", "--json")
	if exitCode(err) != 1 {
		t.Errorf("no match exits 1, got %v", err)
	}
	if !strings.Contains(out, `"matches": []`) {
		t.Errorf("no-match JSON should still be printed with an empty list:\n%s", out)
	}

	out, err = runCLI(t, "match", "--json")
	if exitCode(err) != 2 {
		t.Errorf("no subject is a usage error and exits 2, got %v", err)
	}
	if !strings.Contains(out, `"error"`) {
		t.Errorf("a usage error still prints JSON for scripts:\n%s", out)
	}

	if _, err := runCLI(t, "match", "--path", "x", "--bogus"); exitCode(err) != 2 {
		t.Errorf("an unknown flag exits 2, got %v", err)
	}
}

func TestRunMatchLibrarySelection(t *testing.T) {
	root, global, _ := ruleMatchFixture(t)
	t.Chdir(root)
	isolateHome(t, filepath.Dir(filepath.Dir(global)))

	both := []string{"--path", "app/Jobs/A.php", "--path", "main.go"}
	for _, tc := range []struct {
		flags []string
		want  string
	}{
		{nil, "queue,go-cli"},
		{[]string{"--no-project"}, "go-cli"},
		{[]string{"--global"}, "go-cli"},
		{[]string{"--no-global"}, "queue"},
	} {
		args := append(append([]string{"match"}, both...), append(tc.flags, "--json")...)
		out, _ := runCLI(t, args...)
		var rep RuleMatchReport
		if err := json.Unmarshal([]byte(out), &rep); err != nil {
			t.Fatalf("%v: not JSON: %v\n%s", tc.flags, err, out)
		}
		if got := strings.Join(matchNames(rep), ","); got != tc.want {
			t.Errorf("%v: matches = %s, want %s", tc.flags, got, tc.want)
		}
	}
}

func TestRunMatchHumanOutput(t *testing.T) {
	root, global, extra := ruleMatchFixture(t)
	t.Chdir(root)
	isolateHome(t, filepath.Dir(filepath.Dir(global)))

	out, _ := runCLI(t, "match", "--command", "warden list", "--library", extra)
	for _, want := range []string{"warden (env & secrets CLI)", `command "warden "`, "warden list", "For agents and scripts: hydra match"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}

	out, _ = runCLI(t, "match", "--path", "docs/x.md")
	for _, want := range []string{"No rule matches", "triggers"} {
		if !strings.Contains(out, want) {
			t.Errorf("no-match output should say what to do next (%q):\n%s", want, out)
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec *exitCodeError
	if errors.As(err, &ec) {
		return ec.code
	}
	return 1
}
