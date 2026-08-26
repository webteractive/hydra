# hydra v0.2 Rules Pivot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hydra's skill-curator machinery with a rules library that splices a generated, matcher-scoped index into whatever agent instruction files a project or home directory already has.

**Architecture:** `.hydra/rules/*.md` files carry YAML frontmatter declaring `always`, `paths`, `commands`, and `triggers`. `hydra sync` parses the library once and renders two artifacts from that single pass: a canonical `index.md` and a managed block spliced between sentinels into every detected harness instruction file. Project scope renders repo-relative paths; global scope renders absolute ones. No hooks, no symlinks, no MCP.

**Tech Stack:** Go 1.26, `github.com/spf13/cobra`, `gopkg.in/yaml.v3` (new). Standard library otherwise. Tests are stdlib `testing` with `t.TempDir()`, no network.

**Spec:** `docs/superpowers/specs/2026-08-26-rules-pivot-design.md`

## Global Constraints

- **Branch first.** The repo is on `main`. Before Task 1: `git checkout -b rules-pivot`.
- **Never auto-commit or push.** Each task's commit step stages and commits locally only. Never run `git push`.
- **No `Co-Authored-By` trailer and no `Claude-Session:` URL** in any commit message.
- **`CLAUDE.md` and `AGENTS.md` are mirrors** — identical body, only the top title/intro line differs. Any edit to one MUST be replicated to the other in the same commit.
- **Dependencies:** stdlib + cobra + `gopkg.in/yaml.v3`. Nothing else.
- **Package layout:** flat `package main` at the repo root. No subpackages.
- **Command functions take `(s Scope, ..., out io.Writer) error`** so cobra stays a pure parsing layer and every command is unit-testable. Preserve this.
- **`gofmt -l .` must be empty and `go vet ./...` must be clean** before every commit. CI enforces both.
- **Sentinels:** `<!-- hydra:rules:start -->` and `<!-- hydra:rules:end -->`. The dead v0.1 sentinels are `<!-- hydra:curator:start -->` and `<!-- hydra:curator:end -->`.
- **hydra never deletes authored content.** Teardown leaves `.hydra/skills/` on disk.
- **Shared test helpers.** `readFile(t, path) string` is defined once in `block_test.go` (Task 5) and `mustWrite(t, path, content)` once in `sync_test.go` (Task 6). Later test files use them without redefining — a duplicate definition in the same package will not compile. Tasks must run in order.

---

### Task 1: Demolition to a compiling skeleton

Removes every v0.1 command so later tasks build on clean ground. The repo compiles and tests pass after this task, but hydra only knows `version` and `self-update`.

**Files:**
- Delete: `scope.go`, `scope_test.go`, `config.go`, `init.go`, `init_test.go`, `sync.go`, `sync_test.go`, `new.go`, `new_test.go`, `log.go`, `log_test.go`, `list.go`, `list_test.go`, `doctor.go`, `doctor_test.go`, `review_fixes_test.go`, `cli_test.go`
- Delete: `assets/config`, `assets/curator-block.md`, `assets/curator-reminder.sh`, `assets/skill-curator/SKILL.md`
- Modify: `main.go`, `assets.go`, `assets_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: a `package main` that builds with only `version()`, `newSelfUpdateCmd(out)`, and the file helpers `exists(p) bool`, `isDir(p) bool`, `fileContains(p, sub string) bool`.

- [ ] **Step 1: Create the branch**

```bash
git checkout -b rules-pivot
```

- [ ] **Step 2: Delete the v0.1 command files and their tests**

```bash
git rm scope.go scope_test.go config.go \
       init.go init_test.go \
       sync.go sync_test.go \
       new.go new_test.go \
       log.go log_test.go \
       list.go list_test.go \
       doctor.go doctor_test.go \
       review_fixes_test.go cli_test.go
git rm -r assets
```

- [ ] **Step 3: Rewrite `assets.go` without the asset embed**

Every embedded asset belonged to the curator, and nothing in v0.2 ships file templates —
`init` writes generated content, not copied assets. Only the `VERSION` embed survives.
Replace the whole file with:

```go
package main

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed VERSION
var versionData string

// injectedVersion is set at release time via -ldflags "-X main.injectedVersion=<tag>"
// (see .goreleaser.yaml). When empty (dev/`go build`), the embedded VERSION is used.
var injectedVersion string

