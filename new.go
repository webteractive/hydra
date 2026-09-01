package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedRuleName is the generated catalog; a rule of that name would be
// silently overwritten on the next sync.
var reservedRuleName = strings.TrimSuffix(indexFilename, ".md")

// validRuleName gates every path that decides a rule's filename.
func validRuleName(name string) error {
	if !kebab.MatchString(name) {
		return fmt.Errorf("must be kebab-case (lowercase letters, digits, hyphens): %s", name)
	}
	if name == reservedRuleName {
		return fmt.Errorf("%q is reserved for the generated catalog", name)
	}
	return nil
}

// New scaffolds an empty rule for hand-editing. Every matcher key is present but
// empty, so the shape is obvious without the file claiming matchers it does not
// have.
func New(s Scope, name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("usage: hydra new <name>")
	}
	if err := validRuleName(name); err != nil {
		return err
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
