package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSkill(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\nargument-hint: \"[target]\"\n---\n\n# " + headline(name) + "\n\nDo the work.\n"
}

// writeSkill creates a skill bundle at dir and returns dir.
func writeSkill(t *testing.T, dir, content string) string {
	t.Helper()
	mustWrite(t, filepath.Join(dir, skillFilename), content)
	return dir
}

func initAbilityLibrary(t *testing.T) AbilityScope {
	t.Helper()
	s := ResolveAbilityScope(t.TempDir())
	if err := AbilityInit(s, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAbilityImportMovesSkill(t *testing.T) {
	s := initAbilityLibrary(t)
	src := writeSkill(t, filepath.Join(t.TempDir(), "skills", "bug-report"), validSkill("bug-report", "File a bug."))
	mustWrite(t, filepath.Join(src, "references", "template.md"), "template\n")

	var out bytes.Buffer
	if err := AbilityImport(s, []string{src}, false, &out); err != nil {
		t.Fatal(err)
	}

	bundle := filepath.Join(s.AbilitiesDir, "bug-report")
	got := readFile(t, filepath.Join(bundle, abilityFilename))
	if got != validSkill("bug-report", "File a bug.") {
		t.Errorf("ABILITY.md was not copied verbatim:\n%s", got)
	}
	if exists(filepath.Join(bundle, skillFilename)) {
		t.Error("SKILL.md was carried into the ability bundle")
	}
	if readFile(t, filepath.Join(bundle, "references", "template.md")) != "template\n" {
		t.Error("supporting file was not copied")
	}
	if exists(src) {
		t.Error("source skill was not removed")
	}
	if !strings.Contains(readFile(t, filepath.Join(s.AbilitiesDir, abilityIndexFile)), "bug-report") {
		t.Error("imported ability was not synced into the index")
	}
	if !strings.Contains(out.String(), "has no triggers") {
		t.Errorf("output does not point at the missing triggers:\n%s", out.String())
	}
}

func TestAbilityImportKeepLeavesSource(t *testing.T) {
	s := initAbilityLibrary(t)
	src := writeSkill(t, filepath.Join(t.TempDir(), "bug-report"), validSkill("bug-report", "File a bug."))

	var out bytes.Buffer
	if err := AbilityImport(s, []string{src}, true, &out); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(src, skillFilename)) {
		t.Error("--keep removed the source skill")
	}
	if !exists(filepath.Join(s.AbilitiesDir, "bug-report", abilityFilename)) {
		t.Error("ability was not imported")
	}
}

func TestAbilityImportNamesFromDirectoryAndFlattensDescription(t *testing.T) {
	s := initAbilityLibrary(t)
	content := "---\n# kept comment\ndescription: >\n  Spans several\n  lines: really.\nuser-invocable: true\n---\n\nBody.\n"
	src := writeSkill(t, filepath.Join(t.TempDir(), "multi-line"), content)

	if err := AbilityImport(s, []string{src}, false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.AbilitiesDir, "multi-line", abilityFilename)
	got := readFile(t, path)
	ability, err := ParseAbility("multi-line", path, got)
	if err != nil {
		t.Fatalf("imported ability does not parse: %v\n%s", err, got)
	}
	if ability.Description != "Spans several lines: really." {
		t.Errorf("description = %q", ability.Description)
	}
	for _, want := range []string{"# kept comment", "user-invocable: true", "\nBody.\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("imported ability lost %q:\n%s", want, got)
		}
	}
}

func TestAbilityImportMovesSeveral(t *testing.T) {
	s := initAbilityLibrary(t)
	root := t.TempDir()
	a := writeSkill(t, filepath.Join(root, "alpha"), validSkill("alpha", "First."))
	b := writeSkill(t, filepath.Join(root, "beta"), validSkill("beta", "Second."))

	if err := AbilityImport(s, []string{a, b}, false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if !exists(filepath.Join(s.AbilitiesDir, name, abilityFilename)) {
			t.Errorf("%s was not imported", name)
		}
	}
	if exists(a) || exists(b) {
		t.Error("sources were not removed")
	}
}