func version() string {
	if injectedVersion != "" {
		return injectedVersion
	}
	return strings.TrimSpace(versionData)
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func isDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

func fileContains(p, sub string) bool {
	b, err := os.ReadFile(p)
	return err == nil && strings.Contains(string(b), sub)
}
```

- [ ] **Step 4: Reduce `main.go` to the skeleton**

Replace the whole file with:

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		os.Exit(1)
	}
}

// run builds the root command and executes it against args. It is the testable
// seam: tests drive the CLI through here with in-memory writers.
func run(args []string, out, errw io.Writer) error {
	root := newRootCmd(out, errw)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errw)
	return root.Execute()
}

func newRootCmd(out, errw io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "hydra",
		Short: "hydra — rules library manager for AI coding agents",
		Long:  fmt.Sprintf("hydra %s — manage a library of scoped rules for AI coding agents (Claude Code and others).", version()),
		// Subcommands handle their own error reporting; don't let cobra dump
		// usage text or re-print returned errors (main handles that).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("global", false, "operate on the global scope instead of the current project")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print the hydra version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(out, "hydra %s\n", version())
		},
	})

	root.AddCommand(newSelfUpdateCmd(out))

	return root
}
```

- [ ] **Step 5: Fix `assets_test.go`**

It asserts the deleted assets exist. Replace its body with a version test only:

```go
package main

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesVersionFile(t *testing.T) {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	if got := version(); got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

func TestInjectedVersionWins(t *testing.T) {
	old := injectedVersion
	t.Cleanup(func() { injectedVersion = old })
	injectedVersion = "v9.9.9"
	if got := version(); got != "v9.9.9" {
		t.Errorf("version() = %q, want v9.9.9", got)
	}
}
```

- [ ] **Step 6: Verify it builds and tests pass**

Run: `gofmt -l . && go vet ./... && go test ./... && go build -o /dev/null .`
Expected: no gofmt output, no vet output, `ok  github.com/webteractive/hydra`, build succeeds.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor!: strip the skill-curator machinery down to a skeleton

Removes every v0.1 command, asset, and test. The binary still builds and
knows version and self-update; the rules implementation lands on top."
```

---

### Task 2: The Rule model and frontmatter parsing

**Files:**
- Create: `rule.go`
- Create: `rule_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Rule struct { Name, Path, Title string; Always bool; Paths, Commands, Triggers []string; Body string }`
  - `func ParseRule(name, path, content string) (Rule, error)`
  - `func LoadRules(dir string) ([]Rule, error)` — sorted by `Name`, skips `index.md`, returns empty slice (nil error) when `dir` is absent
  - `func (r Rule) HasMatcher() bool`
  - `func RenderRuleFile(r Rule) string` — frontmatter + body, for `hydra new` and `hydra add`
  - `func headline(name string) string` — `"rust-dependencies"` → `"Rust Dependencies"`

- [ ] **Step 1: Add the yaml dependency**

```bash
go get gopkg.in/yaml.v3@v3.0.1
```

- [ ] **Step 2: Write the failing tests**

Create `rule_test.go`:

```go
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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./... -run 'Rule|HasMatcher|Headline' -v`
Expected: FAIL — `undefined: ParseRule`, `undefined: Rule`, `undefined: LoadRules`, `undefined: RenderRuleFile`.

- [ ] **Step 4: Write `rule.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rule is one Markdown file in the library. Matchers are optional individually,
// but a rule with none of them and Always=false can never fire.
type Rule struct {
	Name     string   `json:"name"`  // filename stem, e.g. "rust-dependencies"
	Path     string   `json:"path"`  // absolute path on disk
	Title    string   `json:"title"` // first H1 in the body, else headline(Name)
	Always   bool     `json:"always"`
	Paths    []string `json:"paths,omitempty"`
	Commands []string `json:"commands,omitempty"`
	Triggers []string `json:"triggers,omitempty"`
	Body     string   `json:"-"` // Markdown after the frontmatter
}

// ruleFrontmatter mirrors Rule's YAML-bearing fields.
type ruleFrontmatter struct {
	Always   bool     `yaml:"always,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
	Triggers []string `yaml:"triggers,omitempty"`
}

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)
	h1Re          = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
)

// ParseRule splits leading YAML frontmatter from the Markdown body. A file with
// no frontmatter is valid and yields a matcher-less rule; malformed YAML is an
// error so doctor can point at the file instead of silently ignoring it.
func ParseRule(name, path, content string) (Rule, error) {
	r := Rule{Name: name, Path: path, Body: content}

	if m := frontmatterRe.FindStringSubmatch(content); m != nil {
		var fm ruleFrontmatter
		if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
			return Rule{}, fmt.Errorf("%s: invalid frontmatter: %w", path, err)
		}
		r.Always = fm.Always
		r.Paths = fm.Paths
		r.Commands = fm.Commands
		r.Triggers = fm.Triggers
		r.Body = content[len(m[0]):]
	}

	r.Body = strings.TrimLeft(r.Body, "\n")
	if m := h1Re.FindStringSubmatch(r.Body); m != nil {
		r.Title = m[1]
	} else {
		r.Title = headline(name)
	}
	return r, nil
}

// HasMatcher reports whether the rule can ever fire.
func (r Rule) HasMatcher() bool {
	return r.Always || len(r.Paths) > 0 || len(r.Commands) > 0 || len(r.Triggers) > 0
}

// LoadRules reads every *.md in dir except index.md, sorted by name. A missing
// directory is not an error — an uninitialized scope simply has no rules.
func LoadRules(dir string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Rule{}, nil
		}
		return nil, err
	}

	rules := []Rule{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == indexFilename {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		r, err := ParseRule(strings.TrimSuffix(e.Name(), ".md"), path, string(data))
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })
	return rules, nil
}

// RenderRuleFile serializes a rule back to disk form: frontmatter then body.
func RenderRuleFile(r Rule) string {
	fm := ruleFrontmatter{Always: r.Always, Paths: r.Paths, Commands: r.Commands, Triggers: r.Triggers}
	var b strings.Builder
	b.WriteString("---\n")
	body := "always: false\n"
	if out, err := yaml.Marshal(fm); err == nil {
		// Every field is omitempty, so an all-zero struct marshals to "{}\n".
		// Emitting that would produce frontmatter ParseRule cannot read back,
		// so fall back to an explicit always: false.
		if s := string(out); s != "{}\n" {
			body = s
		}
	}
	b.WriteString(body)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimLeft(r.Body, "\n"))
	return b.String()
}

// headline turns a kebab-case filename into a display title.
func headline(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// indexFilename is the generated table; it is never itself a rule.
const indexFilename = "index.md"
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS for every `TestParseRule*`, `TestHasMatcher`, `TestLoadRules*`, `TestRenderRuleFileRoundTrips`.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go vet ./...
git add rule.go rule_test.go go.mod go.sum
git commit -m "feat: add the Rule model and frontmatter parsing

Rules are Markdown with optional always/paths/commands/triggers frontmatter.
Adds gopkg.in/yaml.v3 so both flow and block list styles parse."
```

---

### Task 3: Scope rewrite and harness detection

**Files:**
- Create: `scope.go`
- Create: `scope_test.go`
- Create: `detect.go`
- Create: `detect_test.go`

**Interfaces:**
- Consumes: `Rule` (Task 2).
- Produces:
  - `type Scope struct { Global bool; Label, Base, Home, RulesDir string }`
  - `func ResolveScope(global bool, cwd, home string) Scope`
  - `func (s Scope) RuleRef(r Rule) string` — absolute in global scope, `.hydra/rules/<name>.md` in project scope
  - `func (s Scope) RulesDirRef() string` — the same treatment for the grep hint
  - `func scopeFromCmd(cmd *cobra.Command) Scope` — resolves the scope from the persistent `--global` flag; every command file uses it
  - `func candidateTargets(s Scope) []string`
  - `func DetectTargets(s Scope) []string` — the candidates that exist
  - `func DefaultTargets(s Scope) []string` — what `init` creates when none exist

- [ ] **Step 1: Write the failing tests**

Create `scope_test.go`:

```go
package main

import "testing"

func TestResolveScopeProject(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	if s.Home != "/work/app/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.RulesDir != "/work/app/.hydra/rules" {
		t.Errorf("RulesDir = %s", s.RulesDir)
	}
	if s.Label != "project" {
		t.Errorf("Label = %s", s.Label)
	}
	if s.Global {
		t.Error("Global = true for a project scope")
	}
}

func TestResolveScopeGlobal(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	if s.Home != "/home/u/.hydra" {
		t.Errorf("Home = %s", s.Home)
	}
	if s.RulesDir != "/home/u/.hydra/rules" {
		t.Errorf("RulesDir = %s", s.RulesDir)
	}
	if s.Label != "global" {
		t.Errorf("Label = %s", s.Label)
	}
}

func TestRuleRefRelativeInProject(t *testing.T) {
	s := ResolveScope(false, "/work/app", "/home/u")
	r := Rule{Name: "rust", Path: "/work/app/.hydra/rules/rust.md"}
	if got := s.RuleRef(r); got != ".hydra/rules/rust.md" {
		t.Errorf("RuleRef = %s want .hydra/rules/rust.md", got)
	}
	if got := s.RulesDirRef(); got != ".hydra/rules" {
		t.Errorf("RulesDirRef = %s want .hydra/rules", got)
	}
}

func TestRuleRefAbsoluteInGlobal(t *testing.T) {
	s := ResolveScope(true, "/work/app", "/home/u")
	r := Rule{Name: "rust", Path: "/home/u/.hydra/rules/rust.md"}
	if got := s.RuleRef(r); got != "/home/u/.hydra/rules/rust.md" {
		t.Errorf("RuleRef = %s", got)
	}
	if got := s.RulesDirRef(); got != "/home/u/.hydra/rules" {
		t.Errorf("RulesDirRef = %s", got)
	}
}
```

Create `detect_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectTargetsProject(t *testing.T) {
	tmp := t.TempDir()
	touch(t, filepath.Join(tmp, "CLAUDE.md"))
	touch(t, filepath.Join(tmp, "GEMINI.md"))

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	want := []string{filepath.Join(tmp, "CLAUDE.md"), filepath.Join(tmp, "GEMINI.md")}
	if got := DetectTargets(s); !reflect.DeepEqual(got, want) {
		t.Errorf("DetectTargets = %v want %v", got, want)
	}
}

func TestDetectTargetsGlobal(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	touch(t, filepath.Join(home, ".codex", "AGENTS.md"))

	s := ResolveScope(true, tmp, home)
	want := []string{filepath.Join(home, ".codex", "AGENTS.md")}
	if got := DetectTargets(s); !reflect.DeepEqual(got, want) {
		t.Errorf("DetectTargets = %v want %v", got, want)
	}
}

func TestDetectTargetsNoneFound(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	if got := DetectTargets(s); len(got) != 0 {
		t.Errorf("DetectTargets = %v want empty", got)
	}
}

func TestDefaultTargets(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")

	proj := DefaultTargets(ResolveScope(false, tmp, home))
	wantProj := []string{filepath.Join(tmp, "CLAUDE.md"), filepath.Join(tmp, "AGENTS.md")}
	if !reflect.DeepEqual(proj, wantProj) {
		t.Errorf("project defaults = %v want %v", proj, wantProj)
	}

	glob := DefaultTargets(ResolveScope(true, tmp, home))
	wantGlob := []string{filepath.Join(home, ".claude", "CLAUDE.md")}
	if !reflect.DeepEqual(glob, wantGlob) {
		t.Errorf("global defaults = %v want %v", glob, wantGlob)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'Scope|RuleRef|Targets' -v`
Expected: FAIL — `undefined: ResolveScope`, `undefined: DetectTargets`, `undefined: DefaultTargets`.

- [ ] **Step 3: Write `scope.go`**

```go
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Scope is one rules library plus how its paths should be written into agent
// instruction files. Project and global scopes are fully independent: neither
// reads the other, and hydra never merges them.
type Scope struct {
	Global   bool   `json:"global"`
	Label    string `json:"label"`     // "project" | "global"
	Base     string `json:"base"`      // cwd for project, home for global
	Home     string `json:"home"`      // <Base>/.hydra
	RulesDir string `json:"rules_dir"` // <Base>/.hydra/rules
}

func ResolveScope(global bool, cwd, home string) Scope {
	base, label := cwd, "project"
	if global {
		base, label = home, "global"
	}
	hydraHome := filepath.Join(base, ".hydra")
	return Scope{
		Global:   global,
		Label:    label,
		Base:     base,
		Home:     hydraHome,
		RulesDir: filepath.Join(hydraHome, "rules"),
	}
}

// RuleRef renders a rule's path as it should appear in an instruction file.
// Global scope must be absolute: ~/.claude/CLAUDE.md is loaded from whatever
// working directory the agent is in, so a relative path resolves against the
// wrong repo. Project scope stays relative so it survives a clone elsewhere.
func (s Scope) RuleRef(r Rule) string {
	return s.ref(r.Path)
}

// RulesDirRef renders the library directory for the grep hint.
func (s Scope) RulesDirRef() string {
	return s.ref(s.RulesDir)
}

func (s Scope) ref(path string) string {
	if s.Global {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(s.Base, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return strings.TrimPrefix(filepath.ToSlash(rel), "./")
}

// scopeFromCmd resolves the active Scope honoring the persistent --global flag.
// It lives here rather than in main.go so every command file can reach it
// without depending on the order commands get wired up.
func scopeFromCmd(cmd *cobra.Command) Scope {
	global, _ := cmd.Flags().GetBool("global")
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return ResolveScope(global, cwd, home)
}
```

- [ ] **Step 4: Write `detect.go`**

```go
package main

import "path/filepath"

// candidateTargets lists the agent instruction files hydra knows how to write,
// in the order they should be reported. Targets are detected, never configured.
func candidateTargets(s Scope) []string {
	if s.Global {
		return []string{
			filepath.Join(s.Base, ".claude", "CLAUDE.md"),
			filepath.Join(s.Base, ".codex", "AGENTS.md"),
			filepath.Join(s.Base, ".gemini", "GEMINI.md"),
		}
	}
	return []string{
		filepath.Join(s.Base, "CLAUDE.md"),
		filepath.Join(s.Base, "AGENTS.md"),
		filepath.Join(s.Base, "GEMINI.md"),
	}
}

// DetectTargets returns the candidates that already exist.
func DetectTargets(s Scope) []string {
	found := []string{}
	for _, c := range candidateTargets(s) {
		if exists(c) {
			found = append(found, c)
		}
	}
	return found
}

// DefaultTargets is what init creates when detection finds nothing. A project
// gets the CLAUDE.md/AGENTS.md mirror pair; a home directory gets Claude Code's
// file, since creating config directories for agents that aren't installed
// would be presumptuous.
func DefaultTargets(s Scope) []string {
	if s.Global {
		return []string{filepath.Join(s.Base, ".claude", "CLAUDE.md")}
	}
	return []string{
		filepath.Join(s.Base, "CLAUDE.md"),
		filepath.Join(s.Base, "AGENTS.md"),
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go vet ./...
git add scope.go scope_test.go detect.go detect_test.go
git commit -m "feat: add rules-scoped Scope and harness target detection

Project scope renders relative refs, global renders absolute. Instruction
files are detected rather than configured."
```

---

### Task 4: Rendering the index and the managed block

**Files:**
- Create: `render.go`
- Create: `render_test.go`

**Interfaces:**
- Consumes: `Rule`, `Scope`, `Scope.RuleRef`, `Scope.RulesDirRef` (Tasks 2–3).
- Produces:
  - `func RenderIndex(s Scope, rules []Rule) string` — full `index.md` content
  - `func RenderBlock(s Scope, rules []Rule) string` — content **including** both sentinels
  - `const blockStart = "<!-- hydra:rules:start -->"`, `const blockEnd = "<!-- hydra:rules:end -->"`

- [ ] **Step 1: Write the failing tests**

Create `render_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run Render -v`
Expected: FAIL — `undefined: RenderIndex`, `undefined: RenderBlock`, `undefined: blockStart`.

- [ ] **Step 3: Write `render.go`**

```go
package main

import (
	"fmt"
	"strings"
)

const (
	blockStart = "<!-- hydra:rules:start -->"
	blockEnd   = "<!-- hydra:rules:end -->"
)

// RenderIndex produces the canonical index.md: the same table the managed block
// carries, plus a header explaining what the file is.
func RenderIndex(s Scope, rules []Rule) string {
	var b strings.Builder
	b.WriteString("# Hydra Rules Index\n\n")
	b.WriteString("<!-- generated by hydra — do not edit -->\n\n")
	b.WriteString("Find the rows whose trigger or match covers what you are about to do,\n")
	b.WriteString("then read those rule files.\n\n")
	b.WriteString(indexTable(s, rules))
	return b.String()
}

// RenderBlock produces the managed block, sentinels included. Always-rules are
// inlined verbatim; everything else is one table row. Inlining the table (rather
// than pointing at index.md, as Laravel Boost does) means deciding which rules
// apply costs the agent no file reads at all.
func RenderBlock(s Scope, rules []Rule) string {
	var always, indexed []Rule
	for _, r := range rules {
		if r.Always {
			always = append(always, r)
		} else {
			indexed = append(indexed, r)
		}
	}

	var b strings.Builder
	b.WriteString(blockStart + "\n")
	b.WriteString("## Rules\n\n")

	if len(always) > 0 {
		b.WriteString("Always-on rules are inlined below. The rest are lazy-loaded. ")
	}
	b.WriteString("**Before you enter plan mode, run a command, or create/edit a file, you MUST\n")
	b.WriteString("first:** find the index rows whose trigger, glob, or command covers what you are\n")
	b.WriteString("about to do and read those rule files, then run\n")
	b.WriteString(fmt.Sprintf("`grep -rin '<keyword>' %s` to catch what the index alone misses. Do not act\n", s.RulesDirRef()))
	b.WriteString("until you are following every matching rule.\n")

	if !s.Global {
		b.WriteString("\nProject rules override global rules on conflict.\n")
	}

	if len(always) > 0 {
		b.WriteString("\n### Always applies\n")
		for _, r := range always {
			b.WriteString("\n")
			b.WriteString(strings.TrimRight(r.Body, "\n"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n### Rules index\n\n")
	b.WriteString(indexTable(s, indexed))
	b.WriteString(blockEnd + "\n")
	return b.String()
}

// indexTable renders the matcher table. Always-rules are never listed: their
// bodies are already in the block, so a row would be dead weight.
func indexTable(s Scope, rules []Rule) string {
	var rows []string
	for _, r := range rules {
		if r.Always {
			continue
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %s |",
			joinPlain(r.Triggers),
			joinCode(append(append([]string{}, r.Paths...), r.Commands...)),
			s.RuleRef(r),
		))
	}
	if len(rows) == 0 {
		return "No rules recorded yet.\n"
	}
	return "| Applies when | Matches (path glob / command) | Rule |\n| --- | --- | --- |\n" +
		strings.Join(rows, "\n") + "\n"
}

// joinPlain joins trigger phrases; an em-separator keeps multi-trigger rows readable.
func joinPlain(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	return strings.Join(items, " · ")
}

// joinCode joins globs and commands, wrapping each in backticks.
func joinCode(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	quoted := make([]string, 0, len(items))
	for _, it := range items {
		quoted = append(quoted, "`"+it+"`")
	}
	return strings.Join(quoted, " · ")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add render.go render_test.go
git commit -m "feat: render the rules index and the managed instruction block

The block inlines the table rather than pointing at index.md, so deciding
which rules apply costs the agent no file reads."
```

---

### Task 5: Sentinel splicing into instruction files

**Files:**
- Create: `block.go`
- Create: `block_test.go`

**Interfaces:**
- Consumes: `blockStart`, `blockEnd` (Task 4).
- Produces:
  - `func SpliceBlock(path, block string) error` — replace in place, else append; creates the file and its parent directory
  - `func StripBlock(path, start, end string) (bool, error)` — remove a delimited block, reporting whether one was found

- [ ] **Step 1: Write the failing tests**

Create `block_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSpliceBlockCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "CLAUDE.md")
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	if err := SpliceBlock(path, block); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != block {
		t.Errorf("content = %q want %q", got, block)
	}
}

func TestSpliceBlockAppendsPreservingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Project\n\nExisting guidance.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	if err := SpliceBlock(path, block); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.HasPrefix(got, "# Project\n\nExisting guidance.\n") {
		t.Errorf("existing content lost: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("block not appended: %q", got)
	}
}

func TestSpliceBlockReplacesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	original := "# Project\n\n" + blockStart + "\nold\n" + blockEnd + "\n\n## After\n\nTrailing guidance.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SpliceBlock(path, blockStart+"\nnew\n"+blockEnd+"\n"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "old") {
		t.Errorf("old block survived: %q", got)
	}
	if !strings.Contains(got, "new") {
		t.Errorf("new block missing: %q", got)
	}
	if !strings.Contains(got, "## After") || !strings.Contains(got, "# Project") {
		t.Errorf("surrounding content disturbed: %q", got)
	}
	if strings.Count(got, blockStart) != 1 {
		t.Errorf("expected exactly one block, got %d: %q", strings.Count(got, blockStart), got)
	}
}

func TestSpliceBlockIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	for i := 0; i < 3; i++ {
		if err := SpliceBlock(path, block); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFile(t, path); strings.Count(got, blockStart) != 1 {
		t.Errorf("repeated splices duplicated the block: %q", got)
	}
}

func TestStripBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	content := "# Project\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n\n## After\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := StripBlock(path, "<!-- hydra:curator:start -->", "<!-- hydra:curator:end -->")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("found = false, want true")
	}
	got := readFile(t, path)
	if strings.Contains(got, "curator") {
		t.Errorf("block survived: %q", got)
	}
	if !strings.Contains(got, "# Project") || !strings.Contains(got, "## After") {
		t.Errorf("surrounding content lost: %q", got)
	}
}

func TestStripBlockAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := StripBlock(path, "<!-- hydra:curator:start -->", "<!-- hydra:curator:end -->")
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}

func TestStripBlockMissingFile(t *testing.T) {
	found, err := StripBlock(filepath.Join(t.TempDir(), "nope.md"), blockStart, blockEnd)
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'SpliceBlock|StripBlock' -v`
Expected: FAIL — `undefined: SpliceBlock`, `undefined: StripBlock`.

- [ ] **Step 3: Write `block.go`**

```go
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var multiNewline = regexp.MustCompile(`\n{3,}`)

// SpliceBlock writes block into path between its sentinels. An existing block is
// replaced in place so re-running never appends a second copy and never disturbs
// what the user wrote around it; otherwise the block is appended. The file and
// its parent directory are created if missing.
func SpliceBlock(path, block string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	var content string
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	block = strings.TrimRight(block, "\n") + "\n"

	var updated string
	if start := strings.Index(content, blockStart); start != -1 {
		if end := strings.Index(content[start:], blockEnd); end != -1 {
			tail := content[start+end+len(blockEnd):]
			updated = content[:start] + block + strings.TrimLeft(tail, "\n")
		}
	}
	if updated == "" {
		head := strings.TrimRight(content, "\n")
		if head == "" {
			updated = block
		} else {
			updated = head + "\n\n" + block
		}
	}

	updated = multiNewline.ReplaceAllString(updated, "\n\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return os.WriteFile(path, []byte(updated), mode)
}

// StripBlock removes a sentinel-delimited block from path, reporting whether one
// was there. A missing file is not an error — there is simply nothing to strip.
func StripBlock(path, start, end string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	content := string(data)

	i := strings.Index(content, start)
	if i == -1 {
		return false, nil
	}
	j := strings.Index(content[i:], end)
	if j == -1 {
		return false, nil
	}

	updated := content[:i] + strings.TrimLeft(content[i+j+len(end):], "\n")
	updated = multiNewline.ReplaceAllString(updated, "\n\n")
	if strings.TrimSpace(updated) != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(updated), mode)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add block.go block_test.go
git commit -m "feat: splice the managed block into instruction files

Replaces in place when a block exists so repeated syncs never duplicate it,
and preserves everything written around it."
```

---

### Task 6: `hydra sync`

**Files:**
- Create: `sync.go`
- Create: `sync_test.go`

**Interfaces:**
- Consumes: `LoadRules`, `RenderIndex`, `RenderBlock`, `SpliceBlock`, `DetectTargets` (Tasks 2–5).
- Produces: `func Sync(s Scope, out io.Writer) error` — reindexes and rewrites every detected target. Returns an error when `s.RulesDir` does not exist; **never creates it**.

- [ ] **Step 1: Write the failing tests**

Create `sync_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncWritesIndexAndBlocks(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\n---\n\n# Rust\n\nPin versions.\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}

	index := readFile(t, filepath.Join(tmp, ".hydra", "rules", "index.md"))
	if !strings.Contains(index, ".hydra/rules/rust.md") {
		t.Errorf("index missing the rule: %s", index)
	}
	for _, target := range []string{"CLAUDE.md", "AGENTS.md"} {
		got := readFile(t, filepath.Join(tmp, target))
		if !strings.Contains(got, blockStart) {
			t.Errorf("%s missing block: %s", target, got)
		}
		if !strings.Contains(got, "# App") {
			t.Errorf("%s lost its own content: %s", target, got)
		}
	}
	if !strings.Contains(out.String(), "2 target") {
		t.Errorf("output should name the target count: %s", out.String())
	}
}

func TestSyncIdempotent(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := Sync(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if n := strings.Count(got, blockStart); n != 1 {
		t.Errorf("block count = %d want 1: %s", n, got)
	}
}

func TestSyncRefusesUninitialized(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err == nil {
		t.Fatal("expected an error for an uninitialized scope")
	}
	if exists(filepath.Join(tmp, ".hydra")) {
		t.Error("sync must not create the library")
	}
}

func TestSyncWarnsWhenNoTargets(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "a.md"), "---\npaths: [\"a/**\"]\n---\n\n# A\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no agent instruction files") {
		t.Errorf("expected a no-targets warning: %s", out.String())
	}
}

func TestSyncReportsMatcherlessRule(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "orphan.md"), "# Orphan\n\nno matchers\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"), "# App\n")

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Sync(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "orphan") || !strings.Contains(out.String(), "can never fire") {
		t.Errorf("expected a matcher-less warning: %s", out.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run Sync -v`
Expected: FAIL — `undefined: Sync`.

- [ ] **Step 3: Write `sync.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Sync reparses the library and rewrites both generated artifacts: index.md and
// the managed block in every detected instruction file. It deliberately does not
// create the library — that is init's job, so a typo'd directory is reported
// rather than silently scaffolded.
func Sync(s Scope, out io.Writer) error {
	if !isDir(s.RulesDir) {
		return fmt.Errorf("no rules library at %s — run 'hydra init' first", s.RulesDir)
	}

	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		return err
	}

	for _, r := range rules {
		if !r.HasMatcher() {
			fmt.Fprintf(out, "warning: %s has no paths, commands, or triggers and is not always:true — it can never fire\n", r.Name)
		}
	}

	indexPath := filepath.Join(s.RulesDir, indexFilename)
	if err := os.WriteFile(indexPath, []byte(RenderIndex(s, rules)), 0o644); err != nil {
		return err
	}

	targets := DetectTargets(s)
	if len(targets) == 0 {
		fmt.Fprintf(out, "warning: no agent instruction files found for the %s scope — run 'hydra init' to create them\n", s.Label)
	}
	block := RenderBlock(s, rules)
	for _, t := range targets {
		if err := SpliceBlock(t, block); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "indexed %d rule(s) → %d target(s)\n", len(rules), len(targets))
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add sync.go sync_test.go
git commit -m "feat: add hydra sync

Reparses the library and rewrites index.md plus the managed block in every
detected instruction file. Never creates the library."
```

---

### Task 7: v0.1 teardown

**Files:**
- Create: `teardown.go`
- Create: `teardown_test.go`

**Interfaces:**
- Consumes: `StripBlock` (Task 5), `Scope` (Task 3).
- Produces:
  - `func Teardown(s Scope, out io.Writer) (bool, error)` — reports whether any v0.1 artifact was found and removed
  - `func removeHook(data map[string]any, marker string) bool` — deletes matching `UserPromptSubmit` entries from parsed settings JSON

- [ ] **Step 1: Write the failing tests**

Create `teardown_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveHookPreservesOtherSettings(t *testing.T) {
	raw := `{
	  "model": "opus",
	  "hooks": {
	    "UserPromptSubmit": [
	      {"hooks": [{"type": "command", "command": "$CLAUDE_PROJECT_DIR/.hydra/curator-reminder.sh"}]},
	      {"hooks": [{"type": "command", "command": "other.sh"}]}
	    ],
	    "Stop": [{"hooks": [{"type": "command", "command": "keep.sh"}]}]
	  }
	}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	if !removeHook(data, "curator-reminder.sh") {
		t.Fatal("removeHook = false, want true")
	}
	out, _ := json.Marshal(data)
	s := string(out)
	if strings.Contains(s, "curator-reminder") {
		t.Errorf("curator hook survived: %s", s)
	}
	for _, keep := range []string{"other.sh", "keep.sh", "opus"} {
		if !strings.Contains(s, keep) {
			t.Errorf("%s was removed: %s", keep, s)
		}
	}
}

func TestRemoveHookDropsEmptyKeys(t *testing.T) {
	var data map[string]any
	raw := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"x/curator-reminder.sh"}]}]}}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	removeHook(data, "curator-reminder.sh")
	if _, present := data["hooks"]; present {
		t.Errorf("empty hooks key should be dropped: %+v", data)
	}
}

