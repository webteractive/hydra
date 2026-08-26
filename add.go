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
