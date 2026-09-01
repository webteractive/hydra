package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// AbilityMatchKind distinguishes the two deterministic paths an agent is told
// to treat as an explicit invocation. Semantic selection is deliberately not a
// kind: it is the agent's judgement and Hydra cannot verify it offline.
type AbilityMatchKind string

const (
	MatchName    AbilityMatchKind = "name"
	MatchTrigger AbilityMatchKind = "trigger"
)

type AbilityMatch struct {
	Ability     string           `json:"ability"`
	Kind        AbilityMatchKind `json:"kind"`
	Trigger     string           `json:"trigger,omitempty"`
	Description string           `json:"description,omitempty"`
	Path        string           `json:"path"`
}

// MatchAbilities resolves a phrase the way the managed instruction block tells
// the agent to: an exact normalized name match first, then a trigger that is
// itself the whole request. A trigger merely occurring inside a sentence about
// other work is not an invocation — see anchoredMatch.
//
// A name match is decisive — typing the name is explicit intent, and returning
// trigger candidates alongside it would only dilute that. Trigger matches are
// returned in full, descriptions included: Hydra cannot see the conversation
// that would settle which of two overlapping triggers the user meant, so it
// hands the caller a candidate set to choose from rather than guessing.
func MatchAbilities(abilities []Ability, phrase string) []AbilityMatch {
	raw := matchTokens(phrase)
	if raw == "" {
		return nil
	}
	stripped := stripFiller(raw)
	rawName, strippedName := kebabForm(raw), kebabForm(stripped)

	var named []AbilityMatch
	for _, ability := range abilities {
		if rawName == ability.Name || strippedName == ability.Name {
			named = append(named, newAbilityMatch(ability, MatchName, ""))
		}
	}
	if len(named) > 0 {
		return named
	}

	var matches []AbilityMatch
	for _, ability := range abilities {
		for _, trigger := range ability.Triggers {
			if anchoredMatch(raw, stripped, matchTokens(trigger)) {
				matches = append(matches, newAbilityMatch(ability, MatchTrigger, trigger))
				break
			}
		}
	}
	return matches
}

func newAbilityMatch(ability Ability, kind AbilityMatchKind, trigger string) AbilityMatch {
	return AbilityMatch{
		Ability:     ability.Name,
		Kind:        kind,
		Trigger:     trigger,
		Description: ability.Description,
		Path:        ability.Path,
	}
}

// matchTokens reduces wording to space-separated alphanumeric tokens so that
// casing, punctuation, and spacing cannot decide whether an ability fires.
func matchTokens(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		default:
			pendingSpace = true
		}
	}
	return b.String()
}

// triggerLeadIns and triggerTrailers are the only wording a trigger is allowed
// to be wrapped in. The set is deliberately small and closed: the fix for a
// phrasing it does not cover is to add a trigger, not to grow this list, which
// would otherwise creep until anchoring means nothing.
var (
	triggerLeadIns  = []string{"go ahead and", "can you", "could you", "would you", "will you", "please", "okay", "ok", "now", "so", "well", "hey", "just", "lets", "let s"}
	triggerTrailers = []string{"thank you", "for me", "thanks", "please", "pls", "now", "okay", "ok"}
)

// anchoredMatch reports whether a trigger is what the user actually said,
// rather than a phrase that happens to occur inside a sentence about something
// else. "ship it" invokes; "I need to ship it by Friday" does not.
//
// Both forms of the wording are tried, because filler is stripped from what the
// user typed but never from what the author wrote. Comparing only the stripped
// form would make any trigger that legitimately starts or ends with one of
// those words — "just do it", "ok go" — permanently unmatchable.
//
// This is the precision tier. Looser phrasings are not lost — they fall through
// to semantic description matching, which is where recall belongs.
func anchoredMatch(raw, stripped, phrase string) bool {
	return phrase != "" && (raw == phrase || stripped == phrase)
}

// kebabForm renders tokenized wording as an ability name for comparison.
func kebabForm(wording string) string {
	return strings.ReplaceAll(wording, " ", "-")
}

// stripFiller removes politeness and lead-in tokens from both ends so ordinary
// human phrasing does not defeat an otherwise exact invocation.
func stripFiller(wording string) string {
	for changed := true; changed; {
		changed = false
		for _, lead := range triggerLeadIns {
			if strings.HasPrefix(wording, lead+" ") {
				wording, changed = strings.TrimPrefix(wording, lead+" "), true
				break
			}
		}
		for _, trail := range triggerTrailers {
			if strings.HasSuffix(wording, " "+trail) {
				wording, changed = strings.TrimSuffix(wording, " "+trail), true
				break
			}
		}
	}
	return wording
}

func newAbilityMatchCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "match <phrase>",
		Short: "Check which ability a phrase would invoke",
		Long: "Resolve a phrase the way an agent is instructed to: exact name match first,\n" +
			"then trigger match. Use it to verify a trigger fires before relying on it.\n\n" +
			"Exits non-zero when nothing matches deterministically.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := abilityScopeFromCmd()
			if err != nil {
				return err
			}
			abilities, err := LoadAbilities(s.AbilitiesDir)
			if err != nil {
				return fmt.Errorf("invalid abilities:\n%w", err)
			}

			phrase := strings.Join(args, " ")
			matches := MatchAbilities(abilities, phrase)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				data, err := json.MarshalIndent(struct {
					Phrase  string         `json:"phrase"`
					Matches []AbilityMatch `json:"matches"`
				}{phrase, matches}, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
			} else {
				renderAbilityMatchText(out, phrase, matches)
			}
			if len(matches) == 0 {
				return errors.New("")
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "emit machine-readable JSON for agents and scripts")
	return cmd
}

func renderAbilityMatchText(out io.Writer, phrase string, matches []AbilityMatch) {
	if len(matches) == 0 {
		fmt.Fprintf(out, "%q → no name or trigger match\n\n", phrase)
		fmt.Fprintln(out, "An agent could still select semantically by description, but nothing here")
		fmt.Fprintln(out, "guarantees it. To make this phrase deterministic, add it to the ability's")
		fmt.Fprintln(out, "triggers: list, then run hydra ability sync.")
		fmt.Fprintln(out, "\nFor agents and scripts: hydra ability match <phrase> --json")
		return
	}

	if len(matches) > 1 {
		fmt.Fprintf(out, "%q → %d candidates, agent chooses\n", phrase, len(matches))
	} else {
		fmt.Fprintf(out, "%q → %s\n", phrase, matches[0].Ability)
	}
	for _, match := range matches {
		fmt.Fprintf(out, "\n%s\n", match.Ability)
		if match.Description != "" {
			fmt.Fprintf(out, "  %s\n", match.Description)
		}
		if match.Kind == MatchTrigger {
			fmt.Fprintf(out, "  Matched: trigger %q\n", match.Trigger)
		} else {
			fmt.Fprintln(out, "  Matched: exact ability name")
		}
		fmt.Fprintf(out, "  Source: %s\n", match.Path)
	}
	if len(matches) > 1 {
		fmt.Fprintln(out, "\nOverlapping triggers are not an error: the agent picks by description and")
		fmt.Fprintln(out, "conversation context. Narrow a trigger only if the wrong one keeps winning.")
	}
	fmt.Fprintln(out, "\nFor agents and scripts: hydra ability match <phrase> --json")
}
