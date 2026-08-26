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
// empty, so the shape is obvious without the file claiming matchers it does not
// have.
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