// Every refusal must leave both the library and every source untouched, even
// when an earlier path in the same call was valid.
func TestAbilityImportRefusesWithoutTouchingAnything(t *testing.T) {
	cases := map[string]func(t *testing.T, s AbilityScope, root string) string{
		"no SKILL.md": func(t *testing.T, s AbilityScope, root string) string {
			dir := filepath.Join(root, "empty")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		},
		"name collides with an ability": func(t *testing.T, s AbilityScope, root string) string {
			mustWrite(t, filepath.Join(s.AbilitiesDir, "taken", abilityFilename), validAbility("taken", "Already here."))
			if err := AbilitySync(s, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			return writeSkill(t, filepath.Join(root, "taken"), validSkill("taken", "Imported."))
		},
		"duplicate name in one call": func(t *testing.T, s AbilityScope, root string) string {
			return writeSkill(t, filepath.Join(root, "other", "good"), validSkill("good", "Same name twice."))
		},
		"not kebab-case": func(t *testing.T, s AbilityScope, root string) string {
			return writeSkill(t, filepath.Join(root, "Bad_Name"), "---\ndescription: Bad.\n---\n")
		},
		"missing description": func(t *testing.T, s AbilityScope, root string) string {
			return writeSkill(t, filepath.Join(root, "no-desc"), "---\nname: no-desc\n---\n")
		},
		"hydra router": func(t *testing.T, s AbilityScope, root string) string {
			return writeSkill(t, filepath.Join(root, "ability"), "---\nname: ability\ndescription: Router.\n---\n\n"+routerOwnedMarker+"\n")
		},
		"inside the library": func(t *testing.T, s AbilityScope, root string) string {
			return writeSkill(t, filepath.Join(s.AbilitiesDir, "inner"), validSkill("inner", "Already inside."))
		},
		"symlink inside the bundle": func(t *testing.T, s AbilityScope, root string) string {
			dir := writeSkill(t, filepath.Join(root, "linked"), validSkill("linked", "Has a link."))
			if err := os.Symlink("/etc/hosts", filepath.Join(dir, "hosts")); err != nil {
				t.Fatal(err)
			}
			return dir
		},
	}
	wants := map[string]string{
		"no SKILL.md":                   "no SKILL.md",
		"name collides with an ability": "ability already exists",
		"duplicate name in one call":    "more than once",
		"not kebab-case":                "kebab-case",
		"missing description":           "description is required",
		"hydra router":                  "router",
		"inside the library":            "already inside",
		"symlink inside the bundle":     "not a regular file",
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			s := initAbilityLibrary(t)
			root := t.TempDir()
			good := writeSkill(t, filepath.Join(root, "good"), validSkill("good", "Valid."))
			bad := setup(t, s, root)
			before := listDir(t, s.AbilitiesDir)

			err := AbilityImport(s, []string{good, bad}, false, &bytes.Buffer{})
			if err == nil {
				t.Fatal("import succeeded")
			}
			if !strings.Contains(err.Error(), wants[name]) {
				t.Errorf("error does not say %q:\n%v", wants[name], err)
			}
			if !exists(filepath.Join(good, skillFilename)) || !exists(bad) {
				t.Error("a refused import removed a source")
			}
			if after := listDir(t, s.AbilitiesDir); after != before {
				t.Errorf("a refused import changed the library:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}

func TestAbilityImportRemovesOnlyTheLink(t *testing.T) {
	s := initAbilityLibrary(t)
	root := t.TempDir()
	target := writeSkill(t, filepath.Join(root, "store", "linked"), validSkill("linked", "Lives in a store."))
	link := filepath.Join(root, "skills", "linked")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := AbilityImport(s, []string{link}, false, &out); err != nil {
		t.Fatal(err)
	}
	if exists(link) {
		t.Error("the link was not removed")
	}
	if !exists(filepath.Join(target, skillFilename)) {
		t.Error("the link's target was removed")
	}
	if !exists(filepath.Join(s.AbilitiesDir, "linked", abilityFilename)) {
		t.Error("the linked skill was not imported")
	}
	if !strings.Contains(out.String(), target) {
		t.Errorf("output does not name the target it left in place:\n%s", out.String())
	}
}

func TestAbilityImportFailedSyncKeepsSources(t *testing.T) {
	s := initAbilityLibrary(t)
	// A broken bundle already in the library makes the closing sync fail.
	mustWrite(t, filepath.Join(s.AbilitiesDir, "broken", abilityFilename), "no frontmatter\n")
	src := writeSkill(t, filepath.Join(t.TempDir(), "fine"), validSkill("fine", "Valid."))

	if err := AbilityImport(s, []string{src}, false, &bytes.Buffer{}); err == nil {
		t.Fatal("import succeeded over a broken library")
	}
	if !exists(filepath.Join(src, skillFilename)) {
		t.Error("a failed sync removed the source")
	}
	if exists(filepath.Join(s.AbilitiesDir, "fine")) {
		t.Error("a failed sync left the imported bundle behind")
	}
}

func TestAbilityImportInitializesMissingLibrary(t *testing.T) {
	s := ResolveAbilityScope(t.TempDir())
	src := writeSkill(t, filepath.Join(t.TempDir(), "first"), validSkill("first", "Valid."))
	if err := AbilityImport(s, []string{src}, false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(s.AbilitiesDir, "first", abilityFilename)) {
		t.Error("ability was not imported into a fresh library")
	}
}

func TestAbilityImportMapsDisableModelInvocationToExplicit(t *testing.T) {
	s := initAbilityLibrary(t)
	content := "---\nname: deploy-prod\ndescription: Deploy to production.\ndisable-model-invocation: true\n---\n\nBody.\n"
	src := writeSkill(t, filepath.Join(t.TempDir(), "deploy-prod"), content)

	if err := AbilityImport(s, []string{src}, false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.AbilitiesDir, "deploy-prod", abilityFilename)
	got := readFile(t, path)
	ability, err := ParseAbility("deploy-prod", path, got)
	if err != nil {
		t.Fatalf("imported ability does not parse: %v\n%s", err, got)
	}
	if ability.Invocation != abilityInvocationExplicit {
		t.Errorf("Invocation = %q, want explicit:\n%s", ability.Invocation, got)
	}
	for _, want := range []string{"disable-model-invocation: true", "\nBody.\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("imported ability lost %q:\n%s", want, got)
		}
	}
}

// An authored invocation key is the author's decision, and a skill that never
// disabled model invocation needs no rewrite: both are copied byte for byte.
func TestAbilityImportLeavesInvocationAsAuthored(t *testing.T) {
	cases := map[string]string{
		"authored invocation": "---\nname: keep-auto\ndescription: Keep it.\ndisable-model-invocation: true\ninvocation: auto\n---\n\nBody.\n",
		"model invocation on": "---\nname: keep-auto\ndescription: Keep it.\ndisable-model-invocation: false\n---\n\nBody.\n",
	}
	for label, content := range cases {
		s := initAbilityLibrary(t)
		src := writeSkill(t, filepath.Join(t.TempDir(), "keep-auto"), content)
		if err := AbilityImport(s, []string{src}, false, &bytes.Buffer{}); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if got := readFile(t, filepath.Join(s.AbilitiesDir, "keep-auto", abilityFilename)); got != content {
			t.Errorf("%s: ABILITY.md was rewritten:\n%s", label, got)
		}
	}
}

func listDir(t *testing.T, dir string) string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		names = append(names, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(names, " ")
}
