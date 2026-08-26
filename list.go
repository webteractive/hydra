package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// RuleInfo is the reporting view of a rule: no body, and Path rendered the same
// way the instruction block renders it.
type RuleInfo struct {
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Path     string   `json:"path"`
	Always   bool     `json:"always"`
	Paths    []string `json:"paths"`
	Commands []string `json:"commands"`
	Triggers []string `json:"triggers"`
}

// List enumerates the library. An uninitialized scope yields an empty slice and
// a nil error rather than failing.
func List(s Scope) ([]RuleInfo, error) {
	rules, err := LoadRules(s.RulesDir)
	if err != nil {
		return nil, err
	}
	infos := make([]RuleInfo, 0, len(rules))
	for _, r := range rules {
		infos = append(infos, RuleInfo{
			Name:     r.Name,
			Title:    r.Title,
			Path:     s.RuleRef(r),
			Always:   r.Always,
			Paths:    r.Paths,
			Commands: r.Commands,
			Triggers: r.Triggers,
		})
	}
	return infos, nil
}

func newListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list rules in the library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := scopeFromCmd(cmd)
			if err != nil {
				return err
			}
			rules, err := List(s)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				b, err := json.MarshalIndent(rules, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, r := range rules {
				tier := ""
				if r.Always {
					tier = "always"
				}
				matchers := strings.Join(append(append([]string{}, r.Paths...), r.Commands...), " · ")
				if matchers == "" {
					matchers = strings.Join(r.Triggers, " · ")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", tier, r.Name, matchers)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