func TestTeardownRemovesEverythingButSkills(t *testing.T) {
	tmp := t.TempDir()

	// v0.1 wreckage
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator-reminder.sh"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator.log"), "log\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "config"), "HYDRA_RUNTIMES=\"claude\"\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "skills", "keep-me", "SKILL.md"), "authored\n")
	mustWrite(t, filepath.Join(tmp, ".claude", "settings.json"),
		`{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":".hydra/curator-reminder.sh"}]}]}}`)
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"),
		"# App\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n")

	// a dangling symlink, exactly the state this repo is in
	if err := os.MkdirAll(filepath.Join(tmp, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../skills/gone", filepath.Join(tmp, ".claude", "skills", "gone")); err != nil {
		t.Fatal(err)
	}

	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	found, err := Teardown(s, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("found = false, want true")
	}

	for _, gone := range []string{
		filepath.Join(tmp, ".hydra", "curator-reminder.sh"),
		filepath.Join(tmp, ".hydra", "curator.log"),
		filepath.Join(tmp, ".hydra", "config"),
		filepath.Join(tmp, ".claude", "skills"),
	} {
		if exists(gone) {
			t.Errorf("%s should have been removed", gone)
		}
	}
	if !exists(filepath.Join(tmp, ".hydra", "skills", "keep-me", "SKILL.md")) {
		t.Error("authored skills must survive teardown")
	}
	if strings.Contains(readFile(t, filepath.Join(tmp, "CLAUDE.md")), "curator") {
		t.Error("curator block survived")
	}
	if strings.Contains(readFile(t, filepath.Join(tmp, ".claude", "settings.json")), "curator-reminder") {
		t.Error("hook survived")
	}
	if !strings.Contains(out.String(), "keep-me") && !strings.Contains(out.String(), "kept") {
		t.Errorf("teardown should say what it kept: %s", out.String())
	}
}

