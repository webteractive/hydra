package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Init(s Scope, out io.Writer) error {
	scFile := filepath.Join(s.Home, "skills", "skill-curator", "SKILL.md")
	if !exists(scFile) {
		if err := writeAsset(scFile, "skill-curator/SKILL.md", 0o644); err != nil {
			return err
		}
		fmt.Fprintln(out, "seeded skill-curator")
	}
	hook := filepath.Join(s.Home, "curator-reminder.sh")
	if !exists(hook) {
		if err := writeAsset(hook, "curator-reminder.sh", 0o755); err != nil {
			return err
		}
	}
	cfg := filepath.Join(s.Home, "config")
	if !exists(cfg) {
		if err := writeAsset(cfg, "config", 0o644); err != nil {
			return err
		}
	}
	logf := filepath.Join(s.Home, "curator.log")
	if !exists(logf) {
		hdr := "# hydra curator action log — one line per create/update.\n# Format: DATE  ACTION  skill-name  — reason\n"
		if err := os.WriteFile(logf, []byte(hdr), 0o644); err != nil {
			return err
		}
	}
	for _, md := range s.ClaudeMDs {
		if err := appendCuratorBlock(md, out); err != nil {
			return err
		}
	}
	if err := wireHook(s, out); err != nil {
		return err
	}
	if err := Sync(s, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "hydra init complete (%s: %s)\n", s.Label, s.Home)
	return nil
}

func appendCuratorBlock(md string, out io.Writer) error {
	if b, err := os.ReadFile(md); err == nil && bytes.Contains(b, []byte("hydra:curator:start")) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(md), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(md, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, statErr := f.Stat()
	if statErr != nil || fi.Size() > 0 {
		if _, err := f.Write([]byte("\n")); err != nil {
			return err
		}
	}
	if _, err := f.Write(assetBytes("curator-block.md")); err != nil {
		return err
	}
	fmt.Fprintf(out, "appended curator block to %s\n", md)
	return nil
}

func manualHook(cmd string) string {
	return fmt.Sprintf(`{
  "hooks": {
    "UserPromptSubmit": [
      { "hooks": [ { "type": "command", "command": %q } ] }
    ]
  }
}`, cmd)
}

func wireHook(s Scope, out io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(s.Settings), 0o755); err != nil {
		return err
	}
	var data map[string]any
	raw, err := os.ReadFile(s.Settings)
	switch {
	case errors.Is(err, os.ErrNotExist):
		data = map[string]any{}
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, &data); err != nil {
			fmt.Fprintf(out, "settings.json at %s is not valid JSON — not modifying. Add the hook manually:\n%s\n", s.Settings, manualHook(s.HookCmd))
			return nil
		}
	}
	changed, ok := addHook(data, s.HookCmd)
	if !ok {
		fmt.Fprintf(out, "settings.json at %s has an unexpected \"hooks\"/\"UserPromptSubmit\" shape — not modifying. Add the hook manually:\n%s\n", s.Settings, manualHook(s.HookCmd))
		return nil
	}
	if changed {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		mode := os.FileMode(0o644)
		if fi, err := os.Stat(s.Settings); err == nil {
			mode = fi.Mode().Perm()
		}
		if err := os.WriteFile(s.Settings, b, mode); err != nil {
			return err
		}
		fmt.Fprintf(out, "wired UserPromptSubmit hook into %s\n", s.Settings)
	} else {
		fmt.Fprintf(out, "hook already wired in %s\n", s.Settings)
	}
	return nil
}

// addHook appends the UserPromptSubmit hook to data unless already present.
// Returns changed=true if it modified data. Returns ok=false (without mutating)
// when data["hooks"] exists but is not a map, or hooks["UserPromptSubmit"]
// exists but is not an array.
func addHook(data map[string]any, cmd string) (changed, ok bool) {
	hooks, _ := data["hooks"].(map[string]any)
	if hooks == nil {
		if _, present := data["hooks"]; present {
			return false, false
		}
		hooks = map[string]any{}
		data["hooks"] = hooks
	}
	ups, _ := hooks["UserPromptSubmit"].([]any)
	if ups == nil {
		if _, present := hooks["UserPromptSubmit"]; present {
			return false, false
		}
	}
	for _, g := range ups {
		gm, _ := g.(map[string]any)
		hs, _ := gm["hooks"].([]any)
		for _, h := range hs {
			hm, _ := h.(map[string]any)
			if c, _ := hm["command"].(string); c == cmd {
				return false, true
			}
		}
	}
	hooks["UserPromptSubmit"] = append(ups, map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": cmd}},
	})
	return true, true
}
