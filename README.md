# hydra

A CLI that installs two kinds of instruction into your AI coding agents. **Rules** are
mandatory conventions, selected by path, command, or situation. **Abilities** are optional
global workflow bundles, invoked by name or by a trigger phrase, chosen semantically when
neither matches, or loaded explicitly with `$ability`.

Both keep their bulk out of standing context without hiding their existence: the agent
always sees a compact table of what is available, and opens the full file only for the
entries that actually fire.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/webteractive/hydra/main/install.sh | sh
```

Downloads the latest prebuilt binary for your platform, verifies the checksum, and installs
it to `~/.local/bin`.

## Quick start

```bash
cd your-project
hydra init      # scaffold .hydra/rules/ and wire the block into CLAUDE.md / AGENTS.md
hydra add --glob 'app/Http/Controllers/**' \
          --title 'Extend BaseController' \
          --note  'Every controller extends BaseController for tenant scoping.'
hydra ability new testing-notes          # then fill in its description and triggers
hydra ability match "write test notes"   # verify a trigger fires (exit 1 if none does)
hydra doctor
hydra ability doctor
```

Run any command with `--global` to operate on `~/.hydra/rules/` instead, wired into
`~/.claude/CLAUDE.md`. The two libraries are independent — the agent loads both.

Abilities are always global under `~/.hydra/abilities/`. A normal `hydra init` enables
them for fresh installations; existing installations can opt in with
`hydra ability init`.

### Moving the global library

`~/.hydra` is the default, not a requirement. Point `HYDRA_HOME` at an absolute path —
somewhere version-controlled, say — and the global rules and abilities libraries both
move there:

```bash
export HYDRA_HOME="$HOME/dotfiles/hydra"   # holds rules/ and abilities/
```

Only the library moves. The managed blocks stay in `~/.claude/CLAUDE.md` and
`~/.codex/AGENTS.md`, where the agents look for them; they simply reference the new
location. Project rules are unaffected — `.hydra/rules/` stays in the project, because a
project rule is meant to travel with its repository.

`--hydra-home <path>` overrides the variable for a single run, which is mainly useful for
probing another library without touching your environment. Both must be absolute:
resolving a relative path against the working directory would scaffold a *global* library
inside whatever repository happened to be current.

To move a library you already have, use `hydra relocate` rather than `mv` — it moves the
directory *and* rewrites the managed blocks, which hold absolute paths into the directory
you are about to move:

```
$ hydra relocate ~/dotfiles/hydra
moved /Users/you/.hydra -> /Users/you/dotfiles/hydra
indexed 7 rule(s) → 1 target(s)
indexed 4 ability(s) → 2 harness(es)

Add this to your shell profile to make it stick:
  export HYDRA_HOME="/Users/you/dotfiles/hydra"
```

It refuses a destination that already holds anything, and copies across filesystems when
rename cannot (a home directory and a dotfiles repository on separate volumes). The
destination may be relative — unlike `HYDRA_HOME`, it was typed from a known directory.
Setting the variable is the one part it cannot do for you, so it prints the line to add.

Because global blocks embed absolute paths, a shell that lost the variable would resolve
`~/.hydra` and rewrite every block back to it. So every command that writes a global block —
`init`, `sync`, `add`, `new`, `relocate`, and the `ability` ones, including the abilities half
of a plain project `hydra init` — first reads which library the existing block names, and
refuses with exit status 2 when it is not the one this run resolved, before creating
anything:

```
refusing to rewrite the managed block in /Users/you/.claude/CLAUDE.md:
  the block names the library at /Users/you/dotfiles/hydra
  this run resolved              /Users/you/.hydra (default)
Writing now would point every agent at the wrong library. If the block is right, run
  export HYDRA_HOME="/Users/you/dotfiles/hydra"
