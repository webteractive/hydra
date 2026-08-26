package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

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

func newRootCmd(out, errw io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "hydra",
		Short: "hydra — rules library manager for AI coding agents",
		Long:  fmt.Sprintf("hydra %s — manage a library of scoped rules for AI coding agents (Claude Code and others).", version()),
		// Subcommands handle their own error reporting; don't let cobra dump
		// usage text or re-print returned errors (main handles that).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("global", false, "operate on the global scope instead of the current project")

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "scaffold the rules library and wire it into your agent files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Init(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "reindex the library and rewrite every managed block",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Sync(scopeFromCmd(cmd), out)
		},
	})

	root.AddCommand(newAddCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "scaffold a blank rule for hand-editing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return New(scopeFromCmd(cmd), args[0], out)
		},
	})

	root.AddCommand(newListCmd(out))
	root.AddCommand(newDoctorCmd(out))

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

func newAddCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "record a rule",
		Long: "Record a durable rule so the next agent or teammate inherits it.\n\n" +
			"Give it at least one matcher (--glob, --command, --trigger) or --always,\n" +
			"plus a short --title and a few-line --note. Initializes the library if\n" +
			"it does not exist yet.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			title, _ := cmd.Flags().GetString("title")
			note, _ := cmd.Flags().GetString("note")
			always, _ := cmd.Flags().GetBool("always")
			globs, _ := cmd.Flags().GetStringArray("glob")
			commands, _ := cmd.Flags().GetStringArray("command")
			triggers, _ := cmd.Flags().GetStringArray("trigger")
			return Add(scopeFromCmd(cmd), AddRequest{
				Title:    title,
				Note:     note,
				Always:   always,
				Paths:    globs,
				Commands: commands,
				Triggers: triggers,
			}, out)
		},
	}
	cmd.Flags().String("title", "", "short, specific heading for the rule (required)")
	cmd.Flags().String("note", "", "the rule stated plainly, a few lines (required)")
	cmd.Flags().Bool("always", false, "inline this rule into the block instead of indexing it")
	cmd.Flags().StringArray("glob", nil, "file glob the rule applies to (repeatable)")
	cmd.Flags().StringArray("command", nil, "command prefix the rule applies to (repeatable)")
	cmd.Flags().StringArray("trigger", nil, "situation the rule applies to, in prose (repeatable)")
	return cmd
}

func newDoctorCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "verify the library and its wiring",
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
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
