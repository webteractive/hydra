package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type AbilityInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

func AbilityList(s AbilityScope) ([]AbilityInfo, error) {
	abilities, err := LoadAbilities(s.AbilitiesDir)
	if err != nil {
		return nil, err
	}
	infos := make([]AbilityInfo, 0, len(abilities))
	for _, ability := range abilities {
		infos = append(infos, AbilityInfo{
			Name:        ability.Name,
			Description: ability.Description,
			Path:        ability.Path,
		})
	}
	return infos, nil
}

func newAbilityListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show available abilities",
		Long: "Show global abilities in a readable format, including how to invoke each one.\n\n" +
			"Use --json for stable, machine-readable output intended for agents and scripts.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			abilities, err := AbilityList(s)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				data, err := json.MarshalIndent(abilities, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderAbilityListText(out, abilities)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	return cmd
}

func renderAbilityListText(out io.Writer, abilities []AbilityInfo) {
	fmt.Fprintf(out, "Global abilities (%d)\n", len(abilities))
	if len(abilities) == 0 {
		fmt.Fprintln(out, "\nNo abilities found. Create one with hydra ability new <name>.")
		fmt.Fprintln(out, "\nFor agents and scripts: hydra ability list --json")
		return
	}

	for _, ability := range abilities {
		fmt.Fprintf(out, "\n%s\n", ability.Name)
		fmt.Fprintf(out, "  %s\n", ability.Description)
		fmt.Fprintf(out, "  Invoke: $ability %s\n", ability.Name)
		fmt.Fprintf(out, "  Source: %s\n", ability.Path)
	}

	fmt.Fprintln(out, "\nFor agents and scripts: hydra ability list --json")
}
