package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRuleBlockAndFlowLists(t *testing.T) {
	block := `---
always: false
paths:
  - "**/Cargo.toml"
  - "**/Cargo.lock"
commands:
  - cargo add
triggers:
  - auditing a Rust dependency
---

# Rust dependencies

Pin the exact version.
`
	flow := `---
paths: ["**/Cargo.toml", "**/Cargo.lock"]
commands: ["cargo add"]
triggers: ["auditing a Rust dependency"]
---

# Rust dependencies

Pin the exact version.
`
	for name, content := range map[string]string{"block": block, "flow": flow} {
		r, err := ParseRule("rust-dependencies", "/tmp/rust-dependencies.md", content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := []string{"**/Cargo.toml", "**/Cargo.lock"}; !reflect.DeepEqual(r.Paths, want) {
			t.Errorf("%s: Paths = %v want %v", name, r.Paths, want)
		}
		if want := []string{"cargo add"}; !reflect.DeepEqual(r.Commands, want) {
			t.Errorf("%s: Commands = %v want %v", name, r.Commands, want)
		}
		if want := []string{"auditing a Rust dependency"}; !reflect.DeepEqual(r.Triggers, want) {
			t.Errorf("%s: Triggers = %v want %v", name, r.Triggers, want)
		}
		if r.Title != "Rust dependencies" {
			t.Errorf("%s: Title = %q want %q", name, r.Title, "Rust dependencies")
		}
		if r.Always {
			t.Errorf("%s: Always = true, want false", name)
		}
	}
}

func TestParseRuleTitleFallsBackToFilename(t *testing.T) {
	r, err := ParseRule("rust-dependencies", "/tmp/x.md", "---\nalways: true\n---\n\nno heading here\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Rust Dependencies" {
		t.Errorf("Title = %q want %q", r.Title, "Rust Dependencies")
	}
	if !r.Always {
		t.Error("Always = false, want true")
	}
}

func TestParseRuleNoFrontmatter(t *testing.T) {
	r, err := ParseRule("plain", "/tmp/plain.md", "# Plain\n\nbody\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.HasMatcher() {
		t.Error("HasMatcher() = true for a rule with no frontmatter")
	}
	if r.Body != "# Plain\n\nbody\n" {
		t.Errorf("Body = %q", r.Body)
	}
}

func TestParseRuleMalformedYAML(t *testing.T) {
	_, err := ParseRule("bad", "/tmp/bad.md", "---\npaths: [unclosed\n---\n\nbody\n")
	if err == nil {
		t.Fatal("expected an error for malformed frontmatter")
	}
}

func TestHasMatcher(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		want bool
	}{
		{"paths", Rule{Paths: []string{"a/**"}}, true},
		{"commands", Rule{Commands: []string{"cargo"}}, true},
		{"triggers", Rule{Triggers: []string{"doing a thing"}}, true},
		{"always", Rule{Always: true}, true},
		{"none", Rule{}, false},
	}
	for _, c := range cases {
		if got := c.rule.HasMatcher(); got != c.want {
			t.Errorf("%s: HasMatcher() = %v want %v", c.name, got, c.want)
		}
	}
}

func TestLoadRulesSortsAndSkipsIndex(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("zebra.md", "---\npaths: [\"z/**\"]\n---\n\n# Zebra\n")
	write("alpha.md", "---\npaths: [\"a/**\"]\n---\n\n# Alpha\n")
	write("index.md", "# Hydra Rules Index\n")
	write("notes.txt", "ignored")

	rules, err := LoadRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(rules), rules)
	}
	if rules[0].Name != "alpha" || rules[1].Name != "zebra" {
		t.Errorf("order = %s, %s", rules[0].Name, rules[1].Name)
	}
	if rules[0].Path != filepath.Join(dir, "alpha.md") {
		t.Errorf("Path = %s", rules[0].Path)
	}
}

func TestLoadRulesMissingDir(t *testing.T) {
	rules, err := LoadRules(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("want nil error for a missing dir, got %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("len = %d, want 0", len(rules))
	}
}

func TestRenderRuleFileRoundTrips(t *testing.T) {
	in := Rule{
		Name:     "secrets",
		Always:   true,
		Paths:    []string{"**/.env*"},
		Commands: []string{"warden"},
		Triggers: []string{"handling a credential"},
		Body:     "# Secrets\n\nNever read ~/.secrets.\n",
	}
	out, err := ParseRule("secrets", "/tmp/secrets.md", RenderRuleFile(in))
	if err != nil {
		t.Fatal(err)
	}
	if !out.Always || !reflect.DeepEqual(out.Paths, in.Paths) ||
		!reflect.DeepEqual(out.Commands, in.Commands) ||
		!reflect.DeepEqual(out.Triggers, in.Triggers) {
		t.Errorf("round-trip lost data: %+v", out)
	}
	if out.Body != in.Body {
		t.Errorf("Body = %q want %q", out.Body, in.Body)
	}
}
