package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		os.Exit(1)
	}
}

func usage(out io.Writer) {
	fmt.Fprintf(out, `hydra %s — self-improving skill curator

Usage:
  hydra init    [--global]   scaffold the curator into a project (or globally)
  hydra sync    [--global]   rebuild skill symlinks from .hydra/skills/
  hydra new <name> [--global]
  hydra log <CREATE|UPDATE|RENAME> <skill> <reason> [--global]
  hydra doctor  [--global]   verify install health
  hydra version | help
`, version())
}

func run(args []string, out, errw io.Writer) error {
	global := false
	var pos []string
	for _, a := range args {
		if a == "--global" {
			global = true
		} else {
			pos = append(pos, a)
		}
	}
	cmd := "help"
	if len(pos) > 0 {
		cmd = pos[0]
		pos = pos[1:]
	}

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	s := ResolveScope(global, cwd, home)

	switch cmd {
	case "init":
		return Init(s, out)
	case "sync":
		return Sync(s, out)
	case "new":
		name := ""
		if len(pos) > 0 {
			name = pos[0]
		}
		return New(s, name, out)
	case "log":
		if len(pos) < 3 {
			return errors.New("usage: hydra log <CREATE|UPDATE|RENAME> <skill> <reason>")
		}
		return Log(s, pos[0], pos[1], strings.Join(pos[2:], " "), out, time.Now().Format("2006-01-02"))
	case "doctor":
		if !Doctor(s, out) {
			return errors.New("") // already printed "doctor: FAIL"; exit 1 without re-printing
		}
		return nil
	case "version", "--version", "-v":
		fmt.Fprintf(out, "hydra %s\n", version())
		return nil
	case "help", "--help", "-h", "":
		usage(out)
		return nil
	default:
		usage(errw)
		return fmt.Errorf("unknown command: %s", cmd)
	}
}
