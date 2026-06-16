package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// List enumerates the skills under <s.Home>/skills. An uninitialized project
// (no skills dir) yields an empty slice and nil error rather than failing.
func List(s Scope) ([]SkillInfo, error) {
	skillsDir := filepath.Join(s.Home, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SkillInfo{}, nil
		}
		return nil, err
	}

	var skills []SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mdPath := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		info := SkillInfo{Name: e.Name(), Path: mdPath}
		if data, err := os.ReadFile(mdPath); err == nil {
			name, desc := parseFrontmatter(string(data))
			if name != "" {
				info.Name = name
			}
			info.Description = desc
		}
		skills = append(skills, info)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// parseFrontmatter extracts name/description from leading YAML frontmatter
// without a YAML dependency. Returns empty strings when absent.
func parseFrontmatter(content string) (name, desc string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		switch {
		case strings.HasPrefix(line, "name:"):
			name = trimValue(line[len("name:"):])
		case strings.HasPrefix(line, "description:"):
			desc = trimValue(line[len("description:"):])
		}
	}
	return name, desc
}

func trimValue(v string) string {
	v = strings.TrimSpace(v)
	return strings.Trim(v, `"'`)
}

func newListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list skills in the library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			skills, err := List(scopeFromCmd(cmd))
			if err != nil {
				return err
			}
			if asJSON {
				b, err := json.MarshalIndent(skills, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, sk := range skills {
				fmt.Fprintf(tw, "%s\t%s\n", sk.Name, sk.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}
