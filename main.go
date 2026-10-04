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
		code := 1
		var ec *exitCodeError
		if errors.As(err, &ec) {
			code = ec.code
		}
		os.Exit(code)
	}
}

// exitCodeError carries an exit status other than 1 up to main. msg may be
// empty when the command has already reported the problem itself.
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string { return e.msg }

// run builds the root command and executes it against args. It is the testable
// seam: tests drive the CLI through here with in-memory writers.
func run(args []string, out, errw io.Writer) error {
	root := newRootCmd(out, errw)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errw)
	return root.Execute()
}

// versionFormat is the single shape both version surfaces print, so the flag
// and the subcommand cannot drift apart.
const versionFormat = "hydra %s\n"

func newRootCmd(out, errw io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "hydra",
		Short: "hydra — rules and abilities for AI coding agents",
		Long: fmt.Sprintf("hydra %s — manage scoped rules and global ability workflows for AI coding agents.\n\n"+
			"Rules are mandatory conventions, selected by path, command, or situation.\n"+
			"Abilities are optional workflow bundles, invoked by name or trigger phrase.\n"+
			"Both keep their bulk out of standing context without hiding their existence:\n"+
			"the agent sees a compact table and opens the full file only for what fires.", version()),
		// Subcommands handle their own error reporting; don't let cobra dump
		// usage text or re-print returned errors (main handles that).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Setting Version makes cobra generate --version (and -v, which is unused
	// here). The template keeps it identical to the `version` subcommand so the
	// two surfaces can never disagree about what is installed.
	root.Version = version()
	root.SetVersionTemplate(fmt.Sprintf(versionFormat, "{{.Version}}"))

	root.PersistentFlags().Bool("global", false, "operate on the global scope instead of the current project")
	root.PersistentFlags().String(hydraHomeFlagName, "", "absolute path to the global hydra library for this run (default $"+hydraHomeEnv+", else ~/.hydra)")

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Scaffold the rules scope and wire every agent file",
		Long: "Scaffold this scope's rules library and splice the managed block into every\n" +
			"agent instruction file it detects, then ensure the global ability system is\n" +
			"wired too.\n\n" +
			"Idempotent: authored rules and abilities are never rewritten. Also cleans up\n" +
			"artifacts hydra no longer owns.\n\n" +
			"Use --global for the global library, ~/.hydra/rules by default and\n" +
			"$HYDRA_HOME/rules when that is set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			return Init(s, out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Reindex rules and rewrite every managed block",
		Long: "Reparse the rules library and regenerate both artifacts: index.md, and the\n" +
			"managed block in every detected agent instruction file.\n\n" +
			"Does not create the library — that is init's job, so a mistyped directory is\n" +
			"reported rather than silently scaffolded. Warns about rules that can never fire.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			return Sync(s, out)
		},
	})

	root.AddCommand(newAddCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a blank rule for hand-editing",
		Long: "Create <name>.md in this scope's rules library with empty frontmatter, ready\n" +
			"to fill in by hand.\n\n" +
			"Use `hydra add` instead when you can state the rule in one command.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			return New(s, args[0], out)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "relocate <path>",
		Short: "Move the global library somewhere else",
		Long: "Move the global rules and abilities libraries to <path> and rewrite every\n" +
			"managed block so it names the new location.\n\n" +
			"Always operates on the global library — project rules stay in their project,\n" +
			"which is the point of the scope. The source is the library hydra would read\n" +
			"now: --hydra-home, else $" + hydraHomeEnv + ", else ~/.hydra.\n\n" +
			"Refuses a destination that already holds anything. Since $" + hydraHomeEnv + " lives\n" +
			"in your shell profile, the last thing it prints is the line to add there.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := globalScopeFromCmd(cmd)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot resolve the current directory: %w", err)
			}
			dest, err := resolveRelocateDest(args[0], cwd, s.UserHome)
			if err != nil {
				return err
			}
			return Relocate(s, dest, out)
		},
	})

	root.AddCommand(newListCmd(out))
	root.AddCommand(newDoctorCmd(out))
	root.AddCommand(newMatchCmd(out))
	root.AddCommand(newAbilityCmd(out))

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the hydra version",
		Long:  "Print the version of hydra that is installed.\n\nEquivalent to `hydra --version`.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(out, versionFormat, version())
		},
	})

	root.AddCommand(newSelfUpdateCmd(out))

	addForceFlag(root, "init", "sync", "new", "add", "relocate")
	return root
}

// addForceFlag gives the named subcommands --force: every command that
// rewrites a global managed block, and so is guarded against a block that
// names a different library (guard.go).
func addForceFlag(parent *cobra.Command, names ...string) {
	for _, c := range parent.Commands() {
		for _, name := range names {
			if c.Name() == name {
				c.Flags().Bool("force", false, "rewrite managed blocks even when they name a different global library")
			}
		}
	}
}

func newAddCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Record a rule",
		Long: "Record a durable rule so the next agent or teammate inherits it.\n\n" +
			"Give it at least one matcher (--glob, --command, --trigger) or --always,\n" +
			"plus a short --title and a few-line --note. Initializes the library if\n" +
			"it does not exist yet.\n\n" +
			"Rules covering the same area share a file. The area is inferred from the\n" +
			"first --glob, else the first --command, else the title — which means\n" +
			"argument order decides the filename. Pass --area to state it instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			title, _ := cmd.Flags().GetString("title")
			note, _ := cmd.Flags().GetString("note")
			always, _ := cmd.Flags().GetBool("always")
			globs, _ := cmd.Flags().GetStringArray("glob")
			commands, _ := cmd.Flags().GetStringArray("command")
			triggers, _ := cmd.Flags().GetStringArray("trigger")
			area, _ := cmd.Flags().GetString("area")
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			return Add(s, AddRequest{
				Title:    title,
				Note:     note,
				Area:     area,
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
	cmd.Flags().String("area", "", "file to group this rule under, kebab-case (default: inferred from the first --glob, --command, or the title)")
	return cmd
}

func newDoctorCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that everything is wired up",
		Long: "Verify this scope's rules library and its wiring: that rules parse, index.md\n" +
			"is current, the managed block is present and up to date in every detected\n" +
			"agent instruction file, and no artifacts hydra no longer owns are left behind.\n\n" +
			"Exits non-zero when a check fails, so it can gate a script or CI job.\n" +
			"Use --json for machine-readable output.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			rep := Doctor(s)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				b, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else {
				renderDoctorText(out, rep,
					fmt.Sprintf("hydra doctor (%s: %s)", rep.Scope, doctorHome(rep)),
					scopedJSONCommand(s, "doctor"))
			}
			if !rep.OK {
				// Return an empty-message error so main exits 1 without
				// re-printing anything (doctor already reported the failure).
				return errors.New("")
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	return cmd
}
