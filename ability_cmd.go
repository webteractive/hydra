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
		Short: "manage global lazy-loaded ability workflows",
		Long:  "Manage global ability bundles in ~/.hydra/abilities. Ability commands are always global; --global is unnecessary.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "initialize global abilities and harness wiring",
		Args:  cobra.NoArgs,
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
		Short: "validate abilities and refresh generated wiring",
		Args:  cobra.NoArgs,
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
		Short: "scaffold a global ability bundle",
		Args:  cobra.ExactArgs(1),
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
		Short: "verify global abilities and harness wiring",
		Args:  cobra.NoArgs,
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
				fmt.Fprintln(out, abilityDoctorSummary(rep))
				for _, check := range rep.Checks {
					glyph := "✓"
					if !check.OK {
						glyph = "✗"
						if check.Severity == sevWarning {
							glyph = "!"
						}
					}
					line := fmt.Sprintf("  %s %s", glyph, check.Name)
					if !check.OK && check.Detail != "" {
						line += " — " + check.Detail
					}
					fmt.Fprintln(out, line)
				}
				if rep.OK {
					fmt.Fprintln(out, "doctor: PASS")
				} else {
					fmt.Fprintln(out, "doctor: FAIL")
				}
			}
			if !rep.OK {
				return errors.New("")
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
