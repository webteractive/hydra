# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# hydra — installable rules curator (Go CLI)

hydra is a single self-contained Go binary that installs a "rules" mechanism into any
project (or globally with `--global`): it scaffolds a `.hydra/rules/` library of
matcher-scoped Markdown rules, generates an index, and splices a managed block into
whatever agent instruction files already exist (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`).
Commands: `init`, `sync`, `add`, `new`, `list`, `doctor`, `self-update`.

## Build & test
- `go test ./...` — full suite (temp-dir based, no network).
- `go vet ./...` and `gofmt -l .` — must be clean (CI enforces both).
- `go build -o hydra .` — local binary (gitignored).
- The CLI is parsed with `spf13/cobra`. Command logic lives in decoupled functions
  (`Init`/`Sync`/`Add`/`New`/`List`/`Doctor`) that take a resolved `Scope` plus an
  `io.Writer`, which keeps them directly unit-testable; cobra is only the parsing
  layer in `main.go`.

## Layout
- `main.go` — cobra wiring: root command, subcommands, the `--global` persistent flag.
- `scope.go` — `Scope` + `ResolveScope`. Project scope renders relative refs; global
  renders absolute ones, because `~/.claude/CLAUDE.md` loads from any directory.
- `rule.go` — the `Rule` model and YAML frontmatter parsing.
- `render.go` — the index table and the managed block.
- `block.go` — sentinel splicing (`<!-- hydra:rules:start/end -->`), replace-in-place.
- `detect.go` — which agent instruction files exist at this scope.
- `init.go` / `sync.go` / `add.go` / `new.go` / `list.go` / `doctor.go` — one command each.
- `teardown.go` — removes v0.1 skill-curator artifacts.
- `VERSION` — embedded default version; release builds inject the tag via
  `-ldflags "-X main.injectedVersion=..."` (see `.goreleaser.yaml`).

## Conventions
- **`CLAUDE.md` and `AGENTS.md` are mirrors** — identical body, only the top title/intro
  line differs. Any edit to one MUST be replicated to the other in the same change.
- Stdlib + cobra + `gopkg.in/yaml.v3` only — keep the dependency surface minimal.
- **No MCP.** hydra is CLI-only by design; `hydra add` is the only write path.
- **Never auto-commit or push.** The git diff is the review gate; ask before `git commit`.
- Releases are cut by tagging `vX.Y.Z` and pushing the tag — goreleaser builds the binaries.

<!-- hydra:rules:start -->
## Rules

**Before you enter plan mode, run a command, or create/edit a file, you MUST
first:** find the index rows whose trigger, glob, or command covers what you are
about to do and read those rule files, then run
`grep -rin '<keyword>' .hydra/rules` to catch what the index alone misses. Do not act
until you are following every matching rule.

Project rules override global rules on conflict.

### Rules index

No rules recorded yet.
<!-- hydra:rules:end -->
