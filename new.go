package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func New(s Scope, name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("usage: hydra new <name>")
	}
	if !kebab.MatchString(name) {
		return fmt.Errorf("name must be kebab-case (lowercase letters, digits, hyphens): %s", name)
	}
	dir := filepath.Join(s.Home, "skills", name)
	if exists(dir) {
		return fmt.Errorf("skill already exists: %s", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: TODO — say WHEN to use this skill (the line future sessions match against).\n---\n\n# %s\n\nTODO: imperative, focused instructions for this one capability.\n", name, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s/SKILL.md\n", dir)
	return Sync(s, out)
}
