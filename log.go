package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Log(s Scope, action, skill, reason string, out io.Writer, today string) error {
	switch action {
	case "CREATE", "UPDATE", "RENAME":
	default:
		return fmt.Errorf("ACTION must be CREATE, UPDATE, or RENAME")
	}
	if skill == "" || reason == "" {
		return fmt.Errorf("usage: hydra log <CREATE|UPDATE|RENAME> <skill> <reason>")
	}
	if !isDir(s.Home) {
		return fmt.Errorf("no .hydra at %s — run 'hydra init' first", s.Home)
	}
	f, err := os.OpenFile(filepath.Join(s.Home, "curator.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s  %s  %s  — %s\n", today, action, skill, reason); err != nil {
		return err
	}
	fmt.Fprintf(out, "logged: %s %s\n", action, skill)
	return nil
}