func TestTeardownNoopOnCleanScope(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	found, err := Teardown(s, &out)
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'Teardown|removeHook|RemoveHook' -v`
Expected: FAIL — `undefined: Teardown`, `undefined: removeHook`.

- [ ] **Step 3: Write `teardown.go`**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	curatorBlockStart = "<!-- hydra:curator:start -->"
	curatorBlockEnd   = "<!-- hydra:curator:end -->"
	curatorHookMarker = "curator-reminder.sh"
)

// Teardown removes every v0.1 skill-curator artifact and reports whether it
// found any. Authored content is never destroyed: .hydra/skills/ is left in
// place and named in the output so it can be salvaged by hand.
func Teardown(s Scope, out io.Writer) (bool, error) {
	found := false

	for _, name := range []string{curatorHookMarker, "curator.log", "config"} {
		p := filepath.Join(s.Home, name)
		if exists(p) {
			if err := os.Remove(p); err != nil {
				return found, err
			}
			fmt.Fprintf(out, "  removed  %s\n", p)
			found = true
		}
	}

	for _, dir := range []string{
		filepath.Join(s.Base, ".claude", "skills"),
		filepath.Join(s.Base, ".agents", "skills"),
	} {
		removed, err := removeSymlinkFarm(dir, out)
		if err != nil {
			return found, err
		}
		found = found || removed
	}

	settings := filepath.Join(s.Base, ".claude", "settings.json")
	unwired, err := unwireHook(settings, out)
	if err != nil {
		return found, err
	}
	found = found || unwired

	for _, t := range candidateTargets(s) {
		stripped, err := StripBlock(t, curatorBlockStart, curatorBlockEnd)
		if err != nil {
			return found, err
		}
		if stripped {
			fmt.Fprintf(out, "  stripped curator block from %s\n", t)
			found = true
		}
	}

	skills := filepath.Join(s.Home, "skills")
	if isDir(skills) {
		entries, _ := os.ReadDir(skills)
		fmt.Fprintf(out, "  kept     %s (%d skill(s)) — salvage or delete by hand\n", skills, len(entries))
		found = true
	}

	return found, nil
}

// removeSymlinkFarm deletes symlinks hydra created, including dangling ones
// (os.Stat fails on those, so detection goes through Lstat). Real files are left
// alone, and the directory itself only goes if it ends up empty.
func removeSymlinkFarm(dir string, out io.Writer) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, nil
	}
	removed := false
	for _, e := range entries {
		link := filepath.Join(dir, e.Name())
		fi, err := os.Lstat(link)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if err := os.Remove(link); err != nil {
			return removed, err
		}
		removed = true
	}
	if remaining, err := os.ReadDir(dir); err == nil && len(remaining) == 0 {
		if err := os.Remove(dir); err != nil {
			return removed, err
		}
		fmt.Fprintf(out, "  removed  %s\n", dir)
		removed = true
	}
	return removed, nil
}

// unwireHook strips the curator hook from settings.json, leaving every other
// setting untouched. Unparseable JSON is reported, not rewritten.
func unwireHook(path string, out io.Writer) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	if !strings.Contains(string(raw), curatorHookMarker) {
		return false, nil
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		fmt.Fprintf(out, "  warning  %s is not valid JSON — remove the curator hook by hand\n", path)
		return false, nil
	}
	if !removeHook(data, curatorHookMarker) {
		return false, nil
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, err
	}
	b = append(b, '\n')
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		return false, err
	}
	fmt.Fprintf(out, "  removed  UserPromptSubmit hook from %s\n", path)
	return true, nil
}

// removeHook deletes UserPromptSubmit groups whose command contains marker,
// pruning keys that end up empty. Returns whether anything changed.
func removeHook(data map[string]any, marker string) bool {
	hooks, ok := data["hooks"].(map[string]any)
	if !ok {
		return false
	}
	ups, ok := hooks["UserPromptSubmit"].([]any)
	if !ok {
		return false
	}

	kept := make([]any, 0, len(ups))
	changed := false
	for _, g := range ups {
		if groupHasCommand(g, marker) {
			changed = true
			continue
		}
		kept = append(kept, g)
	}
	if !changed {
		return false
	}

	if len(kept) == 0 {
		delete(hooks, "UserPromptSubmit")
	} else {
		hooks["UserPromptSubmit"] = kept
	}
	if len(hooks) == 0 {
		delete(data, "hooks")
	}
	return true
}

func groupHasCommand(group any, marker string) bool {
	gm, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hs, ok := gm["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hs {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if c, _ := hm["command"].(string); strings.Contains(c, marker) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add teardown.go teardown_test.go
git commit -m "feat: tear down v0.1 skill-curator artifacts

Removes the hook, the symlink farms (dangling links included), the curator
block, curator.log and config. Authored .hydra/skills/ is kept and reported."
```

---

### Task 8: `hydra init`

**Files:**
- Create: `init.go`
- Create: `init_test.go`

**Interfaces:**
- Consumes: `Teardown` (Task 7), `DetectTargets`, `DefaultTargets` (Task 3), `Sync` (Task 6).
- Produces: `func Init(s Scope, out io.Writer) error` — idempotent. Runs teardown, creates `RulesDir` if missing, creates default targets when detection finds none, then syncs.

- [ ] **Step 1: Write the failing tests**

Create `init_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFreshProject(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if !isDir(filepath.Join(tmp, ".hydra", "rules")) {
		t.Error("rules dir not created")
	}
	if !exists(filepath.Join(tmp, ".hydra", "rules", "index.md")) {
		t.Error("index.md not created")
	}
	for _, target := range []string{"CLAUDE.md", "AGENTS.md"} {
		p := filepath.Join(tmp, target)
		if !exists(p) {
			t.Fatalf("%s not created", target)
		}
		if !strings.Contains(readFile(t, p), blockStart) {
			t.Errorf("%s missing the block", target)
		}
	}
}

func TestInitUsesExistingTargetsOnly(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "AGENTS.md"), "# App\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, "CLAUDE.md")) {
		t.Error("init created CLAUDE.md even though AGENTS.md was detected")
	}
	if !strings.Contains(readFile(t, filepath.Join(tmp, "AGENTS.md")), blockStart) {
		t.Error("AGENTS.md missing the block")
	}
}

func TestInitIdempotent(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := Init(s, &out); err != nil {
			t.Fatal(err)
		}
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if n := strings.Count(got, blockStart); n != 1 {
		t.Errorf("block count = %d want 1", n)
	}
}

func TestInitPreservesExistingRules(t *testing.T) {
	tmp := t.TempDir()
	rule := filepath.Join(tmp, ".hydra", "rules", "keep.md")
	mustWrite(t, rule, "---\npaths: [\"a/**\"]\n---\n\n# Keep\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, rule), "# Keep") {
		t.Error("existing rule was clobbered")
	}
}

func TestInitRunsTeardown(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "curator-reminder.sh"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(tmp, "CLAUDE.md"),
		"# App\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n")
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(tmp, ".hydra", "curator-reminder.sh")) {
		t.Error("hook script survived init")
	}
	got := readFile(t, filepath.Join(tmp, "CLAUDE.md"))
	if strings.Contains(got, "hydra:curator") {
		t.Error("curator block survived init")
	}
	if !strings.Contains(got, blockStart) {
		t.Error("rules block not written")
	}
}

func TestInitGlobalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	s := ResolveScope(true, tmp, home)

	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".claude", "CLAUDE.md")
	if !exists(target) {
		t.Fatal("global CLAUDE.md not created")
	}
	got := readFile(t, target)
	if !strings.Contains(got, filepath.Join(home, ".hydra", "rules")) {
		t.Errorf("global block should reference an absolute rules dir: %s", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run Init -v`
Expected: FAIL — `undefined: Init`.

- [ ] **Step 3: Write `init.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
)

// Init scaffolds the rules library and wires the managed block into the scope's
// instruction files. It is idempotent: existing rules and instruction files are
// left alone, only missing pieces are created. Every mutating command calls it,
// so recording a rule in a fresh project just works.
func Init(s Scope, out io.Writer) error {
	if found, err := Teardown(s, out); err != nil {
		return err
	} else if found {
		fmt.Fprintln(out, "cleaned up a v0.1 skill-curator install")
	}

	if !isDir(s.RulesDir) {
		if err := os.MkdirAll(s.RulesDir, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "created %s\n", s.RulesDir)
	}

	if len(DetectTargets(s)) == 0 {
		for _, t := range DefaultTargets(s) {
			if err := SpliceBlock(t, blockStart+"\n"+blockEnd+"\n"); err != nil {
				return err
			}
			fmt.Fprintf(out, "created %s\n", t)
		}
	}

	if err := Sync(s, out); err != nil {
		return err
	}

	fmt.Fprintf(out, "hydra init complete (%s: %s)\n", s.Label, s.RulesDir)
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add init.go init_test.go
git commit -m "feat: add hydra init

Idempotent: tears down v0.1 artifacts, creates the rules library and default
instruction files only when missing, then syncs."
```

---

### Task 9: `hydra add`

**Files:**
- Create: `add.go`
- Create: `add_test.go`

**Interfaces:**
- Consumes: `Rule`, `LoadRules`, `RenderRuleFile` (Task 2), `Init` (Task 8), `Sync` (Task 6).
- Produces:
  - `type AddRequest struct { Title, Note string; Always bool; Paths, Commands, Triggers []string }`
  - `func Add(s Scope, req AddRequest, out io.Writer) error`
  - `func areaKey(req AddRequest) string`
  - `func slugify(s string) string`

- [ ] **Step 1: Write the failing tests**

Create `add_test.go`:

```go
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
		{Note: "n", Paths: []string{"a/**"}},                 // no title
		{Title: "T", Paths: []string{"a/**"}},                // no note
		{Title: "T", Note: "n"},                              // no matcher, not always
	}
	for i, req := range cases {
		if err := Add(s, req, &out); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'Add|AreaKey' -v`
Expected: FAIL — `undefined: Add`, `undefined: AddRequest`, `undefined: areaKey`.

- [ ] **Step 3: Write `add.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AddRequest is one recorded rule: what it says, and when it fires.
type AddRequest struct {
	Title    string
	Note     string
	Always   bool
	Paths    []string
	Commands []string
	Triggers []string
}

// Add files a rule into the library, creating the library first when it does not
// exist so an agent recording a rule in a fresh project never hits a setup error.
// Rules covering the same area land in the same file rather than scattering.
func Add(s Scope, req AddRequest, out io.Writer) error {
	req.Title = strings.TrimSpace(strings.ReplaceAll(req.Title, "\n", " "))
	req.Note = strings.TrimSpace(req.Note)

	if req.Title == "" {
		return fmt.Errorf("a rule needs --title")
	}
	if req.Note == "" {
		return fmt.Errorf("a rule needs --note")
	}
	if !req.Always && len(req.Paths) == 0 && len(req.Commands) == 0 && len(req.Triggers) == 0 {
		return fmt.Errorf("a rule needs at least one --glob, --command, or --trigger, or --always")
	}

	if !isDir(s.RulesDir) {
		if err := Init(s, out); err != nil {
			return err
		}
	}

	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		return err
	}

	area := areaKey(req)
	target, existing := findArea(rules, area)

	rule := Rule{Name: target, Path: filepath.Join(s.RulesDir, target+".md")}
	if existing != nil {
		rule = *existing
	} else {
		rule.Body = "# " + headline(target) + "\n"
	}

	rule.Always = rule.Always || req.Always
	rule.Paths = mergeUnique(rule.Paths, req.Paths)
	rule.Commands = mergeUnique(rule.Commands, req.Commands)
	rule.Triggers = mergeUnique(rule.Triggers, req.Triggers)
	rule.Body = strings.TrimRight(rule.Body, "\n") + "\n\n## " + req.Title + "\n" + req.Note + "\n"

	if err := os.WriteFile(rule.Path, []byte(RenderRuleFile(rule)), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "recorded in %s: %s\n", s.ref(rule.Path), req.Title)

	return Sync(s, out)
}

// findArea returns the filename for an area plus the rule already occupying it,
// if any. Area keys are derived deterministically, so two rules for the same
// area always resolve to the same file and share it.
func findArea(rules []Rule, area string) (string, *Rule) {
	for i := range rules {
		if rules[i].Name == area {
			return area, &rules[i]
		}
	}
	return area, nil
}

var (
	wildcardSeg = regexp.MustCompile(`[*?\[\]]`)
	nonSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

// areaKey picks the file a rule belongs in. Precedence is fixed regardless of
// flag order: first glob, then first command, then the title.
func areaKey(req AddRequest) string {
	if len(req.Paths) > 0 {
		if seg := lastMeaningfulSegment(req.Paths[0]); seg != "" {
			return slugify(seg)
		}
	}
	if len(req.Commands) > 0 {
		if fields := strings.Fields(req.Commands[0]); len(fields) > 0 {
			return slugify(fields[0])
		}
	}
	return slugify(req.Title)
}

// lastMeaningfulSegment takes the deepest path segment that is neither a
// wildcard nor a filename, e.g. "app/Http/Controllers/**" -> "Controllers".
func lastMeaningfulSegment(glob string) string {
	segments := strings.Split(filepath.ToSlash(glob), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if seg == "" || wildcardSeg.MatchString(seg) || strings.Contains(seg, ".") {
			continue
		}
		return seg
	}
	return ""
}

func slugify(s string) string {
	s = nonSlug.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "general"
	}
	return s
}

func mergeUnique(existing, added []string) []string {
	seen := make(map[string]bool, len(existing))
	out := make([]string, 0, len(existing)+len(added))
	for _, v := range append(append([]string{}, existing...), added...) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add add.go add_test.go
git commit -m "feat: add hydra add

Files a rule into an area file derived from its first glob, command, or title,
merging matchers into frontmatter. Initializes the library when missing."
```

---

### Task 10: `hydra new`

**Files:**
- Create: `new.go`
- Create: `new_test.go`

**Interfaces:**
- Consumes: `RenderRuleFile` (Task 2), `Init` (Task 8), `Sync` (Task 6).
- Produces: `func New(s Scope, name string, out io.Writer) error`

- [ ] **Step 1: Write the failing tests**

Create `new_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScaffoldsRule(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := New(s, "rust-dependencies", &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, ".hydra", "rules", "rust-dependencies.md")
	got := readFile(t, path)
	for _, want := range []string{"paths:", "commands:", "triggers:", "# Rust Dependencies"} {
		if !strings.Contains(got, want) {
			t.Errorf("scaffold missing %q:\n%s", want, got)
		}
	}
}

func TestNewRejectsBadNames(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	for _, name := range []string{"", "Not Kebab", "UPPER", "trailing-"} {
		if err := New(s, name, &out); err == nil {
			t.Errorf("expected an error for %q", name)
		}
	}
}

func TestNewRefusesToClobber(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := New(s, "dupe", &out); err != nil {
		t.Fatal(err)
	}
	if err := New(s, "dupe", &out); err == nil {
		t.Error("expected an error for an existing rule")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run New -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write `new.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// New scaffolds an empty rule for hand-editing. Every matcher key is present but
// commented out, so the shape is obvious without the file claiming matchers it
// does not have.
func New(s Scope, name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("usage: hydra new <name>")
	}
	if !kebab.MatchString(name) {
		return fmt.Errorf("name must be kebab-case (lowercase letters, digits, hyphens): %s", name)
	}

	if !isDir(s.RulesDir) {
		if err := Init(s, out); err != nil {
			return err
		}
	}

	path := filepath.Join(s.RulesDir, name+".md")
	if exists(path) {
		return fmt.Errorf("rule already exists: %s", path)
	}

	content := fmt.Sprintf(`---
always: false
paths: []
commands: []
triggers: []
---

# %s

TODO: state the rule plainly. A few lines, not an essay. Fill in at least one
of paths, commands, or triggers above, or set always: true.
`, headline(name))

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s\n", s.ref(path))

	return Sync(s, out)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS. `TestSyncReportsMatcherlessRule`-style warnings appear in `New`'s output, which is correct — a fresh scaffold has no matchers yet.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add new.go new_test.go
git commit -m "feat: add hydra new

Scaffolds a blank rule with every matcher key present, then syncs."
```

---

### Task 11: `hydra list`

**Files:**
- Create: `list.go`
- Create: `list_test.go`

**Interfaces:**
- Consumes: `LoadRules` (Task 2), `Scope` (Task 3).
- Produces:
  - `type RuleInfo struct { Name, Title, Path string; Always bool; Paths, Commands, Triggers []string }`
  - `func List(s Scope) ([]RuleInfo, error)`
  - `func newListCmd(out io.Writer) *cobra.Command`

- [ ] **Step 1: Write the failing tests**

Create `list_test.go`:

```go
package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestListReturnsRules(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "secrets.md"),
		"---\nalways: true\n---\n\n# Secrets\n\nNever read.\n")
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\ncommands: [\"cargo add\"]\n---\n\n# Rust\n")

	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d want 2", len(got))
	}
	if got[0].Name != "rust" || got[1].Name != "secrets" {
		t.Errorf("order = %s, %s", got[0].Name, got[1].Name)
	}
	if !got[1].Always {
		t.Error("secrets should be always")
	}
	if got[0].Path != ".hydra/rules/rust.md" {
		t.Errorf("Path = %s want a scope-relative ref", got[0].Path)
	}
}

