package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func addScope(t *testing.T) (Scope, string) {
	t.Helper()
	tmp := t.TempDir()
	return ResolveScope(false, tmp, filepath.Join(tmp, "home")), tmp
}

func TestAddInitializesWhenMissing(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	req := AddRequest{Title: "Extend BaseController", Note: "Always extend BaseController.", Paths: []string{"app/Http/Controllers/**"}}
	if err := Add(s, req, &out); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(tmp, ".hydra", "rules")) {
		t.Error("add should have initialized the library")
	}
	if !exists(filepath.Join(tmp, "CLAUDE.md")) {
		t.Error("add should have created default targets")
	}
}

func TestAddCreatesAreaFileFromGlob(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	req := AddRequest{Title: "Extend BaseController", Note: "Always extend BaseController.", Paths: []string{"app/Http/Controllers/**"}}
	if err := Add(s, req, &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, ".hydra", "rules", "controllers.md")
	got := readFile(t, path)
	if !strings.Contains(got, "app/Http/Controllers/**") {
		t.Errorf("frontmatter missing the glob: %s", got)
	}
	if !strings.Contains(got, "## Extend BaseController") {
		t.Errorf("missing the entry heading: %s", got)
	}
	if !strings.Contains(got, "Always extend BaseController.") {
		t.Errorf("missing the note: %s", got)
	}
}

func TestAddAppendsToSameArea(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	first := AddRequest{Title: "One", Note: "First rule.", Paths: []string{"app/Http/Controllers/**"}}
	second := AddRequest{Title: "Two", Note: "Second rule.", Paths: []string{"app/Http/Controllers/*.php"}}
	if err := Add(s, first, &out); err != nil {
		t.Fatal(err)
	}
	if err := Add(s, second, &out); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(tmp, ".hydra", "rules", "controllers.md"))
	if !strings.Contains(got, "## One") || !strings.Contains(got, "## Two") {
		t.Errorf("both entries should be in one file: %s", got)
	}
	if !strings.Contains(got, "app/Http/Controllers/**") || !strings.Contains(got, "app/Http/Controllers/*.php") {
		t.Errorf("both globs should be merged into frontmatter: %s", got)
	}
	if exists(filepath.Join(tmp, ".hydra", "rules", "controllers-2.md")) {
		t.Error("same area should not create a second file")
	}
}

func TestAddSeparateAreasGetSeparateFiles(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	if err := Add(s, AddRequest{Title: "A", Note: "n", Paths: []string{"app/Models/**"}}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Add(s, AddRequest{Title: "B", Note: "n", Paths: []string{"database/migrations/**"}}, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(tmp, ".hydra", "rules", "models.md")) {
		t.Error("models.md missing")
	}
	if !exists(filepath.Join(tmp, ".hydra", "rules", "migrations.md")) {
		t.Error("migrations.md missing")
	}
}

func TestAreaKeyPrecedence(t *testing.T) {
	cases := []struct {
		name string
		req  AddRequest
		want string
	}{
		{"glob wins", AddRequest{Title: "T", Paths: []string{"app/Models/**"}, Commands: []string{"cargo add"}, Triggers: []string{"x"}}, "models"},
		{"command next", AddRequest{Title: "T", Commands: []string{"cargo add"}, Triggers: []string{"x"}}, "cargo"},
		{"title last", AddRequest{Title: "Release Process", Triggers: []string{"x"}}, "release-process"},
		// A glob of only wildcards and dotted segments has no meaningful
		// segment, so it falls through to the title.
		{"wildcard-only glob", AddRequest{Title: "Cargo Manifests", Paths: []string{"**/Cargo.toml"}}, "cargo-manifests"},
	}
	for _, c := range cases {
		if got := areaKey(c.req); got != c.want {
			t.Errorf("%s: areaKey = %q want %q", c.name, got, c.want)
		}
	}
}

func TestAddAlwaysRule(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	req := AddRequest{Title: "Never commit automatically", Note: "Ask first.", Always: true}
	if err := Add(s, req, &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, ".hydra", "rules", "never-commit-automatically.md")
	if !strings.Contains(readFile(t, path), "always: true") {
		t.Errorf("always flag not written: %s", readFile(t, path))
	}
	if !strings.Contains(readFile(t, filepath.Join(tmp, "CLAUDE.md")), "Ask first.") {
		t.Error("always-rule body should be inlined into the block")
	}
}

func TestAddRequiresTitleNoteAndMatcher(t *testing.T) {
	s, _ := addScope(t)
	var out bytes.Buffer
	cases := []AddRequest{
		{Note: "n", Paths: []string{"a/**"}},  // no title
		{Title: "T", Paths: []string{"a/**"}}, // no note
		{Title: "T", Note: "n"},               // no matcher, not always
	}
	for i, req := range cases {
		if err := Add(s, req, &out); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}

func TestAddTitleDerivedAreaHasNoDuplicateHeading(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	req := AddRequest{Title: "Never commit automatically", Note: "Ask first.", Always: true}
	if err := Add(s, req, &out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(tmp, ".hydra", "rules", "never-commit-automatically.md"))
	if strings.Contains(got, "## Never commit automatically") {
		t.Errorf("title-derived area should not repeat the title as an entry heading:\n%s", got)
	}
	if !strings.Contains(got, "# Never commit automatically") {
		t.Errorf("file heading should be the title verbatim:\n%s", got)
	}
	if !strings.Contains(got, "Ask first.") {
		t.Errorf("note missing:\n%s", got)
	}
}

// Trigger-only rules have no structural area to group by, so each becomes its
// own file rather than piling into a shared one.
func TestAddTriggerOnlyRulesGetTheirOwnFiles(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	if err := Add(s, AddRequest{Title: "Release Process", Note: "Tag then push.", Triggers: []string{"cutting a release"}}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Add(s, AddRequest{Title: "Sign the tag", Note: "Use GPG.", Triggers: []string{"cutting a release"}}, &out); err != nil {
		t.Fatal(err)
	}
	first := readFile(t, filepath.Join(tmp, ".hydra", "rules", "release-process.md"))
	if !strings.Contains(first, "# Release Process") || !strings.Contains(first, "Tag then push.") {
		t.Errorf("release-process.md wrong:\n%s", first)
	}
	second := readFile(t, filepath.Join(tmp, ".hydra", "rules", "sign-the-tag.md"))
	if !strings.Contains(second, "# Sign the tag") || !strings.Contains(second, "Use GPG.") {
		t.Errorf("sign-the-tag.md wrong:\n%s", second)
	}
}

// A rule filed under a structural area (a glob) does get a "## Title" entry, so
// several rules can share one area file.
func TestAddGlobAreaKeepsEntryHeadings(t *testing.T) {
	s, tmp := addScope(t)
	var out bytes.Buffer
	if err := Add(s, AddRequest{Title: "Extend BaseController", Note: "n1", Paths: []string{"app/Http/Controllers/**"}}, &out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(tmp, ".hydra", "rules", "controllers.md"))
	if !strings.Contains(got, "# Controllers") {
		t.Errorf("file heading should name the area:\n%s", got)
	}
	if !strings.Contains(got, "## Extend BaseController") {
		t.Errorf("entry heading missing:\n%s", got)
	}
}
