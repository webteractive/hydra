package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		os.Exit(1)
	}
}

// run builds the root command and executes it against args. It is the testable
// seam: tests drive the CLI through here with in-memory writers.
func run(args []string, out, errw io.Writer) error {
	root := newRootCmd(out, errw)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errw)
	return root.Execute()
}

// scopeFromCmd resolves the active Scope honoring the persistent --global flag.
func scopeFromCmd(cmd *cobra.Command) Scope {
	global, _ := cmd.Flags().GetBool("global")
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return ResolveScope(global, cwd, home)
}

func newRootCmd(out, errw io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "hydra",
		Short: "hydra — skill library manager for AI coding agents",
		Long:  fmt.Sprintf("hydra %s — manage a library of reusable skills for AI coding agents (Claude Code and others).", version()),
		// Subcommands handle their own error reporting; don't let cobra dump
		// usage text or re-print returned errors (main handles that).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("global", false, "operate on the global scope instead of the current project")

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "scaffold the curator into a project (or globally)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Init(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "rebuild skill symlinks from .hydra/skills/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Sync(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "create a new skill scaffold",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return New(scopeFromCmd(cmd), args[0], out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "log <CREATE|UPDATE|RENAME> <skill> <reason>",
		Short: "append an entry to the curator log",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Log(scopeFromCmd(cmd), args[0], args[1], strings.Join(args[2:], " "), out, time.Now().Format("2006-01-02"))
		},
	})

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "verify install health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep := Doctor(scopeFromCmd(cmd))
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				b, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				renderDoctorText(rep, out)
			}
			if !rep.OK {
				// Return an empty-message error so main exits 1 without
				// re-printing anything (doctor already reported the failure).
				return errors.New("")
			}
			return nil
		},
	}
	doctorCmd.Flags().Bool("json", false, "output as JSON")
	root.AddCommand(doctorCmd)

	root.AddCommand(newListCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print the hydra version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(out, "hydra %s\n", version())
		},
	})

	root.AddCommand(newSelfUpdateCmd(out))

	return root
}