func TestListUninitialized(t *testing.T) {
	tmp := t.TempDir()
	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatalf("uninitialized scope should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d want 0", len(got))
	}
}

func TestListJSONShape(t *testing.T) {
	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, ".hydra", "rules", "rust.md"),
		"---\npaths: [\"**/Cargo.toml\"]\n---\n\n# Rust\n")
	got, err := List(ResolveScope(false, tmp, filepath.Join(tmp, "home")))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name":"rust"`, `"always":false`, `"paths":["**/Cargo.toml"]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON missing %s: %s", want, b)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run List -v`
Expected: FAIL — `undefined: List`, `undefined: RuleInfo`.

- [ ] **Step 3: Write `list.go`**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// RuleInfo is the reporting view of a rule: no body, and Path rendered the same
// way the instruction block renders it.
type RuleInfo struct {
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Path     string   `json:"path"`
	Always   bool     `json:"always"`
	Paths    []string `json:"paths"`
	Commands []string `json:"commands"`
	Triggers []string `json:"triggers"`
}

// List enumerates the library. An uninitialized scope yields an empty slice and
// a nil error rather than failing.
func List(s Scope) ([]RuleInfo, error) {
	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		return nil, err
	}
	infos := make([]RuleInfo, 0, len(rules))
	for _, r := range rules {
		infos = append(infos, RuleInfo{
			Name:     r.Name,
			Title:    r.Title,
			Path:     s.RuleRef(r),
			Always:   r.Always,
			Paths:    r.Paths,
			Commands: r.Commands,
			Triggers: r.Triggers,
		})
	}
	return infos, nil
}

func newListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list rules in the library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rules, err := List(scopeFromCmd(cmd))
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				b, err := json.MarshalIndent(rules, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, r := range rules {
				tier := ""
				if r.Always {
					tier = "always"
				}
				matchers := strings.Join(append(append([]string{}, r.Paths...), r.Commands...), " · ")
				if matchers == "" {
					matchers = strings.Join(r.Triggers, " · ")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", tier, r.Name, matchers)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS. (`scopeFromCmd`, used by `newListCmd`, was defined in `scope.go` in Task 3.)

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add list.go list_test.go
git commit -m "feat: add hydra list for the rules library"
```

