package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Library kinds, in the order `hydra match` reports them: a project rule
// overrides a global one, so it is listed first.
const (
	LibraryProject = "project"
	LibraryGlobal  = "global"
	LibraryExtra   = "extra"
)

// RuleLibrary is one rules directory `hydra match` reads. Extra libraries are
// directories hydra does not manage (the dotfiles library, say); they are read
// in the same frontmatter dialect and never written.
type RuleLibrary struct {
	Kind    string `json:"kind"`
	Dir     string `json:"dir"`
	Present bool   `json:"present"`
}

// RuleHit is one matcher that fired: which pattern, against which subject.
type RuleHit struct {
	Kind    string `json:"kind"` // "path" | "command"
	Pattern string `json:"pattern"`
	Subject string `json:"subject"`
}

type RuleMatch struct {
	Rule    string      `json:"rule"`
	Title   string      `json:"title"`
	File    string      `json:"file"`
	Library RuleLibrary `json:"library"`
	Always  bool        `json:"always"`
	Matched []RuleHit   `json:"matched"`
}

// RuleMatchError names a file that could not be read or parsed. It is reported
// beside the matches rather than instead of them.
type RuleMatchError struct {
	Library RuleLibrary `json:"library"`
	File    string      `json:"file"`
	Error   string      `json:"error"`
}

// RuleMatchReport is `hydra match --json`. Every list is present, empty rather
// than null, so a script can range over it without a nil check.
type RuleMatchReport struct {
	Root      string           `json:"root"`
	Paths     []string         `json:"paths"`
	Commands  []string         `json:"commands"`
	Libraries []RuleLibrary    `json:"libraries"`
	Matches   []RuleMatch      `json:"matches"`
	Errors    []RuleMatchError `json:"errors"`
}

type MatchRequest struct {
	Root          string // the project root; relative path subjects resolve against it
	Home          string // expands a leading ~/ in a pattern
	Libraries     []RuleLibrary
	Paths         []string
	Commands      []string
	IncludeAlways bool
}

// MatchRules decides which rules' paths: and commands: fire for the given
// subjects. It is the mechanical half of rule routing; triggers: describe
// situations and stay with the agent's judgement.
//
// It reads several libraries but merges nothing: each match names the library
// it came from, and the libraries are reported in precedence order. Always-on
// rules are skipped unless asked for, because their bodies are already inlined
// in standing context.
func MatchRules(req MatchRequest) RuleMatchReport {
	rep := RuleMatchReport{
		Root:      req.Root,
		Paths:     []string{},
		Commands:  []string{},
		Libraries: []RuleLibrary{},
		Matches:   []RuleMatch{},
		Errors:    []RuleMatchError{},
	}
	for _, p := range req.Paths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(req.Root, p)
		}
		rep.Paths = append(rep.Paths, filepath.Clean(p))
	}
	rep.Commands = append(rep.Commands, req.Commands...)

	for _, lib := range req.Libraries {
		lib.Present = isDir(lib.Dir)
		rep.Libraries = append(rep.Libraries, lib)
		if !lib.Present {
			continue
		}
		rules, errs := LoadRulesTolerant(lib.Dir)
		for _, e := range errs {
			rep.Errors = append(rep.Errors, RuleMatchError{Library: lib, File: e.File, Error: e.Err.Error()})
		}
		for _, r := range rules {
			if r.Always && !req.IncludeAlways {
				continue
			}
			hits := ruleHits(r, lib, req, rep.Paths, rep.Commands)
			if len(hits) == 0 {
				continue
			}
			rep.Matches = append(rep.Matches, RuleMatch{
				Rule: r.Name, Title: r.Title, File: r.Path, Library: lib, Always: r.Always, Matched: hits,
			})
		}
	}
	return rep
}

func ruleHits(r Rule, lib RuleLibrary, req MatchRequest, paths, commands []string) []RuleHit {
	var hits []RuleHit
	for _, subject := range paths {
		for _, pattern := range r.Paths {
			if pathMatches(pattern, subject, lib.Kind, req.Root, req.Home) {
				hits = append(hits, RuleHit{Kind: "path", Pattern: pattern, Subject: subject})
				break
			}
		}
	}
	for _, subject := range commands {
		for _, pattern := range r.Commands {
			if CommandMatches(pattern, subject) {
				hits = append(hits, RuleHit{Kind: "command", Pattern: pattern, Subject: subject})
				break
			}
		}
	}
	return hits
}

