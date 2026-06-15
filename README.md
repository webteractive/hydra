# hydra

A CLI for managing a library of reusable skills for AI coding agents (Claude Code and
others). It sets up a small curator workflow in a project: before real work, the agent
checks the library and captures a new skill when a problem is worth reusing.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/webteractive/hydra/main/install.sh | sh
```

Downloads the latest prebuilt binary for your platform, verifies the checksum, and installs
it to `~/.local/bin`.

## Quick start

```bash
cd your-project
hydra init      # scaffold .hydra/, wire the hook, seed the skill-curator skill
hydra doctor    # check the install
```

Run `hydra init --global` to set it up once for every project (library in `~/.hydra/`, wired
into `~/.claude` and `~/.agents`).

## Commands

| Command | Description |
|---|---|
| `hydra init [--global]` | Set up the curator in a project, or globally. |
| `hydra sync [--global]` | Rebuild skill symlinks from `.hydra/skills/`. |
| `hydra new <name>` | Create a new skill and sync it. |
| `hydra log <CREATE\|UPDATE\|RENAME> <skill> <reason>` | Record a change in `.hydra/curator.log`. |
| `hydra doctor [--global]` | Check that everything is wired up. |
| `hydra self-update` | Update to the latest release. |

## How it works

`hydra init` scaffolds a `.hydra/` directory and seeds one skill, `skill-curator`, which
runs on each prompt through a `UserPromptSubmit` hook:

```
scan → decide → build | update | use | inline → sync → log
```

Skills live in `.hydra/skills/<name>/SKILL.md` and are symlinked into the agent runtimes
(`.claude/skills`, `.agents/skills`) by `hydra sync`. Build a skill when the problem is
likely to recur; otherwise handle it inline and create nothing.

## Development

```bash
go test ./...
go vet ./...
go build -o hydra .
```

The source is the `*.go` files plus `assets/`, embedded with `//go:embed`.