---

### Task 12: `hydra doctor`

**Files:**
- Create: `doctor.go`
- Create: `doctor_test.go`

**Interfaces:**
- Consumes: `LoadRules`, `RenderIndex`, `RenderBlock`, `DetectTargets`, `Teardown` markers (Tasks 2–7).
- Produces:
  - `type DoctorCheck struct { Name string; OK bool; Severity string; Detail string }`
  - `type DoctorReport struct { Scope, Home string; OK bool; Checks []DoctorCheck }`
  - `func Doctor(s Scope) DoctorReport`
  - `func renderDoctorText(r DoctorReport, out io.Writer)`

Severity is `"error"` or `"warning"`. `Report.OK` is false only when an **error**-severity check fails, so a stale index does not turn the exit code red.

- [ ] **Step 1: Write the failing tests**

Create `doctor_test.go`:

```go
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
	renderDoctorText(rep, &out)
	got := out.String()
	if !strings.Contains(got, "✓") || !strings.Contains(got, "!") {
		t.Errorf("expected pass and warn glyphs: %s", got)
	}
	if !strings.Contains(got, "run 'hydra sync'") {
		t.Errorf("detail not rendered: %s", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run Doctor -v`
Expected: FAIL — `undefined: Doctor`, `undefined: DoctorReport`.

