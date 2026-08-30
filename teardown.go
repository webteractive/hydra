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

// Teardown removes artifacts hydra no longer owns — v0.1 skill-curator files,
// and the Gemini wiring dropped after v0.2 — and reports whether it found any.
// Authored content is never destroyed: .hydra/skills/ is left in place and
// named in the output so it can be salvaged by hand.
func Teardown(s Scope, out io.Writer) (bool, error) {
	found := false

	removed, err := removeGeminiArtifacts(s, out)
	if err != nil {
		return found, err
	}
	found = found || removed

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
		target := resolveLink(link)
		if target == "" {
			continue
		}
		if slices.Contains(skillsDirs, filepath.Dir(target)) {
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

// resolveLink returns a symlink's target as a cleaned absolute path, without
// requiring the target to exist. Readlink, not EvalSymlinks: a dangling link
// still names its target, and dangling links are exactly what a stale farm is
// made of. Returns "" when the link cannot be read.
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

// Gemini support was removed after v0.2. Cleanup is split the way the wiring
// was: the rules block follows the rules scope, while the ability block and its
// router are always global and so follow the ability scope. Keeping them
// together under the rules scope would strand the global ability artifacts
// whenever init ran inside a project.

// geminiInstructionPath is where hydra used to write its managed rules block.
func geminiInstructionPath(s Scope) string {
	if s.Global {
		return filepath.Join(s.Base, ".gemini", "GEMINI.md")
	}
	return filepath.Join(s.Base, "GEMINI.md")
}

// removeGeminiArtifacts strips hydra's managed rules block from GEMINI.md. The
// user's own prose is untouched — only sentinel-delimited blocks are removed.
func removeGeminiArtifacts(s Scope, out io.Writer) (bool, error) {
	path := geminiInstructionPath(s)
	stripped, err := StripBlock(path, blockStart, blockEnd)
	if err != nil {
		return false, err
	}
	if stripped {
		fmt.Fprintf(out, "  stripped gemini rules block from %s\n", path)
	}
	return stripped, nil
}

// hasGeminiArtifacts reports whether a rules block is left for Teardown.
func hasGeminiArtifacts(s Scope) bool {
	data, err := os.ReadFile(geminiInstructionPath(s))
	return err == nil && strings.Contains(string(data), blockStart)
}

// removeGeminiAbilityArtifacts strips the abilities block from the global
// GEMINI.md and deletes the ability router hydra installed. The router only
// goes if it still carries hydra's ownership marker, so a hand-authored skill
// at that path is left alone — that directory holds other tools' skills too.
func removeGeminiAbilityArtifacts(s AbilityScope, out io.Writer) (bool, error) {
	found := false

	instructions := filepath.Join(s.UserHome, ".gemini", "GEMINI.md")
	stripped, err := StripBlock(instructions, abilityBlockStart, abilityBlockEnd)
	if err != nil {
		return found, err
	}
	if stripped {
		fmt.Fprintf(out, "  stripped gemini abilities block from %s\n", instructions)
		found = true
	}

	router := filepath.Join(s.UserHome, ".gemini", "skills", "ability", "SKILL.md")
	data, err := os.ReadFile(router)
	switch {
	case os.IsNotExist(err):
		return found, nil
	case err != nil:
		return found, err
	case !strings.Contains(string(data), routerOwnedMarker):
		fmt.Fprintf(out, "  kept     %s (not Hydra-owned)\n", router)
		return found, nil
	}
	if err := os.Remove(router); err != nil {
		return found, err
	}
	fmt.Fprintf(out, "  removed  %s\n", router)
	found = true

	// Prune the directories hydra created for the router, never a shared one
	// that still holds someone else's skills.
	for dir := filepath.Dir(router); dir != s.UserHome; dir = filepath.Dir(dir) {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		fmt.Fprintf(out, "  removed  %s\n", dir)
	}
	return found, nil
}

// hasGeminiAbilityArtifacts reports whether global ability wiring is left.
func hasGeminiAbilityArtifacts(s AbilityScope) bool {
	if data, err := os.ReadFile(filepath.Join(s.UserHome, ".gemini", "GEMINI.md")); err == nil {
		if strings.Contains(string(data), abilityBlockStart) {
			return true
		}
	}
	data, err := os.ReadFile(filepath.Join(s.UserHome, ".gemini", "skills", "ability", "SKILL.md"))
	return err == nil && strings.Contains(string(data), routerOwnedMarker)
}
