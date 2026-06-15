# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.
It mirrors `CLAUDE.md` — keep the two in sync (see Conventions).

# hydra — installable skill curator (Go CLI)

hydra is a single self-contained Go binary that installs a "skill curator" mechanism into
any project (or globally with `--global`): it scaffolds a `.hydra/` skill library, symlinks
skills into agent runtimes (`.claude/skills`, `.agents/skills`), wires a `UserPromptSubmit`
hook, and appends a curator block to `CLAUDE.md`/`AGENTS.md`. Commands: `init`, `sync`,
`new`, `log`, `doctor`.

## Build & test
- `go test ./...` — full suite (temp-dir based, no network).
- `go vet ./...` and `gofmt -l .` — must be clean (CI enforces both).
- `go build -o hydra .` — local binary (gitignored).
- The CLI is parsed with `spf13/cobra`. Command logic lives in decoupled functions
  (`Init`/`Sync`/`New`/`Log`/`Doctor`) that take a resolved `Scope` plus an `io.Writer`,
  which keeps them directly unit-testable; cobra is only the parsing layer in `main.go`.

## Layout
- `main.go` — cobra wiring: root command, subcommands, the `--global` persistent flag.
- `scope.go` — `Scope` + `ResolveScope` (project `./.hydra` vs global `~/.hydra`).
- `sync.go` / `init.go` / `new.go` / `log.go` / `doctor.go` — one command each.
- `config.go` — `.hydra/config` parsing (`HYDRA_RUNTIMES`).
- `assets.go` + `assets/` — embedded via `//go:embed`: the skill-curator skill, the hook,
  the curator block, and the default config seeded into target projects. Edit `assets/`,
  then rebuild — there is no separate asset build step.
- `VERSION` — embedded default version; release builds inject the tag via
  `-ldflags "-X main.injectedVersion=..."` (see `.goreleaser.yaml`).

## Conventions
- **`CLAUDE.md` and `AGENTS.md` are mirrors** — identical body, only the top title/intro
  line differs. Any edit to one MUST be replicated to the other in the same change.
- Stdlib + cobra only — keep the dependency surface minimal.
- **Never auto-commit or push.** The git diff is the review gate; ask before `git commit`.
- Releases are cut by tagging `vX.Y.Z` and pushing the tag — goreleaser builds the binaries.