- [ ] **Step 3: Write `doctor.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sevError   = "error"
	sevWarning = "warning"
)

type DoctorCheck struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

type DoctorReport struct {
	Scope  string        `json:"scope"`
	Home   string        `json:"home"`
	OK     bool          `json:"ok"`
	Checks []DoctorCheck `json:"checks"`
}

// Doctor inspects a scope and returns a structured report. It performs no output
// I/O; rendering is the caller's job. Only error-severity failures clear OK, so a
// stale index reports honestly without turning the exit code red.
func Doctor(s Scope) DoctorReport {
	rep := DoctorReport{Scope: s.Label, Home: s.Home, OK: true}
	add := func(name string, ok bool, severity, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, OK: ok, Severity: severity, Detail: detail})
		if !ok && severity == sevError {
			rep.OK = false
		}
	}

	if !isDir(s.RulesDir) {
		add("rules directory present", false, sevError, "run 'hydra init'")
		return rep
	}
	add("rules directory present", true, sevError, "")

	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		add("every rule parses", false, sevError, err.Error())
		return rep
	}
	add("every rule parses", true, sevError, "")

	seen := map[string]bool{}
	for _, r := range rules {
		add(r.Name+" has a matcher", r.HasMatcher(), sevError,
			"add paths, commands, or triggers, or set always: true")
		add(r.Name+" is uniquely named", !seen[r.Name], sevError, "")
		seen[r.Name] = true
	}

	indexPath := filepath.Join(s.RulesDir, indexFilename)
	current, _ := os.ReadFile(indexPath)
	add("index.md is current", string(current) == RenderIndex(s, rules), sevWarning, "run 'hydra sync'")

	targets := DetectTargets(s)
	add("at least one instruction file detected", len(targets) > 0, sevWarning, "run 'hydra init'")

	block := RenderBlock(s, rules)
	for _, t := range targets {
		add("block current in "+t, blockMatches(t, block), sevWarning, "run 'hydra sync'")
	}

	add("no v0.1 skill-curator artifacts", !hasV01Artifacts(s), sevWarning, "run 'hydra init' to clean up")

	return rep
}

// blockMatches reports whether the target's managed block is byte-identical to
// what a fresh render would produce.
func blockMatches(path, want string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	start := strings.Index(content, blockStart)
	if start < 0 {
		return false
	}
	end := strings.Index(content[start:], blockEnd)
	if end < 0 {
		return false
	}
	got := content[start : start+end+len(blockEnd)]
	return got+"\n" == want
}

func hasV01Artifacts(s Scope) bool {
	for _, p := range []string{
		filepath.Join(s.Home, curatorHookMarker),
		filepath.Join(s.Home, "curator.log"),
		filepath.Join(s.Home, "config"),
	} {
		if exists(p) {
			return true
		}
	}
	for _, dir := range []string{
		filepath.Join(s.Base, ".claude", "skills"),
		filepath.Join(s.Base, ".agents", "skills"),
	} {
		if isDir(dir) {
			return true
		}
	}
	if fileContains(filepath.Join(s.Base, ".claude", "settings.json"), curatorHookMarker) {
		return true
	}
	for _, t := range candidateTargets(s) {
		if fileContains(t, curatorBlockStart) {
			return true
		}
	}
	return false
}

func renderDoctorText(r DoctorReport, out io.Writer) {
	fmt.Fprintf(out, "hydra doctor (%s: %s)\n", r.Scope, r.Home)
	for _, c := range r.Checks {
		glyph := "✓"
		if !c.OK {
			glyph = "✗"
			if c.Severity == sevWarning {
				glyph = "!"
			}
		}
		line := fmt.Sprintf("  %s %s", glyph, c.Name)
		if !c.OK && c.Detail != "" {
			line += " — " + c.Detail
		}
		fmt.Fprintln(out, line)
	}
	if r.OK {
		fmt.Fprintln(out, "doctor: PASS")
	} else {
		fmt.Fprintln(out, "doctor: FAIL")
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./...
git add doctor.go doctor_test.go
git commit -m "feat: add hydra doctor for the rules library

Errors fail the report; stale artifacts and leftover v0.1 wreckage are
warnings with a fix-it hint."
```

---

### Task 13: Wire the CLI and add end-to-end tests

**Files:**
- Modify: `main.go`
- Create: `cli_test.go`

**Interfaces:**
- Consumes: `Init`, `Sync`, `Add`, `New`, `List`, `Doctor` (Tasks 6–12), `scopeFromCmd` (Task 3).
- Produces: `func newAddCmd(out io.Writer) *cobra.Command`, `func newDoctorCmd(out io.Writer) *cobra.Command`, and the full command tree.

- [ ] **Step 1: Write the failing end-to-end tests**

Create `cli_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
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
	t.Setenv("HOME", filepath.Join(tmp, "home"))

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
	if err != nil || !strings.Contains(out, "controllers") {
		t.Errorf("list: out=%q err=%v", out, err)
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
	t.Setenv("HOME", filepath.Join(tmp, "home"))

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
	t.Setenv("HOME", home)

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
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "sync"); err == nil {
		t.Error("sync on an uninitialized project should fail")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run Run -v`
Expected: FAIL — `unknown command "init"`, `unknown command "add"`.

- [ ] **Step 3: Rewrite `main.go`**

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		os.Exit(1)
	}
}

// run builds the root command and executes it against args. It is the testable
// seam: tests drive the CLI through here with in-memory writers.
func run(args []string, out, errw io.Writer) error {
	root := newRootCmd(out, errw)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errw)
	return root.Execute()
}

func newRootCmd(out, errw io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "hydra",
		Short: "hydra — rules library manager for AI coding agents",
		Long:  fmt.Sprintf("hydra %s — manage a library of scoped rules for AI coding agents (Claude Code and others).", version()),
		// Subcommands handle their own error reporting; don't let cobra dump
		// usage text or re-print returned errors (main handles that).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("global", false, "operate on the global scope instead of the current project")

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "scaffold the rules library and wire it into your agent files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Init(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "reindex the library and rewrite every managed block",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Sync(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(newAddCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "scaffold a blank rule for hand-editing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return New(scopeFromCmd(cmd), args[0], out)
		},
	})

	root.AddCommand(newListCmd(out))
	root.AddCommand(newDoctorCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print the hydra version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(out, "hydra %s\n", version())
		},
	})

	root.AddCommand(newSelfUpdateCmd(out))

	return root
}

func newAddCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "record a rule",
		Long: "Record a durable rule so the next agent or teammate inherits it.\n\n" +
			"Give it at least one matcher (--glob, --command, --trigger) or --always,\n" +
			"plus a short --title and a few-line --note. Initializes the library if\n" +
			"it does not exist yet.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			title, _ := cmd.Flags().GetString("title")
			note, _ := cmd.Flags().GetString("note")
			always, _ := cmd.Flags().GetBool("always")
			globs, _ := cmd.Flags().GetStringArray("glob")
			commands, _ := cmd.Flags().GetStringArray("command")
			triggers, _ := cmd.Flags().GetStringArray("trigger")
			return Add(scopeFromCmd(cmd), AddRequest{
				Title:    title,
				Note:     note,
				Always:   always,
				Paths:    globs,
				Commands: commands,
				Triggers: triggers,
			}, out)
		},
	}
	cmd.Flags().String("title", "", "short, specific heading for the rule (required)")
	cmd.Flags().String("note", "", "the rule stated plainly, a few lines (required)")
	cmd.Flags().Bool("always", false, "inline this rule into the block instead of indexing it")
	cmd.Flags().StringArray("glob", nil, "file glob the rule applies to (repeatable)")
	cmd.Flags().StringArray("command", nil, "command prefix the rule applies to (repeatable)")
	cmd.Flags().StringArray("trigger", nil, "situation the rule applies to, in prose (repeatable)")
	return cmd
}

func newDoctorCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "verify the library and its wiring",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep := Doctor(scopeFromCmd(cmd))
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				b, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				renderDoctorText(rep, out)
			}
			if !rep.OK {
				// Return an empty-message error so main exits 1 without
				// re-printing anything (doctor already reported the failure).
				return errors.New("")
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
```

- [ ] **Step 4: Run the full suite**

Run: `gofmt -l . && go vet ./... && go test ./... -v && go build -o /dev/null .`
Expected: all PASS, clean vet and gofmt, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add main.go cli_test.go
git commit -m "feat: wire the rules commands into the CLI

init, sync, add, new, list, doctor. Adds end-to-end lifecycle tests for both
project and global scopes."
```

---

### Task 14: Documentation, version, and cleaning this repo

**Files:**
- Modify: `CLAUDE.md`, `AGENTS.md`, `README.md`, `.gitignore`, `VERSION`
- Delete: `.claude/skills/`, `.agents/skills/` (this repo's own dangling symlinks)

**Interfaces:**
- Consumes: the finished binary.
- Produces: documentation matching the shipped behavior.

- [ ] **Step 1: Bump the version**

```bash
printf '0.2.0\n' > VERSION
```

- [ ] **Step 2: Clean this repo's own v0.1 wreckage**

```bash
go build -o /tmp/hydra-v2 .
/tmp/hydra-v2 init
/tmp/hydra-v2 doctor
```

Expected: `init` reports removing `.claude/skills/` and `.agents/skills/`, strips the curator block from `CLAUDE.md` and `AGENTS.md`, creates `.hydra/rules/`, and writes the rules block. `doctor` then passes.

- [ ] **Step 3: Update `.gitignore`**

Replace the first stanza:

```
# Generated symlinks (regenerated by bin/sync.sh) — keep settings.json tracked.
/.claude/skills/
/.agents/skills/
```

with nothing — hydra no longer generates symlinks, and `.hydra/rules/` is meant to be committed. Leave the rest of the file untouched.

- [ ] **Step 4: Rewrite the project description in `CLAUDE.md`**

Replace the `# hydra — installable skill curator (Go CLI)` section and the `## Layout` and `## Conventions` sections with:

```markdown
# hydra — installable rules curator (Go CLI)

hydra is a single self-contained Go binary that installs a "rules" mechanism into any
project (or globally with `--global`): it scaffolds a `.hydra/rules/` library of
matcher-scoped Markdown rules, generates an index, and splices a managed block into
whatever agent instruction files already exist (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`).
Commands: `init`, `sync`, `add`, `new`, `list`, `doctor`, `self-update`.

## Build & test
- `go test ./...` — full suite (temp-dir based, no network).
- `go vet ./...` and `gofmt -l .` — must be clean (CI enforces both).
- `go build -o hydra .` — local binary (gitignored).
- The CLI is parsed with `spf13/cobra`. Command logic lives in decoupled functions
  (`Init`/`Sync`/`Add`/`New`/`List`/`Doctor`) that take a resolved `Scope` plus an
  `io.Writer`, which keeps them directly unit-testable; cobra is only the parsing
  layer in `main.go`.

## Layout
- `main.go` — cobra wiring: root command, subcommands, the `--global` persistent flag.
- `scope.go` — `Scope` + `ResolveScope`. Project scope renders relative refs; global
  renders absolute ones, because `~/.claude/CLAUDE.md` loads from any directory.
- `rule.go` — the `Rule` model and YAML frontmatter parsing.
- `render.go` — the index table and the managed block.
- `block.go` — sentinel splicing (`<!-- hydra:rules:start/end -->`), replace-in-place.
- `detect.go` — which agent instruction files exist at this scope.
- `init.go` / `sync.go` / `add.go` / `new.go` / `list.go` / `doctor.go` — one command each.
- `teardown.go` — removes v0.1 skill-curator artifacts.
- `VERSION` — embedded default version; release builds inject the tag via
  `-ldflags "-X main.injectedVersion=..."` (see `.goreleaser.yaml`).

## Conventions
- **`CLAUDE.md` and `AGENTS.md` are mirrors** — identical body, only the top title/intro
  line differs. Any edit to one MUST be replicated to the other in the same change.
- Stdlib + cobra + `gopkg.in/yaml.v3` only — keep the dependency surface minimal.
- **No MCP.** hydra is CLI-only by design; `hydra add` is the only write path.
- **Never auto-commit or push.** The git diff is the review gate; ask before `git commit`.
- Releases are cut by tagging `vX.Y.Z` and pushing the tag — goreleaser builds the binaries.
```

- [ ] **Step 5: Mirror the same body into `AGENTS.md`**

`AGENTS.md` must carry an identical body; only the top title/intro line differs. Verify:

```bash
diff <(tail -n +2 CLAUDE.md) <(tail -n +2 AGENTS.md)
```

Expected: no output beyond the intentional intro-line difference. Reconcile any other drift.

- [ ] **Step 6: Rewrite `README.md`**

```markdown
# hydra

A CLI for managing a library of scoped rules for AI coding agents (Claude Code and
others). Rules are committed Markdown; hydra keeps an index of them in your agent
instruction files so an agent reads only the ones that apply to what it's doing.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/webteractive/hydra/main/install.sh | sh
```

Downloads the latest prebuilt binary for your platform, verifies the checksum, and installs
it to `~/.local/bin`.

## Quick start

```bash
cd your-project
hydra init      # scaffold .hydra/rules/ and wire the block into CLAUDE.md / AGENTS.md
hydra add --glob 'app/Http/Controllers/**' \
          --title 'Extend BaseController' \
          --note  'Every controller extends BaseController for tenant scoping.'
hydra doctor
```

Run any command with `--global` to operate on `~/.hydra/rules/` instead, wired into
`~/.claude/CLAUDE.md`. The two libraries are independent — the agent loads both.

## Commands

| Command | Description |
|---|---|
| `hydra init [--global]` | Scaffold the library and wire it into your agent files. |
| `hydra sync [--global]` | Reindex and rewrite every managed block. |
| `hydra add …` | Record a rule. Initializes the library if it doesn't exist. |
| `hydra new <name>` | Scaffold a blank rule for hand-editing. |
| `hydra list [--json]` | List rules with their matchers. |
| `hydra doctor [--json]` | Check that everything is wired up. |
| `hydra self-update` | Update to the latest release. |

## How it works

A rule is one Markdown file with frontmatter declaring when it fires:

```markdown
---
paths:    ["**/Cargo.toml"]
commands: ["cargo add"]
triggers: ["auditing a Rust dependency"]
---

# Rust dependencies

## Pin the exact version
...
```

`hydra sync` renders every rule into an index table and splices it into your agent
instruction files between `<!-- hydra:rules:start -->` sentinels. The agent reads the
table on every prompt and opens only the rule files whose matchers hit. Rules marked
`always: true` are inlined into the block instead of indexed.

## Development

```bash
go test ./...
go vet ./...
go build -o hydra .
```
```

- [ ] **Step 7: Verify the whole thing**

Run: `gofmt -l . && go vet ./... && go test ./... && go build -o /dev/null .`
Expected: clean.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs: rewrite for the rules pivot and bump to 0.2.0

Updates CLAUDE.md/AGENTS.md/README for rules, drops the symlink gitignore
entries, and clears this repo's own dangling v0.1 skill symlinks."
```

---

## Post-implementation

The branch is `rules-pivot`. **Do not push and do not tag** — hand back to the user for
review. A release is cut only after they approve, by tagging `v0.2.0` and pushing the tag.
