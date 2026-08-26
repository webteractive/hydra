package main

import (
	"strings"
	"testing"
)

func sampleRules() []Rule {
	return []Rule{
		{
			Name: "rust-dependencies", Path: "/work/app/.hydra/rules/rust-dependencies.md",
			Title:    "Rust dependencies",
			Paths:    []string{"**/Cargo.toml"},
			Commands: []string{"cargo add"},
			Triggers: []string{"auditing a Rust dependency"},
			Body:     "# Rust dependencies\n\nPin the version.\n",
		},
		{
			Name: "secrets", Path: "/work/app/.hydra/rules/secrets.md",
			Title:  "Secrets",
			Always: true,
			Body:   "# Secrets\n\nNever read ~/.secrets.\n",
		},
	}
}

func TestRenderIndexTable(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	got := RenderIndex(s, sampleRules())

	if !strings.Contains(got, "| auditing a Rust dependency | `**/Cargo.toml` · `cargo add` | .hydra/rules/rust-dependencies.md |") {
		t.Errorf("missing indexed row:\n%s", got)
	}
	if strings.Contains(got, "secrets.md") {
		t.Errorf("always-rule should not appear in the index:\n%s", got)
	}
	if !strings.Contains(got, "do not edit") {
		t.Errorf("missing generated-file warning:\n%s", got)
	}
}

func TestRenderIndexEmptyLibrary(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	got := RenderIndex(s, nil)
	if !strings.Contains(got, "No rules recorded yet.") {
		t.Errorf("empty index should say so:\n%s", got)
	}
}

func TestRenderBlockInlinesAlwaysRules(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	got := RenderBlock(s, sampleRules())

	if !strings.HasPrefix(got, blockStart) || !strings.HasSuffix(strings.TrimRight(got, "\n"), blockEnd) {
		t.Errorf("block must be sentinel-delimited:\n%s", got)
	}
	if !strings.Contains(got, "Never read ~/.secrets.") {
		t.Errorf("always-rule body should be inlined:\n%s", got)
	}
	if !strings.Contains(got, "grep -rin '<keyword>' .hydra/rules") {
		t.Errorf("grep hint should use the relative dir:\n%s", got)
	}
	if !strings.Contains(got, ".hydra/rules/rust-dependencies.md") {
		t.Errorf("indexed rule should appear in the table:\n%s", got)
	}
}

func TestRenderBlockGlobalUsesAbsolutePaths(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	rules := []Rule{{
		Name: "rust", Path: "/home/u/.hydra/rules/rust.md", Title: "Rust",
		Paths: []string{"**/Cargo.toml"}, Body: "# Rust\n",
	}}
	got := RenderBlock(s, rules)

	if !strings.Contains(got, "/home/u/.hydra/rules/rust.md") {
		t.Errorf("global block should use absolute rule paths:\n%s", got)
	}
	if !strings.Contains(got, "grep -rin '<keyword>' /home/u/.hydra/rules") {
		t.Errorf("global grep hint should be absolute:\n%s", got)
	}
	if strings.Contains(got, "override global rules") {
		t.Errorf("the precedence note belongs to project scope only:\n%s", got)
	}
}

func TestRenderBlockProjectStatesPrecedence(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	if got := RenderBlock(s, sampleRules()); !strings.Contains(got, "override global rules") {
		t.Errorf("project block should state precedence:\n%s", got)
	}
}

func TestRenderBlockNoAlwaysSectionWhenNone(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	rules := []Rule{{Name: "a", Path: "/work/app/.hydra/rules/a.md", Title: "A", Paths: []string{"a/**"}, Body: "# A\n"}}
	if got := RenderBlock(s, rules); strings.Contains(got, "### Always applies") {
		t.Errorf("no always-rules means no Always section:\n%s", got)
	}
}

func TestDemoteHeadings(t *testing.T) {
	got := demoteHeadings("# One\n\ntext\n\n## Two\n\n###### Deep\n", 3)
	want := "#### One\n\ntext\n\n##### Two\n\n###### Deep\n"
	if got != want {
		t.Errorf("demoteHeadings =\n%q\nwant\n%q", got, want)
	}
}

func TestDemoteHeadingsIgnoresNonHeadings(t *testing.T) {
	in := "text with # hash mid-line\n\n```\n# a comment in code\n```\n"
	if got := demoteHeadings(in, 3); !strings.Contains(got, "text with # hash mid-line") {
		t.Errorf("mid-line hash should be untouched: %q", got)
	}
}

func TestRenderBlockDemotesInlinedAlwaysRule(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	got := RenderBlock(s, sampleRules())
	if strings.Contains(got, "\n# Secrets") {
		t.Errorf("inlined body must not drop an H1 into the host document:\n%s", got)
	}
	if !strings.Contains(got, "#### Secrets") {
		t.Errorf("inlined H1 should be demoted to H4:\n%s", got)
	}
}