// pathMatches applies one paths: glob to an absolute subject.
//
// A project rule is anchored at the project root: it sees the path relative to
// the root and nothing outside it, so `app/**` means this project's app/. A
// global or extra rule is loaded from every directory, so it also sees the
// absolute path, and `**/.env*` matches a .env anywhere on disk.
func pathMatches(pattern, abs, kind, root, home string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "~/") && home != "" {
		pattern = filepath.ToSlash(filepath.Join(home, pattern[2:]))
	}

	if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if GlobMatch(pattern, filepath.ToSlash(rel)) {
			return true
		}
	}
	if kind == LibraryProject {
		return false
	}
	return GlobMatch(strings.TrimPrefix(pattern, "/"), strings.TrimPrefix(filepath.ToSlash(abs), "/"))
}

// GlobMatch reports whether a /-separated path matches a paths: glob.
//
// `*`, `?` and `[...]` match within one segment; a `**` segment matches zero or
// more whole segments. A pattern with no slash matches the basename at any
// depth, as gitignore does, so `*.php` means every PHP file.
func GlobMatch(pattern, name string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "/") {
		ok, _ := path.Match(pattern, path.Base(name))
		return ok
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			// Collapse runs of ** — they mean the same as one.
			for len(pattern) > 1 && pattern[1] == "**" {
				pattern = pattern[1:]
			}
			rest := pattern[1:]
			for i := 0; i <= len(name); i++ {
				if matchSegments(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pattern[0], name[0]); !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// CommandMatches reports whether a commands: pattern occurs in a shell command
// line, anchored at word boundaries.
//
// Whitespace is collapsed on both sides, so line breaks and doubled spaces do
// not matter. The pattern may start anywhere — `artisan test` matches
// `php artisan test`, `vendor/bin/pest` matches `./vendor/bin/pest` — but
// it must not start or end inside a word: `warden` does not match `wardenx`.
// A pattern's own trailing space (the dotfiles library writes `"warden "`) is
// redundant with that rule and trimmed. Quoting is not parsed, so
// `echo "git tag"` matches `git tag`; that costs one rule read, never more.
func CommandMatches(pattern, command string) bool {
	pattern = strings.Join(strings.Fields(pattern), " ")
	command = strings.Join(strings.Fields(command), " ")
	if pattern == "" {
		return false
	}
	for from := 0; from <= len(command)-len(pattern); {
		i := strings.Index(command[from:], pattern)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(pattern)
		if (start == 0 || !isWordByte(command[start-1])) && (end == len(command) || !isWordByte(command[end])) {
			return true
		}
		from = start + 1
	}
	return false
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// Exit codes for hydra match: a script branches on these without parsing.
const (
	matchExitNone  = 1
	matchExitUsage = 2
)

func newMatchCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "match",
		Short: "Check which rules a file or command fires",
		Long: "Decide which rules' paths: and commands: matchers fire for the given files\n" +
			"and shell commands — the mechanical half of rule routing. triggers: describe\n" +
			"situations and are left to the agent's judgement, so they never match here.\n\n" +
			"Reads the project library (./.hydra/rules), the global library, and every\n" +
			"--library directory, and writes nothing. --global reads the global library\n" +
			"alone (the same as --no-project). Always-on rules are skipped unless\n" +
			"--include-always is given, because they are already in standing context.\n\n" +
			"Matching:\n" +
			"  paths     globs; * and ? within a segment, ** across segments. A pattern with\n" +
			"            no slash matches the file name at any depth. Project rules see the\n" +
			"            path relative to the project; global and --library rules also see\n" +
			"            the absolute path.\n" +
			"  commands  found anywhere in the command line, but never starting or ending\n" +
			"            inside a word: 'artisan test' matches 'php artisan test', and\n" +
			"            'warden' does not match 'wardenx'.\n\n" +
			"Exit status: 0 when a rule matched, 1 when none did, 2 for a usage error.\n" +
			"--json prints a report in all three cases.",
		Example: "  hydra match --path app/Jobs/SendMail.php\n" +
			"  hydra match --command 'php artisan test' --json\n" +
			"  hydra match --path ~/AI/warden/README.md --library ~/AI/dotfiles/rules --json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			usage := func(msg string) error {
				if asJSON {
					data, _ := json.MarshalIndent(map[string]string{"error": msg}, "", "  ")
					fmt.Fprintln(out, string(data))
					return &exitCodeError{code: matchExitUsage}
				}
				return &exitCodeError{code: matchExitUsage, msg: msg}
			}

			paths, _ := cmd.Flags().GetStringArray("path")
			commands, _ := cmd.Flags().GetStringArray("command")
			extras, _ := cmd.Flags().GetStringArray("library")
			noProject, _ := cmd.Flags().GetBool("no-project")
			noGlobal, _ := cmd.Flags().GetBool("no-global")
			global, _ := cmd.Flags().GetBool("global")
			includeAlways, _ := cmd.Flags().GetBool("include-always")
			if len(paths) == 0 && len(commands) == 0 {
				return usage("give at least one --path or --command")
			}

			req, err := matchRequestFromCmd(cmd, extras, noProject || global, noGlobal)
			if err != nil {
				return usage(err.Error())
			}
			req.Paths, req.Commands, req.IncludeAlways = paths, commands, includeAlways
			rep := MatchRules(req)

			if asJSON {
				data, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
			} else {
				renderMatchText(out, rep)
			}
			if len(rep.Matches) == 0 {
				return &exitCodeError{code: matchExitNone}
			}
			return nil
		},
	}
	cmd.Flags().StringArray("path", nil, "file the agent is about to read or write; relative to the current directory (repeatable)")
	cmd.Flags().StringArray("command", nil, "shell command the agent is about to run (repeatable)")
	cmd.Flags().StringArray("library", nil, "another rules directory to read, e.g. ~/AI/dotfiles/rules (repeatable)")
	cmd.Flags().Bool("no-project", false, "skip the project library")
	cmd.Flags().Bool("no-global", false, "skip the global library")
	cmd.Flags().Bool("include-always", false, "also report always-on rules")
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &exitCodeError{code: matchExitUsage, msg: err.Error()}
	})
	return cmd
}

