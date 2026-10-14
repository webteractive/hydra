package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFilename = "SKILL.md"

// skillImport is one validated source, ready to be copied into the library.
type skillImport struct {
	Source   string // the path as given; a link here is removed, never its target
	Resolved string // where the bundle's files actually live
	Name     string
	Content  string // the ABILITY.md to write
	Triggers int
}

// AbilityImport turns Agent Skills into abilities. Every source is validated
// before anything is written, the library is synced before any source is
// removed, and a failed sync takes the new bundles back out, so a refusal at
// any step leaves both the library and the skills as they were.
func AbilityImport(s AbilityScope, paths []string, keep bool, out io.Writer) error {
	if len(paths) == 0 {
		return fmt.Errorf("usage: hydra ability import <skill-dir>...")
	}
	if err := guardAbilities(s); err != nil {
		return err
	}

	imports, err := prepareSkillImports(s, paths)
	if err != nil {
		return err
	}
	if !isDir(s.AbilitiesDir) {
		if err := AbilityInit(s, out); err != nil {
			return err
		}
	}

	var placed []string
	rollback := func() {
		for _, dir := range placed {
			os.RemoveAll(dir)
		}
	}
	for _, imp := range imports {
		dest, err := placeSkillImport(s, imp)
		if err != nil {
			rollback()
			return err
		}
		placed = append(placed, dest)
		fmt.Fprintf(out, "imported %s → %s\n", imp.Source, filepath.Join(dest, abilityFilename))
	}
	if err := AbilitySync(s, out); err != nil {
		rollback()
		return fmt.Errorf("%w\nnothing was imported; every skill was left in place", err)
	}

	var problems []error
	for _, imp := range imports {
		if imp.Triggers == 0 {
			fmt.Fprintf(out, "%s has no triggers — add some to its %s, then run 'hydra ability sync'\n", imp.Name, abilityFilename)
		}
		if keep {
			fmt.Fprintf(out, "kept %s — it is still a skill as well as an ability\n", imp.Source)
			continue
		}
		if err := removeSkillSource(imp, out); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

// prepareSkillImports validates every path and renders its ABILITY.md, all
// before the caller writes anything.
func prepareSkillImports(s AbilityScope, paths []string) ([]skillImport, error) {
	library := resolvedPath(s.AbilitiesDir)
	existing := map[string]bool{}
	if entries, err := os.ReadDir(s.AbilitiesDir); err == nil {
		for _, entry := range entries {
			existing[entry.Name()] = true
		}
	}

	var (
		imports  []skillImport
		problems []error
	)
	seen := map[string]string{}
	for _, path := range paths {
		imp, err := prepareSkillImport(path, library)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		switch {
		case existing[imp.Name]:
			problems = append(problems, fmt.Errorf("%s: ability already exists: %s", path, filepath.Join(s.AbilitiesDir, imp.Name)))
		case seen[imp.Name] != "":
			problems = append(problems, fmt.Errorf("%s: %q is named more than once (also %s)", path, imp.Name, seen[imp.Name]))
		default:
			seen[imp.Name] = path
			imports = append(imports, imp)
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("nothing was imported:\n%w", errors.Join(problems...))
	}
	return imports, nil
}

func prepareSkillImport(path, library string) (skillImport, error) {
	source := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return skillImport{}, fmt.Errorf("%s: %w", path, err)
	}
	if resolved, err = filepath.Abs(resolved); err != nil {
		return skillImport{}, fmt.Errorf("%s: %w", path, err)
	}
	if within(resolved, library) {
		return skillImport{}, fmt.Errorf("%s: already inside the abilities library", path)
	}
	if within(library, resolved) {
		return skillImport{}, fmt.Errorf("%s: contains the abilities library", path)
	}

	data, err := os.ReadFile(filepath.Join(resolved, skillFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return skillImport{}, fmt.Errorf("%s: no %s — not a skill directory", path, skillFilename)
		}
		return skillImport{}, fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Contains(data, []byte(routerOwnedMarker)) {
		return skillImport{}, fmt.Errorf("%s: this is hydra's generated $ability router, not a skill", path)
	}
	if err := checkRegularTree(resolved); err != nil {
		return skillImport{}, fmt.Errorf("%s: %w", path, err)
	}

	name, content, err := skillToAbility(string(data), filepath.Base(source))
	if err != nil {
		return skillImport{}, fmt.Errorf("%s: %w", path, err)
	}
	ability, err := ParseAbility(name, filepath.Join(path, skillFilename), content)
	if err != nil {
		return skillImport{}, err
	}
	return skillImport{Source: source, Resolved: resolved, Name: name, Content: content, Triggers: len(ability.Triggers)}, nil
}

// skillToAbility returns the ability's name and its ABILITY.md. A skill that
// already fits is copied byte for byte; otherwise only its frontmatter is
// re-encoded, to name it after its directory (as Claude Code does when name is
// absent) and to fold a multi-line description onto the one line abilities need.
func skillToAbility(content, dirName string) (string, string, error) {
	m := frontmatterRe.FindStringSubmatchIndex(content)
	if m == nil {
		return "", "", fmt.Errorf("%s has no YAML frontmatter", skillFilename)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content[m[2]:m[3]]), &doc); err != nil {
		return "", "", fmt.Errorf("%s has invalid frontmatter: %w", skillFilename, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", "", fmt.Errorf("%s frontmatter is not a mapping", skillFilename)
	}
	fields := doc.Content[0]

	changed := false
	name := strings.TrimSpace(mappingValue(fields, "name"))
	if name == "" {
		name = dirName
		setMappingValue(fields, "name", name)
		changed = true
	}
	description := mappingValue(fields, "description")
	if flat := strings.Join(strings.Fields(description), " "); flat != description {
		setMappingValue(fields, "description", flat)
		changed = true
	}
	if !changed {
		return name, content, nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return "", "", err
	}
	if err := enc.Close(); err != nil {
		return "", "", err
	}
	return name, "---\n" + buf.String() + "---\n" + content[m[1]:], nil
}

func mappingValue(mapping *yaml.Node, key string) string {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1].Value
		}
	}
	return ""
}

