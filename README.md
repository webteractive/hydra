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
| `hydra list [--json]` | List rules with their matchers. |
| `hydra doctor [--json]` | Check that everything is wired up. |
| `hydra ability init` | Initialize the global abilities catalog and harness routers. |
| `hydra ability sync` | Validate abilities and refresh generated wiring. |
| `hydra ability new <name>` | Scaffold `~/.hydra/abilities/<name>/ABILITY.md`. |
| `hydra ability list [--json]` | List abilities without loading their bodies. |
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

`ABILITY.md` requires `name` and `description` frontmatter. `hydra ability sync` keeps a
searchable external catalog at `~/.hydra/abilities/index.md`; descriptions and bodies are
not copied into standing agent context. A short managed instruction lets the agent decide
when to search that catalog and load one complete ability.

Hydra also installs one small native router skill for each detected supported harness.
Use `$ability <name>` when you want deterministic, explicit loading. Hydra currently has
adapters for Claude Code, Codex/Agent Skills, and Gemini.

## Development

```bash
go test ./...
go vet ./...
go build -o hydra .
```
