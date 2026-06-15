package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Sync(s Scope, out io.Writer) error {
	skillsDir := filepath.Join(s.Home, "skills")
	if !isDir(skillsDir) {
		return fmt.Errorf("no skills at %s — run 'hydra init' first", skillsDir)
	}
	targets := s.RuntimeTargets(runtimes(s))
	for _, t := range targets {
		if err := os.MkdirAll(t, 0o755); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return err
	}
	count, linked := 0, 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		count++
		want := relSkills + "/" + name
		for _, t := range targets {
			link := filepath.Join(t, name)
			fi, err := os.Lstat(link)
			switch {
			case err == nil && fi.Mode()&os.ModeSymlink != 0:
				cur, _ := os.Readlink(link)
				switch {
				case cur == want:
					// already correct
				case strings.HasPrefix(cur, relSkills+"/"):
					os.Remove(link)
					if err := os.Symlink(want, link); err != nil {
						return err
					}
					linked++
				default:
					fmt.Fprintf(out, "collision: %s already points elsewhere (%s) — skipping\n", link, cur)
				}
			case err == nil:
				fmt.Fprintf(out, "collision: %s is a real file (not a symlink) — skipping\n", link)
			default:
				if err := os.Symlink(want, link); err != nil {
					return err
				}
				linked++
			}
		}
	}

	for _, t := range targets {
		entries, err := os.ReadDir(t)
		if err != nil {
			continue
		}
		for _, e := range entries {
			link := filepath.Join(t, e.Name())
			fi, err := os.Lstat(link)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if _, err := os.Stat(link); err != nil { // dangling
				if cur, e := os.Readlink(link); e == nil && strings.HasPrefix(cur, relSkills+"/") {
					fmt.Fprintf(out, "prune (dangling): %s\n", link)
					os.Remove(link)
				}
			}
		}
	}

	fmt.Fprintf(out, "synced %d skill(s); %d link(s) (re)created across %d target(s)\n", count, linked, len(targets))
	return nil
}
