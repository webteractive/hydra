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
