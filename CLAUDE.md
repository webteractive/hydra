# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# hydra — installable rules and abilities curator (Go CLI)

hydra is a single self-contained Go binary that installs scoped rules and global,
lazy-loaded abilities for AI coding agents. Rules live in `.hydra/rules/` (or
`~/.hydra/rules/` with `--global`). Ability bundles live in `~/.hydra/abilities/`, stay
outside standing context, and can be selected by the agent or invoked through the
generated `$ability` router. Commands: `init`, `sync`, `add`, `new`, `list`, `doctor`,
`ability init|sync|new|list|doctor`, `self-update`.

## Build & test
- `go test ./...` — full suite (temp-dir based, no network).
- `go vet ./...` and `gofmt -l .` — must be clean (CI enforces both).
- `govulncheck ./...` — must report no vulnerabilities (CI enforces). Findings here are
  usually standard-library ones cleared by a Go patch release rather than by code changes.
  `go.mod` carries a `toolchain` floor for exactly that reason: raise it when a new
  advisory lands, and every build — local, CI, and release — honours it without anyone
  needing the right Go installed system-wide.
- `goreleaser check` — validates `.goreleaser.yaml` (CI enforces, so a broken release
  config surfaces on the PR rather than after a tag is pushed).
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
- `ability.go` / `ability_scope.go` — ability metadata, validation, and the global scope.
- `ability_render.go` / `ability_harness.go` — external catalog, discovery block, and
  native router adapters.
- `ability_lifecycle.go` / `ability_list.go` / `ability_doctor.go` / `ability_cmd.go` —
  the global ability command group.
- `teardown.go` — removes v0.1 skill-curator artifacts.
- `VERSION` — embedded default version; release builds inject the tag via
  `-ldflags "-X main.injectedVersion=..."` (see `.goreleaser.yaml`).

## Conventions
- **`CLAUDE.md` and `AGENTS.md` are mirrors** — identical body, only the top title/intro
  line differs. Any edit to one MUST be replicated to the other in the same change.
- Stdlib + cobra + `gopkg.in/yaml.v3` only — keep the dependency surface minimal.
- **No MCP.** hydra is CLI-only by design; `hydra add` records rules and
  `hydra ability new` scaffolds authored ability bundles.
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
