package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const abilityFilename = "ABILITY.md"

// Ability is one optional workflow bundle. Path always points at the authored
// ABILITY.md; supporting resources live beside it and are not parsed by Hydra.
type Ability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Body        string `json:"-"`
}

type abilityFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ParseAbility validates the metadata Hydra needs for discovery. Unknown
// frontmatter fields are intentionally ignored so authored abilities can carry
// harness- or workflow-specific metadata without waiting on Hydra.
func ParseAbility(bundleName, path, content string) (Ability, error) {
	a := Ability{Path: path}
	m := frontmatterRe.FindStringSubmatch(content)
	if m == nil {
		return Ability{}, fmt.Errorf("%s: missing YAML frontmatter", path)
	}

	var fm abilityFrontmatter
	if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
		return Ability{}, fmt.Errorf("%s: invalid frontmatter: %w", path, err)
	}
	a.Name = strings.TrimSpace(fm.Name)
	a.Description = strings.TrimSpace(fm.Description)
	a.Body = strings.TrimLeft(content[len(m[0]):], "\n")

	var problems []error
	if a.Name == "" {
		problems = append(problems, fmt.Errorf("name is required"))
	} else {
		if !kebab.MatchString(a.Name) {
			problems = append(problems, fmt.Errorf("name must be kebab-case: %s", a.Name))
		}
		if a.Name != bundleName {
			problems = append(problems, fmt.Errorf("name %q must match directory %q", a.Name, bundleName))
		}
	}
	if a.Description == "" {
		problems = append(problems, fmt.Errorf("description is required"))
	}
	if strings.ContainsAny(a.Description, "\r\n") {
		problems = append(problems, fmt.Errorf("description must be one line"))
	}
	if len(problems) > 0 {
		return Ability{}, fmt.Errorf("%s: %w", path, errors.Join(problems...))
	}
	return a, nil
}

// LoadAbilities reads immediate, non-hidden bundle directories and reports all
// invalid bundles together. A missing library is treated as empty so list can
// remain useful before initialization; sync and doctor enforce its presence.
func LoadAbilities(dir string) ([]Ability, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Ability{}, nil
		}
		return nil, err
	}

	var abilities []Ability
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name(), abilityFilename)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				problems = append(problems, fmt.Errorf("%s: missing %s", filepath.Join(dir, entry.Name()), abilityFilename))
			} else {
				problems = append(problems, err)
			}
			continue
		}
		ability, err := ParseAbility(entry.Name(), path, string(data))
		if err != nil {
			problems = append(problems, err)
			continue
		}
		abilities = append(abilities, ability)
	}

	sort.Slice(abilities, func(i, j int) bool { return abilities[i].Name < abilities[j].Name })
	if len(problems) > 0 {
		return abilities, errors.Join(problems...)
	}
	return abilities, nil
}

func RenderAbilityFile(name string) string {
	return fmt.Sprintf(`---
name: %s
description: TODO describe when this ability is useful.
---

# %s

TODO: describe the workflow. Link supporting files relative to this directory
and tell the agent exactly when to load or run them.
`, name, headline(name))
}
