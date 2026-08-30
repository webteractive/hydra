# hydra

A CLI for scoped rules and lazy-loaded abilities for AI coding agents. Rules are
mandatory conventions selected by path, command, or situation. Abilities are optional
global workflow bundles selected by the agent or invoked explicitly with `$ability`.

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
hydra ability new testing-notes
hydra doctor
hydra ability doctor
```

Run any command with `--global` to operate on `~/.hydra/rules/` instead, wired into
`~/.claude/CLAUDE.md`. The two libraries are independent — the agent loads both.

Abilities are always global under `~/.hydra/abilities/`. A normal `hydra init` enables
them for fresh installations; existing installations can opt in with
`hydra ability init`.

## Commands

| Command | Description |
|---|---|
| `hydra init [--global]` | Scaffold the rules scope and ensure global abilities are wired. |
| `hydra sync [--global]` | Reindex and rewrite every managed block. |
| `hydra add …` | Record a rule. Initializes the library if it doesn't exist. |
| `hydra new <name>` | Scaffold a blank rule for hand-editing. |
| `hydra list [--json]` | Show labeled rule details for people; use `--json` for agents and scripts. |
| `hydra doctor [--json]` | Check that everything is wired up. |
| `hydra ability init` | Initialize the global abilities catalog and harness routers. |
| `hydra ability sync` | Validate abilities and refresh generated wiring. |
| `hydra ability new <name>` | Scaffold `~/.hydra/abilities/<name>/ABILITY.md`. |
| `hydra ability list [--json]` | Show descriptions and invocation hints for people; use `--json` for agents and scripts. |
| `hydra ability match <phrase>` | Check which ability a phrase would invoke. Exits non-zero when nothing matches. |
| `hydra ability doctor [--json]` | Check the global catalog, blocks, and routers. |
| `hydra self-update` | Update to the latest release. |

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
Use `$ability <name>` when you want deterministic, explicit loading. Hydra currently has
adapters for Claude Code and Codex/Agent Skills.

Gemini was supported through v0.2 and has been removed. Cleanup follows the same scoping
as the wiring: `hydra init` strips the rules block from that scope's `GEMINI.md`, while
`hydra ability init` strips the abilities block from `~/.gemini/GEMINI.md` and deletes the
router it installed. Since a plain `hydra init` runs both, either path cleans up. Your own
prose and any skill hydra does not own are left untouched, and `hydra doctor` plus
`hydra ability doctor` report anything outstanding.

## Development

```bash
go test ./...
go vet ./...
go build -o hydra .
```
