package main

import "path/filepath"

const relSkills = "../../.hydra/skills" // <runtime>/skills/<name> -> .hydra/skills/<name>, both scopes

type Scope struct {
	Global    bool
	Label     string
	Home      string // <base>/.hydra
	ClaudeDir string
	AgentsDir string
	Settings  string
	ClaudeMDs []string
	HookCmd   string
}

func ResolveScope(global bool, cwd, home string) Scope {
	base := cwd
	mds := []string{filepath.Join(cwd, "CLAUDE.md"), filepath.Join(cwd, "AGENTS.md")}
	hookCmd := "$CLAUDE_PROJECT_DIR/.hydra/curator-reminder.sh"
	label := "project"
	if global {
		base = home
		mds = []string{filepath.Join(home, ".claude", "CLAUDE.md")}
		label = "global"
	}
	s := Scope{
		Global:    global,
		Label:     label,
		Home:      filepath.Join(base, ".hydra"),
		ClaudeDir: filepath.Join(base, ".claude"),
		AgentsDir: filepath.Join(base, ".agents"),
		Settings:  filepath.Join(base, ".claude", "settings.json"),
		ClaudeMDs: mds,
	}
	if global {
		s.HookCmd = filepath.Join(s.Home, "curator-reminder.sh")
	} else {
		s.HookCmd = hookCmd
	}
	return s
}

func (s Scope) RuntimeTargets(runtimes []string) []string {
	var t []string
	for _, r := range runtimes {
		switch r {
		case "claude":
			t = append(t, filepath.Join(s.ClaudeDir, "skills"))
		case "agents":
			t = append(t, filepath.Join(s.AgentsDir, "skills"))
		}
	}
	return t
}
