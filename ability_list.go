package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

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
		Short: "list global abilities",
		Args:  cobra.NoArgs,
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
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, ability := range abilities {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", ability.Name, ability.Description, ability.Path)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
