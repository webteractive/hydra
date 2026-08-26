package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	curatorBlockStart = "<!-- hydra:curator:start -->"
	curatorBlockEnd   = "<!-- hydra:curator:end -->"
	curatorHookMarker = "curator-reminder.sh"
	curatorSkillName  = "skill-curator"
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

	for _, dir := range skillFarms(s) {
		removed, err := removeSymlinkFarm(dir, ownedSkillDirs(s), out)
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

// skillFarms lists the directories v0.1 symlinked skills into.
func skillFarms(s Scope) []string {
	return []string{
		filepath.Join(s.Base, ".claude", "skills"),
		filepath.Join(s.Base, ".agents", "skills"),
	}
}

// ownedSkillDirs lists the library locations a symlink may point into for hydra
// to claim it: the v0.1 library, and the flat <base>/skills its predecessor
// used. Anything pointing elsewhere belongs to another tool.
func ownedSkillDirs(s Scope) []string {
	return []string{
		filepath.Clean(filepath.Join(s.Home, "skills")),
		filepath.Clean(filepath.Join(s.Base, "skills")),
	}
}

// hydraOwnedLinks returns the entries in dir that are symlinks pointing directly
// into one of skillsDirs. These directories are shared: globally,
// ~/.claude/skills also holds links from skillset, dotfiles, and plugins, plus
// real directories. Touching anything else would destroy another tool's work,
// so ownership is proven by the link target, never assumed from the directory.
func hydraOwnedLinks(dir string, skillsDirs []string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var owned []string
	for _, e := range entries {
		link := filepath.Join(dir, e.Name())
		fi, err := os.Lstat(link)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		// Readlink, not EvalSymlinks: a dangling link still names its target,
		// and dangling links are exactly what a stale farm is made of.
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		parent := filepath.Dir(filepath.Clean(target))
		if slices.Contains(skillsDirs, parent) {
			owned = append(owned, link)
		}
	}
	return owned
}

// staleLinks narrows hydraOwnedLinks to the ones teardown may actually delete:
// the curator's own link, and links whose target no longer exists. A link to a
// skill that still exists is live content — removing it would unexpose a working
// skill, which is the same kind of destruction as deleting the skill itself.
func staleLinks(dir string, skillsDirs []string) (stale, live []string) {
	for _, link := range hydraOwnedLinks(dir, skillsDirs) {
		if filepath.Base(link) == curatorSkillName || !exists(resolveLink(link)) {
			stale = append(stale, link)
			continue
		}
		live = append(live, link)
	}
	return stale, live
}

// resolveLink returns a symlink's target as an absolute path, without requiring
// it to exist.
func resolveLink(link string) string {
	target, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	return filepath.Clean(target)
}

// removeSymlinkFarm deletes the curator link and any dangling links hydra left
// in dir. Links to skills that still exist, links belonging to other tools, and
// real files are left alone; the directory itself only goes if it ends up empty.
func removeSymlinkFarm(dir string, skillsDirs []string, out io.Writer) (bool, error) {
	stale, live := staleLinks(dir, skillsDirs)
	removed := false
	for _, link := range stale {
		if err := os.Remove(link); err != nil {
			return removed, err
		}
		fmt.Fprintf(out, "  removed  %s\n", link)
		removed = true
	}
	for _, link := range live {
		fmt.Fprintf(out, "  kept     %s (skill still exists)\n", link)
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