// matchRequestFromCmd resolves the libraries a match reads. The global library
// is optional here, unlike for --global commands: a machine with no home
// directory still has its project rules to match.
func matchRequestFromCmd(cmd *cobra.Command, extras []string, noProject, noGlobal bool) (MatchRequest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return MatchRequest{}, fmt.Errorf("cannot resolve the current directory: %w", err)
	}
	home, _ := os.UserHomeDir()
	req := MatchRequest{Root: cwd, Home: home}

	if !noProject {
		req.Libraries = append(req.Libraries, RuleLibrary{Kind: LibraryProject, Dir: filepath.Join(cwd, ".hydra", "rules")})
	}
	if !noGlobal {
		flagHome, envHome := hydraHomeOverrides(cmd)
		if hydraHomeOverridden(flagHome, envHome) || home != "" {
			globalHome, _, err := resolveHydraHome(flagHome, envHome, home)
			if err != nil {
				return MatchRequest{}, err
			}
			req.Libraries = append(req.Libraries, RuleLibrary{Kind: LibraryGlobal, Dir: filepath.Join(globalHome, "rules")})
		}
	}
	for _, dir := range extras {
		expanded, err := expandTilde(dir, home)
		if err != nil {
			return MatchRequest{}, fmt.Errorf("--library %q: %w", dir, err)
		}
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return MatchRequest{}, fmt.Errorf("--library %q: %w", dir, err)
		}
		req.Libraries = append(req.Libraries, RuleLibrary{Kind: LibraryExtra, Dir: abs})
	}
	return req, nil
}

func renderMatchText(out io.Writer, rep RuleMatchReport) {
	if len(rep.Matches) == 0 {
		fmt.Fprintln(out, "No rule matches.")
	} else {
		fmt.Fprintf(out, "%d rule(s) match\n", len(rep.Matches))
	}
	for _, m := range rep.Matches {
		fmt.Fprintf(out, "\n%s (%s)\n", m.Title, m.Rule)
		fmt.Fprintf(out, "  Library: %s %s\n", m.Library.Kind, m.Library.Dir)
		for _, hit := range m.Matched {
			fmt.Fprintf(out, "  Matched: %s %q ← %s\n", hit.Kind, hit.Pattern, hit.Subject)
		}
		if m.Always {
			fmt.Fprintln(out, "  Always loaded: already in standing context")
		}
		fmt.Fprintf(out, "  File: %s\n", m.File)
	}

	fmt.Fprintln(out, "\nChecked")
	for _, p := range rep.Paths {
		fmt.Fprintf(out, "  path     %s\n", p)
	}
	for _, c := range rep.Commands {
		fmt.Fprintf(out, "  command  %s\n", c)
	}
	for _, lib := range rep.Libraries {
		state := ""
		if !lib.Present {
			state = " (not present)"
		}
		fmt.Fprintf(out, "  library  %s %s%s\n", lib.Kind, lib.Dir, state)
	}
	if len(rep.Errors) > 0 {
		fmt.Fprintln(out, "\nCould not read")
		for _, e := range rep.Errors {
			fmt.Fprintf(out, "  %s\n", e.Error)
		}
	}
	if len(rep.Matches) == 0 {
		fmt.Fprintln(out, "\nOnly paths: and commands: are checked here. A rule meant for this work")
		fmt.Fprintln(out, "may be described by its triggers instead, which the agent judges; to make it")
		fmt.Fprintln(out, "fire mechanically, add a paths: or commands: matcher to it.")
	}
	fmt.Fprintln(out, "\nFor agents and scripts: hydra match --path <file> --command <cmd> --json")
}
