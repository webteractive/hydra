package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkByPrefix(r DoctorReport, prefix string) (DoctorCheck, bool) {
	for _, c := range r.Checks {
		if strings.HasPrefix(c.Name, prefix) {
			return c, true
		}
	}
	return DoctorCheck{}, false
}

func TestDoctorHealthyInstall(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}

	rep := Doctor(s)
	if !rep.OK {
		t.Errorf("report not OK: %+v", rep.Checks)
	}
}

func TestDoctorMissingRulesDirIsError(t *testing.T) {
	tmp := t.TempDir()
	rep := Doctor(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if rep.OK {
		t.Error("missing rules dir should fail the report")
	}
	c, ok := checkByPrefix(rep, "rules directory")
	if !ok || c.OK || c.Severity != "error" {
		t.Errorf("expected a failing error-severity check, got %+v (found=%v)", c, ok)
	}
}

func TestDoctorMatcherlessRuleIsError(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "orphan.md"), "# Orphan\n\nnothing\n")

	rep := Doctor(s)
	if rep.OK {
		t.Error("a matcher-less rule should fail the report")
	}
	if c, ok := checkByPrefix(rep, "orphan has a matcher"); !ok || c.OK {
		t.Errorf("expected a failing matcher check, got %+v (found=%v)", c, ok)
	}
}

func TestDoctorStaleIndexIsWarningOnly(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")
	// no sync — index.md is now stale

	rep := Doctor(s)
	c, ok := checkByPrefix(rep, "index.md is current")
	if !ok || c.OK {
		t.Errorf("expected a failing index check, got %+v (found=%v)", c, ok)
	}
	if c.Severity != "warning" {
		t.Errorf("Severity = %q want warning", c.Severity)
	}
	if !rep.OK {
		t.Error("a warning must not fail the report")
	}
}

func TestDoctorMalformedFrontmatterIsError(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "bad.md"), "---\npaths: [unclosed\n---\n\n# Bad\n")

	rep := Doctor(s)
	if rep.OK {
		t.Error("malformed frontmatter should fail the report")
	}
}

func TestDoctorDetectsV01Wreckage(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../skills/gone", filepath.Join(tmp, ".claude", "skills", "gone")); err != nil {
		t.Fatal(err)
	}

	rep := Doctor(s)
	c, ok := checkByPrefix(rep, "no v0.1")
	if !ok || c.OK {
		t.Errorf("expected a failing wreckage check, got %+v (found=%v)", c, ok)
	}
	if c.Severity != "warning" {
		t.Errorf("Severity = %q want warning", c.Severity)
	}
}

func TestRenderDoctorText(t *testing.T) {
	rep := DoctorReport{
		Scope: "project", Home: "/x/.hydra", OK: false,
		Checks: []DoctorCheck{
			{Name: "rules directory present", OK: true, Severity: "error"},
			{Name: "index.md is current", OK: false, Severity: "warning", Detail: "run 'hydra sync'"},
		},
	}
	var out bytes.Buffer
	renderDoctorText(&out, rep, "hydra doctor (project: /x/.hydra)", "hydra doctor --json")
	got := out.String()
	if !strings.Contains(got, "✓") || !strings.Contains(got, "!") {
		t.Errorf("expected pass and warn glyphs: %s", got)
	}
	if !strings.Contains(got, "run 'hydra sync'") {
		t.Errorf("detail not rendered: %s", got)
	}
	if !strings.Contains(got, "hydra doctor (project: /x/.hydra)") {
		t.Errorf("header not rendered: %s", got)
	}
	if !strings.Contains(got, "For agents and scripts: hydra doctor --json") {
		t.Errorf("humans should be pointed at the machine-readable output: %s", got)
	}
}
