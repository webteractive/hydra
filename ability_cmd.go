package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newAbilityCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ability",
		Short: "Manage global ability workflows",
		Long: "Manage global ability bundles in ~/.hydra/abilities.\n\n" +
			"Each ability's name, triggers, and description are inlined into your agent\n" +
			"instruction files; only the authored ABILITY.md body is read on selection.\n\n" +
			"Ability commands are always global; --global is unnecessary.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize the global abilities catalog and routers",
		Long: "Create ~/.hydra/abilities, wire the discovery block into each detected agent\n" +
			"instruction file, and install the $ability router skill for each harness.\n\n" +
			"Never creates or rewrites an authored ability bundle.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			return AbilityInit(s, out)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Validate abilities and refresh generated wiring",
		Long: "Validate every ability bundle, then regenerate index.md, the discovery block\n" +
			"in each agent instruction file, and the routers.\n\n" +
			"Run this after editing an ABILITY.md by hand — nothing you author takes\n" +
			"effect until the generated wiring is refreshed.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			return AbilitySync(s, out)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a global ability bundle",
		Long: "Create ~/.hydra/abilities/<name>/ABILITY.md with frontmatter ready to fill in,\n" +
			"then sync.\n\n" +
			"Edit its description and triggers before relying on it, and check a phrase\n" +
			"reaches it with `hydra ability match`.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			return AbilityNew(s, args[0], out)
		},
	})

	cmd.AddCommand(newAbilityListCmd(out))
	cmd.AddCommand(newAbilityMatchCmd(out))
	cmd.AddCommand(newAbilityDoctorCmd(out))
	return cmd
}

func newAbilityDoctorCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check the global catalog, blocks, and routers",
		Long: "Verify the global ability catalog and its wiring: that every bundle is valid,\n" +
			"index.md is current, the discovery block and routers are present and current,\n" +
			"and no trigger is dead.\n\n" +
			"Exits non-zero when a check fails. Use --json for machine-readable output.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			rep := AbilityDoctor(s)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				data, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
			} else {
				renderDoctorText(out, rep, abilityDoctorSummary(rep), "hydra ability doctor --json")
			}
			if !rep.OK {
				return errors.New("")
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	return cmd
}
