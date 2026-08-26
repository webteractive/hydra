# hydra

A CLI for managing a library of scoped rules for AI coding agents (Claude Code and
others). Rules are committed Markdown; hydra keeps an index of them in your agent
instruction files so an agent reads only the ones that apply to what it's doing.

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
hydra doctor
```

Run any command with `--global` to operate on `~/.hydra/rules/` instead, wired into
`~/.claude/CLAUDE.md`. The two libraries are independent — the agent loads both.

## Commands

| Command | Description |
|---|---|
| `hydra init [--global]` | Scaffold the library and wire it into your agent files. |
| `hydra sync [--global]` | Reindex and rewrite every managed block. |
| `hydra add …` | Record a rule. Initializes the library if it doesn't exist. |
| `hydra new <name>` | Scaffold a blank rule for hand-editing. |
| `hydra list [--json]` | List rules with their matchers. |
| `hydra doctor [--json]` | Check that everything is wired up. |
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

## Development

```bash
go test ./...
go vet ./...
go build -o hydra .
```
