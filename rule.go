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

// indexFilename is the generated table; it is never itself a rule.
const indexFilename = "index.md"

// readmeFilename documents a library rather than ruling anything, unless it
// carries frontmatter.
const readmeFilename = "README.md"

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

// parsedFrontmatter is what ParseRule reads: hydra's keys plus the title key the
// dotfiles rule library uses in place of an H1. Title is a fallback for a body
// with no H1, never an override — the H1 is how hydra names a rule, and the
// dotfiles migration turns title: into one. It is read, never written:
// RenderRuleFile serializes ruleFrontmatter alone.
type parsedFrontmatter struct {
	ruleFrontmatter `yaml:",inline"`
	Title           string `yaml:"title,omitempty"`
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

	var title string
	if m := frontmatterRe.FindStringSubmatch(content); m != nil {
		var fm parsedFrontmatter
		if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
			return Rule{}, fmt.Errorf("%s: invalid frontmatter: %w", path, err)
		}
		r.Always = fm.Always
		r.Paths = fm.Paths
		r.Commands = fm.Commands
		r.Triggers = fm.Triggers
		r.Body = content[len(m[0]):]
		title = strings.TrimSpace(fm.Title)
	}

	r.Body = strings.TrimLeft(r.Body, "\n")
	switch m := h1Re.FindStringSubmatch(r.Body); {
	case m != nil:
		r.Title = m[1]
	case title != "":
		r.Title = title
	default:
		r.Title = headline(name)
	}
	return r, nil
}

// HasMatcher reports whether the rule can ever fire.
func (r Rule) HasMatcher() bool {
	return r.Always || len(r.Paths) > 0 || len(r.Commands) > 0 || len(r.Triggers) > 0
}

// LoadRules reads every *.md in dir except index.md, sorted by name. A missing
// directory is not an error — an uninitialized scope simply has no rules. The
// first file that fails to read or parse fails the whole load, which is what
// sync and doctor want: they must not render a library they cannot fully read.
func LoadRules(dir string) ([]Rule, error) {
	rules, errs := loadRulesEach(dir)
	if len(errs) > 0 {
		return nil, errs[0].Err
	}
	return rules, nil
}

// ruleLoadError is one file LoadRulesTolerant could not use.
type ruleLoadError struct {
	File string
	Err  error
}

// LoadRulesTolerant is LoadRules for readers that only report: every rule that
// parses is returned, and each file that does not is listed beside them, so one
// broken rule cannot hide the rest of its library.
func LoadRulesTolerant(dir string) ([]Rule, []ruleLoadError) {
	return loadRulesEach(dir)
}

func loadRulesEach(dir string) ([]Rule, []ruleLoadError) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Rule{}, nil
		}
		return nil, []ruleLoadError{{File: dir, Err: err}}
	}

	rules := []Rule{}
	var errs []ruleLoadError
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == indexFilename {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, ruleLoadError{File: path, Err: err})
			continue
		}
		// A library's README documents the library; it is a rule only if it
		// says so with frontmatter. The dotfiles library keeps one beside its
		// rules, and without this it reads as a rule that can never fire.
		if e.Name() == readmeFilename && !frontmatterRe.Match(data) {
			continue
		}
		r, err := ParseRule(strings.TrimSuffix(e.Name(), ".md"), path, string(data))
		if err != nil {
			errs = append(errs, ruleLoadError{File: path, Err: err})
			continue
		}
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })
	return rules, errs
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