and try again; to point the block at /Users/you/.hydra instead, pass --force.
```

`relocate` rewires the blocks itself, so its own rewrite is exempt — but it refuses to move
a library the blocks do not name, which would hide the one they do. `hydra doctor` and
`hydra ability doctor` report the same mismatch as an error and advise the variable rather
than a sync the guard would refuse. They also name the library they read and what put it
there:

```
hydra doctor (global: /Users/you/dotfiles/hydra via HYDRA_HOME)
```

Scripts get the same two facts as `home` and `home_source` in `--json`.

## Commands

| Command | Description |
|---|---|
| `hydra init [--global]` | Scaffold the rules scope and ensure global abilities are wired. |
| `hydra sync [--global]` | Reindex and rewrite every managed block. |
| `hydra add …` | Record a rule. Initializes the library if it doesn't exist. |
| `hydra new <name>` | Scaffold a blank rule for hand-editing. |
| `hydra list [--json]` | Show labeled rule details for people; use `--json` for agents and scripts. |
| `hydra doctor [--json]` | Check that everything is wired up. |
| `hydra match --path <file> --command <cmd> [--json]` | Check which rules a file or command fires. Exits 1 when none does. |
| `hydra relocate <path>` | Move the global library and rewrite every managed block. |
| `hydra ability init` | Initialize the global abilities catalog and harness routers. |
| `hydra ability sync` | Validate abilities and refresh generated wiring. |
| `hydra ability new <name>` | Scaffold `~/.hydra/abilities/<name>/ABILITY.md`. |
| `hydra ability list [--json]` | Show descriptions and invocation hints for people; use `--json` for agents and scripts. |
| `hydra ability match <phrase>` | Check which ability a phrase would invoke. Exits non-zero when nothing matches. |
| `hydra ability doctor [--json]` | Check the global catalog, blocks, and routers. |
| `hydra self-update` | Update to the latest release. |

Every command also takes `--hydra-home <path>`, the single-run form of `HYDRA_HOME`.

## How it works

A rule is one Markdown file with frontmatter declaring when it fires:

```markdown
---
paths:    ["**/Cargo.toml"]
commands: ["cargo add"]
triggers: ["auditing a Rust dependency"]
---

# Rust dependencies

## Pin the exact version
...
```

`hydra sync` renders every rule into an index table and splices it into your agent
instruction files between `<!-- hydra:rules:start -->` sentinels. The agent reads the
table on every prompt and opens only the rule files whose matchers hit. Rules marked
`always: true` are inlined into the block instead of indexed.

### How a match is decided

`paths` and `commands` are mechanical, so hydra can decide them without an agent:

```bash
hydra match --path app/Jobs/SendMail.php
hydra match --command 'php artisan test' --json
```

- **`paths`** are globs. `*` and `?` match within one path segment and `**` matches any
  number of segments, so `app/Jobs/**` covers everything under `app/Jobs/`. A pattern with
  no slash matches the file name at any depth, as in `.gitignore`: `*.php` means every PHP
  file. A project rule is matched against the path relative to the project root. A global
  rule is loaded from every directory, so it is also matched against the absolute path —
  `**/.env*` fires for a `.env` anywhere on disk.
- **`commands`** match anywhere in the command line, but never starting or ending inside a
  word: `artisan test` matches `php artisan test`, `vendor/bin/pest` matches
  `./vendor/bin/pest`, and `warden` does not match `wardenx`. Whitespace is collapsed, and
  quoting is not parsed, so `echo "git tag"` matches `git tag`.
- **`triggers`** describe situations. Only an agent can judge those, so `match` never
  reports them.

`match` reads the project library, the global library, and any `--library <dir>` you add.
An extra library may use the dotfiles dialect: a `title:` key names a rule whose body has no
H1 (an H1 always wins), `when:` is ignored, and a `README.md` without frontmatter is skipped. It writes nothing, reports each match with the
library and pattern that produced it, and lists unparseable files beside the matches
rather than failing. Always-on rules are left out unless you pass `--include-always`,
since they are already in context. Exit status is 0 when a rule matched, 1 when none did,
and 2 for a usage error; `--json` prints a report in every case.

`hydra doctor --json` and `hydra ability doctor --json` give each check a `fix` — the
command that clears it, as an argument vector, scoped like the report (`hydra sync
--global` for the global library) — and an `initialized` flag that tells "no library here"
apart from a broken one.

### Abilities

An ability is a directory containing an `ABILITY.md` plus optional supporting resources:

```text
~/.hydra/abilities/testing-notes/
├── ABILITY.md
├── references/
├── scripts/
└── assets/
```

`ABILITY.md` requires `name` and `description` frontmatter and accepts an optional
`triggers` list of short phrases a user would actually say:

```yaml
---
name: prepare-for-production
description: Review and harden a PHP or Laravel change for production.
triggers:
  - make it production ready
  - primetime