// setMappingValue replaces key's value with a plain scalar, letting the
// encoder quote it if it has to, or adds the key at the top when absent.
func setMappingValue(mapping *yaml.Node, key, value string) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = scalar
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	mapping.Content = append([]*yaml.Node{keyNode, scalar}, mapping.Content...)
}

// checkRegularTree refuses a bundle holding anything but directories and
// regular files. A link inside a skill points somewhere the import cannot
// promise to carry along.
func checkRegularTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file — replace it with a copy, or remove it", path)
		}
		return nil
	})
}

// placeSkillImport stages the bundle in a hidden directory, which
// LoadAbilities skips, and renames it into place only once it is complete.
func placeSkillImport(s AbilityScope, imp skillImport) (string, error) {
	staging, err := os.MkdirTemp(s.AbilitiesDir, ".import-"+imp.Name+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	if err := copyTree(imp.Resolved, staging); err != nil {
		return "", fmt.Errorf("cannot copy %s: %w", imp.Source, err)
	}
	if err := os.Remove(filepath.Join(staging, skillFilename)); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(staging, abilityFilename), []byte(imp.Content), 0o644); err != nil {
		return "", err
	}
	dest := filepath.Join(s.AbilitiesDir, imp.Name)
	if err := os.Rename(staging, dest); err != nil {
		return "", fmt.Errorf("cannot place %s: %w", dest, err)
	}
	return dest, nil
}

// removeSkillSource removes the skill that was imported. A link is removed on
// its own: its target may be owned by another tool and reached by other links.
func removeSkillSource(imp skillImport, out io.Writer) error {
	info, err := os.Lstat(imp.Source)
	if err != nil {
		return fmt.Errorf("imported %s but could not remove its source: %w", imp.Name, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		if err := os.Remove(imp.Source); err != nil {
			return fmt.Errorf("imported %s but could not remove the link %s: %w", imp.Name, imp.Source, err)
		}
		fmt.Fprintf(out, "removed link %s — its target %s was left in place, so other links to it still load it as a skill\n", imp.Source, imp.Resolved)
		return nil
	}
	if err := os.RemoveAll(imp.Source); err != nil {
		return fmt.Errorf("imported %s but could not remove %s: %w", imp.Name, imp.Source, err)
	}
	fmt.Fprintf(out, "removed %s\n", imp.Source)
	return nil
}

// resolvedPath is the absolute path with links resolved through its deepest
// existing ancestor, so a library that is not created yet, or is reached
// through a link (macOS's /var is one), still compares equal to itself.
func resolvedPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	var missing []string
	for dir := path; ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...)
		}
		if dir == filepath.Dir(dir) {
			return path
		}
		missing = append([]string{filepath.Base(dir)}, missing...)
	}
}
