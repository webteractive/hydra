# hydra v0.2 — pivot from skill curator to rules

**Date:** 2026-08-26
**Status:** approved design, pending implementation plan
**Breaking:** yes. v0.2.0 replaces everything v0.1 did. No compatibility shims.

## Summary

hydra stops being a skill curator and becomes a **rules** CLI. It manages a library of
path-, command-, and situation-scoped Markdown rules and splices a generated index into
whatever agent instruction files a project (or the user's home directory) already has.

The curator is deleted outright: no `UserPromptSubmit` hook, no skill symlink farms, no
`scan → decide → build → sync → log` loop, no `hydra log`.

## Motivation

The curator nagged on every prompt through a hook, and paid for itself only on the rare
prompt that warranted a new skill. Rules invert that: the standing cost is one table in
context, and a rule's body is read only when the work actually touches its globs.

Prior art is Laravel Boost's Project Rules, and the pattern the author already runs by
hand in `~/.claude/CLAUDE.md` against `~/AI/dotfiles/rules`. hydra is the portable,
language-agnostic version of that.

### What was taken from Boost, and what was not

Taken:

- Rules as committed Markdown with `paths:` frontmatter, grouped by area.
- A generated index mapping matchers to rule files.
- One short always-loaded instruction telling the agent to consult the index before
  planning or editing, and to `grep` the rules directory for what a path match misses.
- Sentinel-delimited splicing into agent instruction files, replacing in place so
  re-running never appends a second copy (`GuidelineWriter`'s behavior).

Not taken:

- **MCP.** Boost records rules through a `record-rule` MCP tool because it already runs an
  MCP server. hydra is CLI-only, permanently. `hydra add` is the only write path.
- **Package detection / vendored guidelines.** Boost composes guidelines from installed
  Composer packages via `laravel/roster`. hydra has no equivalent and wants none.
- **Blade templating and `@scoped` extraction.** hydra's rules are plain Markdown; there is
  no always-on guideline blob for path-scoped content to escape from.
- **Pointing at the index instead of inlining it.** See "Generated artifacts".

## The model

### A rule

One Markdown file with YAML frontmatter in `<library>/rules/<name>.md`:

```markdown
---
always: false
paths:    ["**/Cargo.toml", "**/Cargo.lock"]
commands: ["cargo add", "cargo install"]
triggers:
  - adding, updating, or auditing a Rust dependency
  - a crate name looks like a typosquat of a well-known one
---

# Rust dependencies

## Pin the exact version
...
```

All four frontmatter keys are optional:

| Key | Type | Meaning |
|---|---|---|
| `always` | bool, default `false` | Inline this rule's body into the block instead of indexing it |
| `paths` | list of glob strings | Fires when a file matching one of these is read or written |
| `commands` | list of command prefixes | Fires when a matching command is about to be run |
| `triggers` | list of prose phrases | Fires when the described situation applies |

The rule's **title** is the first `#` heading in the body, falling back to a headline-cased
filename. The **name** is the filename stem; it is never stored in frontmatter.

A rule with `always: false` and no `paths`, `commands`, or `triggers` can never fire.
`doctor` reports it as an error.

### Two tiers

- **Always** (`always: true`) — body inlined verbatim into the managed block. This is the
  "Always applies" tier: git conventions, secrets handling, things that must be in context
  before the agent does anything. Not listed in the index; it is already present.
- **Indexed** (everything else) — one row in the index table. The body is read only when a
  matcher hits.

## Scopes

Two libraries, fixed locations, fully independent. They never read each other and hydra
never merges them; the harness loads both instruction files, so combining happens there.

| Scope | Library | Path rendering |
|---|---|---|
| Global (`--global`) | `~/.hydra/rules/` | **Absolute** — `/Users/x/.hydra/rules/rust.md` |
| Project (default) | `./.hydra/rules/` | **Relative** — `.hydra/rules/rust.md` |

Global renders absolute because `~/.claude/CLAUDE.md` is loaded from whatever working
directory the agent happens to be in; a relative path there resolves against the wrong
repo. Project renders relative so the rules survive a clone to a different checkout path.

Path rendering is a property of the scope, not a setting. There is no configuration file —
`.hydra/config` and `HYDRA_RUNTIMES` are deleted along with the symlink logic they drove.

**Conflict resolution** is stated in the project block's prose ("project rules override
global rules on conflict") and enforced by nothing. hydra does not detect cross-scope
conflicts; it has no visibility into the other scope by design.

## Harness detection

Targets are detected, never configured. hydra splices the block into every instruction
file that already exists at that scope:

| Scope | Candidates |
|---|---|
| Global | `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, `~/.gemini/GEMINI.md` |
| Project | `./CLAUDE.md`, `./AGENTS.md`, `./GEMINI.md` |

If a **project** has none of them, `init` creates `CLAUDE.md` and `AGENTS.md`. If the
**global** scope has none, `init` creates `~/.claude/CLAUDE.md`.

## Generated artifacts

`sync` parses the whole library once and emits both artifacts from that single render, so
they cannot drift.

### 1. `<library>/rules/index.md`

The canonical table, for humans and for `hydra list`:

```markdown
# Hydra Rules Index

<!-- generated by hydra — do not edit -->

| Applies when | Matches (path glob / command) | Rule |
| --- | --- | --- |
| adding or auditing a Rust dependency | `**/Cargo.toml` · `cargo add` | .hydra/rules/rust-dependencies.md |
```

`triggers` join with ` · ` into the first column; `paths` and `commands` join with ` · `
into the second. Rules with `always: true` are excluded — their bodies are already inlined.

### 2. The managed block

Spliced between sentinels into each detected target:

```markdown
<!-- hydra:rules:start -->
## Rules

Always-on rules are inlined below. The rest are lazy-loaded. **Before you enter plan mode,
run a command, or create/edit a file, you MUST first:** find the index rows whose trigger,
glob, or command covers what you are about to do and read those rule files, then run
`grep -rin '<keyword>' .hydra/rules` to catch what the index alone misses. Do not act until
you are following every matching rule.

### Always applies

<!-- bodies of always:true rules, inlined verbatim -->

### Rules index

<!-- the same table as index.md -->
<!-- hydra:rules:end -->
```

Both examples above are rendered in **project** flavor. In global scope every path in both
artifacts is absolute, including the grep hint — `grep -rin '<keyword>' /Users/x/.hydra/rules`
and `/Users/x/.hydra/rules/rust-dependencies.md`.

**Divergence from Boost, deliberate:** Boost's block *points at* `.ai/rules/index.md`,
costing the agent a file read before it can decide anything applies. hydra inlines the
table, so deciding is free and only matching rule bodies get opened. `index.md` is still
written as the canonical artifact.

### Splicing rules

- Match `<!-- hydra:rules:start -->…<!-- hydra:rules:end -->` and replace in place.
- Never append a second block; never disturb content around it.
- If no block exists, append to the end of the file, separated by a blank line.
- Create the file (and parent directory) if missing.
- Normalize runs of 3+ newlines to 2; ensure a trailing newline.

## Command surface

| Command | Behavior |
|---|---|
| `hydra init [--global]` | Idempotent. Runs v0.1 teardown if wreckage is found, creates the rules dir if missing, detects (or creates) targets, writes the block. |
| `hydra sync [--global]` | Reindex and rewrite every block. **Does not create** — on an uninitialized directory it reports that and exits non-zero. |
| `hydra add [--global] --title T --note N [--glob G]… [--command C]… [--trigger T]… [--always]` | Files a rule, then syncs. **Calls `init` first if the library does not exist.** |
| `hydra new <name> [--global]` | Scaffolds a blank rule with empty frontmatter, then syncs. Calls `init` first if needed. |
| `hydra list [--global] [--json]` | Rules with tier and matchers. |
| `hydra doctor [--global] [--json]` | Health check. |
| `hydra self-update` | Unchanged from v0.1. |

Removed: `hydra log`. The git diff on `.hydra/rules/` is a better record than a log file
that has to be kept honest.

`--glob`, `--command`, and `--trigger` are repeatable. `--title` and `--note` are required.
At least one matcher, or `--always`, is required.

### Filing heuristic for `hydra add`

Ported from Boost's `RuleRepository::write()`:

1. Derive an **area key**, in this precedence order regardless of flag order on the command
   line: the first `--glob` if any, else the first `--command`, else the title. For a glob,
   drop wildcard and dotted segments (`app/Http/Controllers/**` → `app/Http/Controllers`).
   For a command, use its first word. For a title, slugify it.
2. If an existing rule file shares that area key, append to it; otherwise create
   `<slug>.md`, where the slug is built from the trailing meaningful segments, extended
   leftward on collision, then suffixed `-2`, `-3` as a last resort.
3. Merge the new matchers into the target file's frontmatter (union, no duplicates).
4. Append `## <title>` followed by the note.
5. Reindex and rewrite blocks.

## v0.1 teardown

Run by `init` when v0.1 artifacts are detected. Reported line by line:

- Remove the `UserPromptSubmit` hook entry from `.claude/settings.json`. If that leaves
  `hooks` empty, remove the key. Leave every other setting untouched.
- Delete `.hydra/curator-reminder.sh`.
- Delete the skills symlink farms `.claude/skills/` and `.agents/skills/` — including
  dangling links. Only remove the directory itself if it is empty afterward, so
  hand-authored skills are never destroyed.
- Strip the curator block from `CLAUDE.md` / `AGENTS.md`.
- Delete `.hydra/curator.log` and `.hydra/config`.
- **Leave `.hydra/skills/` on disk**, and say so: "kept .hydra/skills/ (N skills) — salvage
  or delete by hand". hydra never deletes authored content.

This repository is itself in the broken v0.1 state: `.claude/skills/` and `.agents/skills/`
hold three symlinks into a `../../skills/` directory that no longer exists. Teardown must
handle dangling symlinks specifically — `os.Stat` fails on them; `os.Lstat` is required.

## `doctor` checks

| Check | Severity |
|---|---|
| Rules directory exists | error |
| Every rule file's frontmatter parses | error |
| Every rule has a matcher or `always: true` | error |
| No duplicate rule filenames | error |
| `index.md` matches a fresh render | warning (stale — run `sync`) |
| Block present and current in each detected target | warning |
| At least one target detected | warning |
| No v0.1 wreckage (hook, symlinks, curator block, `curator.log`) | warning (run `init`) |

`--json` emits the same structure as v0.1's doctor: a list of checks with name, status,
and detail.

## Code layout

**New:**

- `rule.go` — the `Rule` struct, frontmatter parse and render, `LoadAll(dir)`.
- `render.go` — index table and block body from a `[]Rule` plus a `Scope`.
- `block.go` — sentinel splice into a target file. The one piece worth lifting from Boost.
- `detect.go` — harness instruction file detection per scope.
- `add.go` — the filing heuristic.
- `teardown.go` — v0.1 uninstall.

**Rewritten:** `scope.go` (rules dir, detected targets, absolute-vs-relative rendering),
`init.go`, `sync.go`, `new.go`, `list.go`, `doctor.go`, `main.go`.

**Deleted:** `log.go`, `log_test.go`, `config.go`, `assets/config`,
`assets/curator-block.md`, `assets/curator-reminder.sh`, `assets/skill-curator/`, and all
symlink logic in `sync.go`.

**Also updated:** this repo's own `CLAUDE.md` and `AGENTS.md` (mirrors — both must change
in the same commit), `README.md`, and the `.gitignore` entries for the symlink farms.

Command functions keep the v0.1 shape: a resolved `Scope` plus an `io.Writer`, with cobra
as the parsing layer only. That is what makes them unit-testable and it survives the pivot.

## Dependencies

Adds `gopkg.in/yaml.v3`. Frontmatter is user-authored, so both `paths: ["a", "b"]` and
block-list style must parse; a hand-rolled subset would break on whichever style was not
anticipated. The `CLAUDE.md` convention is amended from "stdlib + cobra only" to
"stdlib + cobra + yaml".

## Testing

Existing temp-dir style, no network. Coverage per unit:

- `rule.go` — frontmatter round-trip, both list styles, missing keys, malformed YAML,
  title from heading vs filename.
- `render.go` — absolute vs relative rendering, `always` exclusion from the table, empty
  library, matcher joining.
- `block.go` — insert into empty file, insert into file with existing content, replace in
  place, idempotence across repeated syncs, surrounding content preserved.
- `detect.go` — each scope's candidates, none found, several found.
- `add.go` — new area file, append to existing area, slug collision, matcher union.
- `teardown.go` — dangling symlinks, hook removal preserving other settings, `.hydra/skills/`
  survival.
- `doctor.go` — each check firing and passing.
- End-to-end: `init` → `add` → `sync` → `doctor` in a temp project, and the same with
  `--global` against a temp home.

## Non-goals

- MCP, in any version.
- Merging or coordinating the two scopes.
- Skills, symlinks, hooks, or anything that runs on every prompt.
- Detecting the project's language or framework to seed starter rules.
- Migrating v0.1 skills into rules.

## Version

v0.2.0. `VERSION` updated; released by tagging as before.
