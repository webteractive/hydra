package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
		Short: "Show rules in this scope",
		Long: "Show rules in this scope in a readable, labeled format.\n\n" +
			"Use --json for stable, machine-readable output intended for agents and scripts.",
		Args: cobra.NoArgs,
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
			renderRuleListText(out, s, rules)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	return cmd
}

func renderRuleListText(out io.Writer, s Scope, rules []RuleInfo) {
	jsonCommand := "hydra list --json"
	if s.Global {
		jsonCommand = "hydra list --global --json"
	}

	fmt.Fprintf(out, "Rules in %s scope (%d)\n", s.Label, len(rules))
	if len(rules) == 0 {
		fmt.Fprintln(out, "\nNo rules found. Add one with hydra add or hydra new <name>.")
		fmt.Fprintf(out, "\nFor agents and scripts: %s\n", jsonCommand)
		return
	}

	for _, rule := range rules {
		fmt.Fprintf(out, "\n%s (%s)\n", rule.Title, rule.Name)
		if rule.Always {
			fmt.Fprintln(out, "  Applies: Every task (always loaded)")
		}
		if len(rule.Paths) > 0 {
			fmt.Fprintf(out, "  Files: %s\n", strings.Join(rule.Paths, " · "))
		}
		if len(rule.Commands) > 0 {
			fmt.Fprintf(out, "  Commands: %s\n", strings.Join(rule.Commands, " · "))
		}
		if len(rule.Triggers) > 0 {
			fmt.Fprintf(out, "  When: %s\n", strings.Join(rule.Triggers, " · "))
		}
		fmt.Fprintf(out, "  Source: %s\n", rule.Path)
	}

	fmt.Fprintf(out, "\nFor agents and scripts: %s\n", jsonCommand)
}