---
```

Abilities load the way agent skills do. `hydra ability sync` inlines each ability's name,
triggers, and description into the managed instruction block, and keeps the same catalog
at `~/.hydra/abilities/index.md`. Only the authored body stays lazy — it is read when the
ability is selected, not before. Metadata has to be in standing context for an ability to
be selected at all.

Before selecting another reusable workflow, the managed instruction requires the agent to
check that table. An exact normalized ability-name match or a trigger match is an explicit
invocation and takes priority; otherwise the agent can select an ability semantically.
Either way it then loads the complete `ABILITY.md`.

Only names, triggers, and descriptions are inlined — the `File` column stays in
`index.md`, since every ability resolves to `~/.hydra/abilities/<name>/ABILITY.md` and
standing context is the one place a redundant column costs something on every turn.

A trigger fires only when it is what the user actually said — the whole request, aside
from case, punctuation, and politeness like "can you" or "please". A trigger occurring
inside a sentence about other work is not an invocation, so `what changed` invokes but
`what changed in the nginx config?` does not. That looser phrasing is not lost: it falls
through to semantic description matching, which is the recall tier. Triggers are the
precision tier.

Because trigger matching is what makes invocation deterministic, you can check a phrase
without starting an agent session:

```bash
hydra ability match "Primetime!"
# "Primetime!" → prepare-for-production
#   Matched: trigger "primetime"

hydra ability match "prep for prod"
# no name or trigger match  (exit 1)
```

Two abilities may share a trigger. That is not an error: `match` returns every candidate
with its description, and the managed instruction tells the agent to pick the one that
fits what the user is actually doing and say which it picked. Hydra cannot see the
conversation that settles it, so it does not guess.

An exact ability-name match is decisive and suppresses trigger candidates, so
`hydra ability doctor` warns about a trigger that normalizes to some *other* ability's
name — that trigger can never fire. It also warns about abilities with no triggers.

Hydra also installs one small native router skill for each detected supported harness.
Use `$ability <name>` when you want deterministic, explicit loading. A bare `$ability`
asks you to choose through the harness's own selection prompt (`AskUserQuestion` in
Claude Code, `request_user_input` in Codex), paging with a `More…` option when the catalog
is larger than one question holds; where that tool is unavailable it falls back to a
numbered list and waits for your reply. Hydra currently has adapters for Claude Code and
Codex/Agent Skills.

Gemini was supported through v0.2 and has been removed. Cleanup follows the same scoping
as the wiring: `hydra init` strips the rules block from that scope's `GEMINI.md`, while
`hydra ability init` strips the abilities block from `~/.gemini/GEMINI.md` and deletes the
router it installed. Since a plain `hydra init` runs both, either path cleans up. Your own
prose and any skill hydra does not own are left untouched, and `hydra doctor` plus
`hydra ability doctor` report anything outstanding.

## Claude Code integration

`integrations/claude-code/` is a Claude Code mod that does the mechanical half of this
routing in code: it routes a prompt that is exactly an ability's name or trigger to that
ability, delivers a rule the first time a file or command its `paths` / `commands` cover is
touched, and runs the doctors so a stale library is named with the command that fixes it.
It calls the CLI for every decision and changes nothing for other harnesses. Load it with
`claude --plugin-dir integrations/claude-code` or `CLAUDE_CODE_PLUGIN_DIRS`; see its
[README](integrations/claude-code/README.md).

## Development

```bash
go test -race ./...   # CI runs the suite with the race detector
go vet ./...
gofmt -l .            # must print nothing
govulncheck ./...
goreleaser check      # validates .goreleaser.yaml
go build -o hydra .
```

The Claude Code mod is checked locally rather than in CI. Run both before every release:

```bash
claude plugin validate integrations/claude-code
claude plugin test integrations/claude-code
```

CI enforces gofmt, vet, the race-enabled suite, and govulncheck, with `goreleaser check`
as a separate job so a broken release config surfaces on the PR rather than after a tag
is pushed.

Releases are cut by pushing a `vX.Y.Z` tag; `.github/workflows/release.yml` runs
GoReleaser to build and attach the binaries.
