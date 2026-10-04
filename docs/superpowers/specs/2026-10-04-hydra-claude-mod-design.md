# hydra for Claude Code — a mod that makes routing deterministic

**Date:** 2026-10-04
**Status:** approved 2026-10-04 and implemented: `hydra match`, doctor `fix`/`initialized`, and `integrations/claude-code/`. The answers to the open questions are recorded under "Decisions" below.
**Breaking:** no. The mod is additive; every harness keeps the instruction-based routing it
has today, and a session without the mod behaves exactly as it does now.

## Summary

hydra routes by instruction: the managed blocks tell the agent to check a table, match a
trigger or a glob, and open the file. That is portable, which is the point, but it leaves
the matching to the model, which can miss it. Claude Code now loads **mods**: plugins of
TypeScript function hooks that run inside the session (`prompt.submit`, `tool.call`,
`$.ui.status`, `$.process.run`, ...). A mod can do the deterministic half of hydra's routing
in code, for Claude Code only, while leaving the instructions in place for everyone else.

The mod does three things:

1. **Ability router.** Runs `hydra ability match` on every prompt the user submits. On an
   exact name or trigger match it attaches the ability to the prompt as hidden context and
   toasts `⚡ ability: <name>`. It also parses `$ability <name>` itself.
2. **Rules fired.** Matches every Read, Edit, Write, and Bash call against rule `paths:` and
   `commands:`. It lists the matching rules on the status line and puts each rule's body in
   front of the model the first time it applies in a loop. This needs a new
   `hydra match` subcommand.
3. **Live doctor.** Runs the doctors at session start and again after anything under a
   hydra library changes. It toasts once when the library drifts, naming the exact sync
   command.

The source lives in this repository under `integrations/claude-code/`. It is published
through a plugin marketplace manifest at the repo root and installed with
`claude plugin install`. hydra itself never edits Claude Code's plugin configuration.

## Motivation

- **Trigger precision is wasted on a model that has to notice it.** `hydra ability match`
  already settles invocation offline in about 13 ms, with the same normalization the
  managed block describes. Today the agent re-derives that result by reading a table, and
  it can get it wrong. The mod hands the agent hydra's answer instead.
- **"You MUST read the rule first" is the most-skipped instruction in the setup.** An
  indexed rule costs nothing until it is opened, but only if it actually gets opened.
  `paths:` and `commands:` are mechanical matchers, so code can check them on every tool
  call. The model only needs to judge `triggers:`.
- **Drift is silent.** A rule or `ABILITY.md` edited by hand without `hydra sync` leaves a
  stale block that `doctor` would flag, but nobody runs `doctor` until something already
  looks wrong.

## Goals

- Deterministic ability invocation in Claude Code on an exact name, trigger, or
  `$ability <name>` match, without changing what the user sees as their message.
- Path and command rules delivered before the model acts on them, at a context cost no
  higher than a compliant agent reading the file itself.
- Drift noticed within one prompt of an in-session edit, and within a few minutes of an
  edit made outside the session.
- **Never block or slow the user when hydra is missing, old, or broken.** Every failure
  falls back to the behavior without the mod.
- Both rule libraries handled: hydra's (`.hydra/rules/`, `$HYDRA_HOME/rules/`) and the
  dotfiles library (`~/AI/dotfiles/rules/`, with `when:`/`title:` frontmatter).

## Non-goals

- **Replacing the managed blocks or the dotfiles rules block.** Codex and cursor-agent read
  the same files, and `rules/README.md` in dotfiles deliberately routes by instruction for
  portability. The mod adds to that routing and removes nothing.
- **Matching `triggers:` / `when:` situations.** Those are prose and need judgment. They
  stay with the model, as do the "grep the rules directory" catch-all and the "before you
  enter plan mode" clause, which have no tool call to hook.
- **Policy enforcement.** The mod holds a call once so that a rule gets read. It is not a
  deny list. Things that must never happen (`migrate:fresh`, reading `~/.secrets`) belong
  to always-on rules and to guards like the warden mod, which this mod composes with.
- **Writing.** The mod never runs `hydra sync`, `init`, `add`, `new`, or `relocate`. It
  names the command. The user or the model runs it.
- **Codex parity.** `hydra match` makes a Codex hook possible later. Building one is out of
  scope here.
- **MCP.** The mod registers no tools, which keeps hydra's "no MCP" convention intact.

## Architecture at a glance

```
             prompt.submit ──► hydra ability match --json ──► e.context += ability   ──► toast
                                                                                         
 tool.call (Read/Edit/Write/  ──► hydra match --path|--command --json ──► status line
            NotebookEdit/Bash)                                    └──► first time per loop:
                                                                        Read  → result.context += rule body
                                                                        write → { deny: rule body } once
                                                                        
 session.start / edits under ──► hydra doctor --json (project, global) ──► drift? toast + status
 a hydra library / stale timer    hydra ability doctor --json
```

Every hydra call goes through one helper (`hooks/hydra.ts`). It runs
`$.process.run([hydraPath, ...args], { cwd: sessionRoot, timeoutMs })`, parses stdout as
JSON **whatever the exit code** (`match` exits 1 on no match), and maps a rejection (binary
missing, timeout) or a parse failure to `undefined`. Callers treat `undefined` as "no
opinion" and pass through.

At `session.start` the helper probes `hydra --version` once and records:

- `available` — false if the binary cannot start (both `hydraPath` and
  `$HOME/.local/bin/hydra` fail). The desktop app's `PATH` may not include `~/.local/bin`.
- `version` — gates the parts that need newer hydra. Feature 1 and feature 3 work on 0.2.7
  today. Feature 2 needs `hydra match` (proposed for 0.3.0).
- `globalHome` — taken from `hydra doctor --global --json` → `home`, so the mod never
  reimplements `--hydra-home` / `$HYDRA_HOME` / `~/.hydra` resolution.

The probe is fired with `$.clock.after(0, …)` and not awaited, because `session.start` is
awaited before the first prompt. Until it lands, hooks pass through.

## Feature 1 — deterministic ability router

### Events

| Event | Matcher | Role |
|---|---|---|
| `prompt.submit` | none (filtered in code) | resolve, attach context, toast |
| `skill.prompt` | `{ skill: 'ability' }` | optional, phase 6: inline the body when the router skill expands |

### Flow

1. **Origin filter.** Route only `e.origin.kind` of `composer`, `bridge`, or `sdk`.
   Task notifications, scheduled triggers, and peer or channel deliveries pass through
   untouched. A notification whose text happens to equal a trigger must not load a
   workflow.
2. **`$ability` syntax.** Matched first, in code:
   `^\s*\$ability(?:\s+([a-z0-9][a-z0-9-]*))?(?:\s+([\s\S]*))?$`
   - With a name, validate it with `hydra ability match "<name>" --json`. Only a
     `kind: "name"` result counts, which reuses hydra's normalization. Any text after the
     name is the task and stays in the prompt.
   - With a name that doesn't exist, toast `hydra: no ability named <name>` and attach a
     context block listing the exact names (from a cached `hydra ability list --json`), so
     the model reports them the way the router skill would.
   - A bare `$ability` passes through to the router skill, which owns the
     `AskUserQuestion` picker. A mod-native picker is open question 10.
3. **Length pre-filter.** A trigger or name fires only when it is the *whole* request,
   apart from filler. The mod caches `hydra ability list --json` and computes a
   conservative bound: the longest name or trigger in tokens, plus 8 tokens of filler. A
   prompt with more tokens than that, or containing a newline, skips the spawn. This keeps
   long prompts at zero cost. The bound is only an optimization: hydra still decides every
   prompt that reaches it, so the two cannot drift on semantics.
4. **Match.** Run `hydra ability match <text> --json` with `timeoutMs: 750`.
   - `matches` empty or null: pass through.
   - One match: attach the ability.
   - Several matches: attach a candidate list with descriptions, telling the model to pick
     by description and name its pick (the contract from the managed block).
5. **Attach.** Call `next({ ...e, context: [...(e.context ?? []), block] })`, then toast
   `⚡ ability: <name>`, or `⚡ abilities: a | b` for several matches. Record
   `lastAbility` in `$.state` for `/hydra`.

### Choosing the injection channel

| Option | Visible to user | Prompt cache | Verdict |
|---|---|---|---|
| Rewrite `text` | **Yes.** "the user message on screen follows" | unaffected | Rejected: puts machine text in the user's own message and transcript. |
| `prompt.compose` section | no | **Busts it.** The system prompt precedes every message, so a per-prompt change invalidates the whole cached prefix. | Rejected for per-prompt data. Used once, statically, for feature 2. |
| `prompt.context` block | no | Set once per conversation | Rejected: it fires only for the first message. |
| `$.session.append` meta row | no | fine | Workable, but ordering relative to the prompt row is subtle. |
| **`prompt.submit` `context`** | **no** ("never shown the user") | fine: one block after the prompt, this turn only | **Chosen.** Built for exactly this purpose. |

### The attached block

When there is a single match, the block inlines the body, because ABILITY.md files are
0.5–7 KB. That saves a Read round trip and removes the chance the model skips the read.
The block also carries the path, so relative `references/` and `scripts/` still resolve.

```text
<hydra-ability name="prepare-for-production" matched-by="trigger: primetime"
               path="/Users/x/.hydra/abilities/prepare-for-production/ABILITY.md">
hydra matched this request to the ability above deterministically. This is an explicit
invocation. The complete ABILITY.md follows and counts as read: do not open it again.
Resolve relative references from its directory.

…body, frontmatter stripped…
</hydra-ability>
```

A body larger than `maxInjectChars` (default 12,000) is replaced with an instruction to
read the file at the path. Several candidates are never inlined. Only their names and
descriptions are attached.

### Latency budget

| Path | Cost |
|---|---|
| Long, multi-line, or non-user prompt | ~0 ms, no spawn |
| Short prompt | one spawn, ~13 ms measured on 0.2.7 (`hydra ability match primetime`) |
| Hard ceiling | 750 ms `timeoutMs`, then pass through. The engine's hook budget is 10 s. |
| Body read on match | one `$.fs.read`, under 1 ms |

### Failure modes

| Failure | Behavior |
|---|---|
| hydra missing or old | Pass through. One `$.ui.log` line per session. |
| Spawn timeout or crash | Pass through for this prompt. |
| `ABILITY.md` unreadable at the reported path | Attach the pointer form. The model's Read then surfaces the error. |
| Hook throws | The engine skips it ("a broken plugin never blocks a prompt"). |

## Feature 2 — rules fired

### Rule sources

| Library | Dialect | How the mod reads it |
|---|---|---|
| Project `.hydra/rules/` (session root) | hydra: `paths`/`commands`/`triggers`/`always` | `hydra match` (implicit) |
| Global `$HYDRA_HOME/rules/` | hydra | `hydra match` (implicit) |
| Dotfiles `~/AI/dotfiles/rules/` | dotfiles: `title`/`when`/`paths`/`commands`/`always` | `hydra match --library <dir>`, from the mod's `ruleLibraries` option |

**Decision: hydra does the matching for every library, and the mod parses no frontmatter.**
The dotfiles dialect already parses as a hydra rule: yaml.v3 ignores `when:`/`title:`, and
`paths`/`commands`/`always` have the same keys. Matching therefore needs no dialect
support. Two small tolerances make the output right (see "hydra CLI changes"):

- `title:` read as the title.
- A library's `README.md` skipped when it has no frontmatter.

The alternatives were weighed and rejected:

- **Glob and command matching in TypeScript, from a cached `hydra list --json`.** This
  saves a spawn per call, but it creates a second glob implementation that will drift
  from the Go one. It also does nothing for the "why didn't my rule fire" question that
  `hydra match` answers for humans and other harnesses.
- **The mod parsing dotfiles rules itself.** This needs a YAML subset parser in an
  environment with no Node and no npm. It would also make the mod the only place the
  dotfiles dialect is matched.

`always: true` rules are excluded from the results. They are already inlined in standing
context.

### Events

| Event | Matcher | Role |
|---|---|---|
| `tool.call` | `{ tool: 'Read' }`, `'Edit'`, `'Write'`, `'NotebookEdit'`, `'Bash'` | match, hold or annotate, status |
| `turn.step` (async generator, pass-through) | none | count steps per loop so parallel calls are held correctly (see below) |
| `prompt.submit` | none | reset the per-turn "fired" list for the status line |
| `session.compact`, `session.end` (`reason: 'clear'`) | none | reset per-loop `loaded` sets: injected bodies may be summarized away |
| `prompt.compose` | none | append one **static** section, `hydra:rules-delivery` (below) |

The subject of each call is `e.file_path` (Read, Edit, Write), `e.notebook_path`
(NotebookEdit), or `e.command` (Bash). Glob and Grep are not matched in v1. Their `path`
is a search root, not a file being touched.

### Matching

`hydra match --path <abs> --json` or `hydra match --command <cmd> --json` runs with
`cwd = $.session.root()`, which is where project scope resolves. The mod memoizes by
subject for the session, and clears the memo whenever feature 3 sees a library change.
Read bursts then cost one spawn per distinct file, about 15 ms each.

### Modes (`ruleMode` option)

| Mode | Read | Edit / Write / NotebookEdit / Bash |
|---|---|---|
| `off` | — | — |
| `status` | status line only | status line only |
| `inject` | status + body attached to the **result** (`context`) | status + body attached to the **result** |
| `enforce` (proposed default) | status + body attached to the result | status + **held once**: `{ deny: <body> }`, then let through |

**Weighing context cost against reliability.**

- *Cost.* Rule bodies run 0.7–11 KB, roughly 200–3,000 tokens. Each is delivered at most
  once per loop. A compliant agent pays exactly that by reading the file, as CLAUDE.md
  already requires. So for a rule that applies, the mod costs **nothing extra**, and it
  skips the `grep`/Read round trips. The real risk is duplication: the model reads the
  file anyway because CLAUDE.md says to. Three mitigations cover it:
  - The block says the rule "counts as read".
  - The static compose section explains the delivery.
  - A Read of a rule file by the model marks that rule `loaded`, so it is never injected
    after a manual read.
- *Reliability.* `inject` delivers the rule **after** the action. For a Read that is ideal,
  since the model has the rule before it edits anything. For `gh release create` or a
  `git tag` it is too late. `enforce` puts the rule in front of the first mutating call:
  the model gets the body as the call's error result, and re-issues the call (adjusted if
  needed). The cost is one extra model round trip per rule per loop, which is rare and
  bounded.
- *Recommendation.* Default to `enforce`. It is the only mode that implements what the
  instruction says ("read the rule **before** you act"). `inject` is the fallback if
  holding calls turns out to be annoying in practice (open question 1).

**Replacing the CLAUDE.md instruction with code: not in v1.** Three reasons:

- The instruction also covers `triggers`/`when`, the grep catch-all, and plan mode.
  Code can see none of those.
- The same text drives Codex and cursor-agent.
- `prompt.context` could rewrite the `claudeMd` block for Claude only, but a rewritten
  block makes `instructionFiles` unknown to every hook beneath. It also forks the routing
  text the user maintains in two generators.

Instead, the mod appends one static `prompt.compose` section (stable text, so it is
cache-safe):

```text
hydra mod: path and command rule matches are detected automatically. When a
<hydra-rule> block appears (as a tool result's note, or as the reason a call was held),
it is the rule's complete text and counts as read. You still decide trigger/situation
matches yourself, and still grep the rules directories as instructed.
```

### Held-call semantics (`enforce`)

State lives in `$.state` (it survives hot reloads), keyed by loop: `agentId ?? 'main'`.
Subagents have fresh context, so each loop gets its own record.

- `loaded: Record<loop, string[]>` — rule ids (`<library>:<name>`) the loop has seen.
- `held: Record<loop, Record<ruleId, stepIndex>>` — the step in which the rule was
  delivered by a deny.
- `step: Record<loop, { turnId, index }>` — updated by the `turn.step` hook.

On a mutating call with matched rules R:

1. Drop rules in `loaded[loop]`.
2. For each remaining rule: if `held[loop][r]` was set in an **earlier** step, the model
   has now seen it, so move it to `loaded` and let it through. If it was set in the
   **same** step, this is a parallel sibling call: deny it again with a short reason
   ("held: rule X applies, its text is on a sibling result").
3. Otherwise, read the bodies (one `$.fs.read` each, frontmatter stripped, capped at
   `maxInjectChars`). Then return:

   ```text
   { deny: "<hydra-rule name=… source=… path=…>…body…</hydra-rule>
            This call was held once because the rule above applies to <subject> and had
            not been read in this session. It now counts as read. Re-issue the call,
            adjusted if the rule requires it." }
   ```

   Set `held[loop][r] = currentStep` and toast `⚖ rule: <name>`. This is the only toast
   feature 2 makes, and it fires at most once per rule per loop.

A rule is never held twice in one loop. A model that ignores it and re-issues the call
gets through: this mode makes sure the rule gets read, it does not enforce compliance.

On Read, and in `inject` mode, the mod calls `next(e)` and returns
`{ ...result, context: [...(result.context ?? []), block] }`, then marks the rule `loaded`.
A Read whose `file_path` is itself a rule file marks that rule `loaded` and injects
nothing.

Composition with other mods (the warden mod, for one): this mod either calls `next(e)` or
returns a deny. It never rewrites arguments, so a guard above or below sees the call
unchanged.

### Status line

Feature 2 and feature 3 share the plugin's one status line, composed in
`hooks/status.ts`:

```
hydra ▸ rules: warden, php-laravel        (fired this turn, newest first, ≤ 3 + "+N")
hydra ▸ rules: releases · ! hydra sync --global
hydra ▸ ! hydra ability sync               (drift only)
```

When nothing has fired this turn and there is no drift, the status is cleared
(`$.ui.status(undefined)`), so a quiet session shows nothing.

The mod deliberately draws no `AbovePrompt` band. That band is one instance shared by
every plugin, and the status line is enough here. If a band is ever added, it has to
compose with whatever the other plugins draw instead of replacing it. Concretely: return
`next(e)` when `e.props.hasSurvey` is set; otherwise
`const below = await next(e); return <Box flexDirection="column">{mine}{below}</Box>`,
and return `below` alone when there is nothing to show. That way the hydra, schedy, and
warden bands stack instead of hiding each other.

## Feature 3 — live doctor

### Which doctors run

| Doctor | Runs when | Why not always |
|---|---|---|
| `hydra doctor --json` (project) | `<root>/.hydra/rules` exists | Elsewhere it fails with "run 'hydra init'". In a Laravel project (where Boost owns rules) or in the dotfiles repo, that advice is **wrong** (`rules/hydra.md`). |
| `hydra doctor --global --json` | `<globalHome>/rules` exists | — |
| `hydra ability doctor --json` | `<globalHome>/abilities` exists | — |

### When they run

- **Session start.** Fired from `session.start` via `$.clock.after(0, …)` and never
  awaited.
- **After an in-session change.** That means a successful `tool.call` (after `next`) on
  Edit, Write, or NotebookEdit whose path is under `<root>/.hydra/`, under
  `<globalHome>/`, or under a configured `ruleLibraries` directory. It also means any Bash
  command matching `hydra ` (the user or model just ran sync, add, or new). These are
  debounced 1.5 s with `$.clock.after`, so a burst of edits runs the doctors once.
- **Changes made outside the session**, such as in an editor. On `prompt.submit`, if the
  last run is older than `doctorIntervalSec` (default 300), the doctors re-run in the
  background. There is no polling timer while the session is idle.

Every run also clears the `hydra match` memo and the cached ability list.

### Reporting

The mod compares the set of failing checks to the previous run and acts only on
**changes**:

- **New failure, warning severity:** toast
  `hydra: abilities index stale — run hydra ability sync` (`timeoutMs: 8000`). Add the
  command to the status line.
- **New failure, error severity** (a rule fails to parse, a router isn't hydra-owned):
  toast with the check's own detail. Status shows `✗ hydra doctor`.
- **All clear after a failure:** toast `hydra: in sync` once, and drop the status suffix.

Fix commands are scope-correct. The mod maps them as follows until doctor carries them
itself (CLI change 4):

| Scope | `detail` today | Command the mod shows |
|---|---|---|
| project | `run 'hydra sync'` | `hydra sync` |
| global | `run 'hydra sync'` (**missing `--global`**) | `hydra sync --global` |
| global abilities | `run 'hydra ability sync'` | `hydra ability sync` |

The mod never runs these itself. The model can be pointed at them, and the user can run
them as `! hydra sync --global`.

## `/hydra` command

Registered in `session.start` with `$.command.register({ name: 'hydra', … })` and
answered in `command.run`. It returns `{ text }`, a CommandOutput row in the transcript.

| Invocation | Output |
|---|---|
| `/hydra` | hydra version and library homes; doctor summary with fix commands; rules fired this session per loop (name, source, matched-by, delivered how); last ability routing |
| `/hydra doctor` | re-run the doctors now, print the summary |
| `/hydra match <path-or-command>` | dry run of feature 2 for one subject, which helps answer "why didn't my rule fire" |

A Pane version (a live doctor and fired-rules view) is phase 6. It is not needed for v1.

## Options (`userConfig`)

| Field | Type | Default | Meaning |
|---|---|---|---|
| `hydraPath` | string | `hydra` | Binary. Falls back to `$HOME/.local/bin/hydra`. |
| `abilityRouting` | boolean | `true` | Feature 1 on/off. |
| `ruleMode` | string, options `off`/`status`/`inject`/`enforce` | `enforce` | Feature 2 (above). |
| `ruleLibraries` | string | `""` | Extra rule libraries, `:`-separated absolute paths, passed as `--library`. This user sets `~/AI/dotfiles/rules`. |
| `maxInjectChars` | number | `12000` | Larger bodies are delivered as a pointer instead. |
| `doctorIntervalSec` | number | `300` | Staleness bound for re-running the doctors on a prompt. |

These live in settings under `pluginConfigs.hydra.options`. Each is a `/config` row, and
changing one reloads the module.

## hydra CLI changes

Features 1 and 3 work against hydra 0.2.7. Feature 2 needs change 1. The rest are
correctness and ergonomics fixes, targeted at **v0.3.0**.

1. **`hydra match` — new subcommand.** It is top-level for consistency with
   `hydra list`/`add`/`new`, where rules are the top-level noun and abilities live under
   `ability`. The lead suggested `hydra rules match`; see open question 6.

   ```
   hydra match [--path <p>]... [--command <c>]... [--library <dir>]...
               [--no-project] [--no-global] [--include-always] [--json]
   ```

   - Reads the project library (cwd), the global library (`$HYDRA_HOME`), and each
     `--library` directory, read-only. This is the first command that reads more than one
     scope. It reports per library and **merges nothing**, so the "scopes never read each
     other" invariant holds for every command that writes.
   - **Tolerant loading.** A library that fails to parse is reported under `errors` and
     the others still match. Today `LoadRules` fails the whole library on one bad file.
   - **Exit codes:** 0 when something matched, 1 when nothing did (mirroring
     `ability match`), 2 on usage error. JSON is printed in all three cases.
   - **JSON shape:**

     ```json
     {
       "paths": ["/abs/app/Jobs/Foo.php"], "commands": [],
       "matches": [{
         "rule": "queue", "title": "Queue conventions",
         "file": "/abs/.hydra/rules/queue.md",
         "library": { "kind": "project", "dir": "/abs/.hydra/rules" },
         "always": false,
         "matched": [{ "kind": "path", "pattern": "app/Jobs/**", "subject": "/abs/app/Jobs/Foo.php" }]
       }],
       "errors": []
     }
     ```

   - The text output says which pattern matched which subject, for humans debugging a
     rule that doesn't fire.

2. **Write down and implement matcher semantics** (new `match.go`, stdlib only, no
   doublestar dependency).
   - **`paths`.** `/`-separated globs: `*` and `?` within a segment, `**` across zero or
     more segments.
     - A project rule matches against the path relative to the project root.
     - A global or `--library` rule matches against both that relative path (when the file
       is under the root) and the absolute path, and either hit counts. So
       `**/.hydra/**` matches `/Users/x/.hydra/rules/a.md` from anywhere.
   - **`commands`.** Boundary-anchored substring match on the command line with
     whitespace collapsed. The pattern must start and end at a non-`[A-Za-z0-9_-]`
     boundary or at the line's edge. Trailing spaces in a pattern (dotfiles' `"warden "`)
     are trimmed, since the boundary rule replaces them.

     This reconciles the two libraries' documented semantics: hydra's README says
     "prefixes", dotfiles says "substrings". The examples show what the rule covers:
     - `php artisan test` matches `artisan test`.
     - `./vendor/bin/pest` matches `vendor/bin/pest`.
     - `wardenx` does not match `warden`.
     - Known false positive, accepted: `echo "git tag"`.

   - The README's "How it works" gains a short "How a match is decided" subsection.

3. **Accept the dotfiles dialect when reading foreign libraries.**
   - `ParseRule` reads an optional `title:` frontmatter key, which takes precedence over
     the first H1.
   - `LoadRules` skips `README.md` when it has no frontmatter.
   - `when:` stays ignored, because `match` does not use triggers. Accepting it as an
     alias of `triggers:` is optional and only matters for `hydra list --library`.

4. **Doctor fixes.**
   - The global scope's details say `run 'hydra sync'` but need `--global`. This is a bug;
     fix the strings.
   - Add `"fix": ["hydra", "sync", "--global"]` (argv) to each check, so callers stop
     parsing prose.
   - Optionally add top-level `"initialized": false` to the project doctor when
     `.hydra/rules` is absent, so callers can tell "not a hydra project" from "broken".

5. **CLAUDE.md / AGENTS.md (mirrored).**
   - Add `integrations/claude-code/` to Layout.
   - Note that the Go dependency rule (stdlib + cobra + yaml) is unchanged and the mod is
     TypeScript with no npm dependencies.
   - Note that the mod registers no MCP tools.

No change makes `hydra init` install the mod. See "Distribution".

## File layout

```
hydra/
├── .claude-plugin/
│   └── marketplace.json            # marketplace "hydra": one entry, source "./integrations/claude-code"
└── integrations/claude-code/
    ├── .claude-plugin/plugin.json  # name "hydra", version (kept equal to VERSION), userConfig, "types"
    ├── hooks/
    │   ├── hooks.json              # { "modules": ["./register.tsx"] }
    │   ├── register.tsx            # wires events to the feature modules; JSX only for the phase-6 pane
    │   ├── hydra.ts                # runHydra(): spawn, timeout, JSON parse, availability, version gate
    │   ├── abilities.ts            # $ability parse, length bound, match, context block
    │   ├── rules.ts                # match memo, loaded/held state, inject/enforce, compose section
    │   ├── doctor.ts               # which doctors, debounce, diff, toast, fix-command mapping
    │   ├── status.ts               # the one status line
    │   └── command.ts              # /hydra
    ├── types/index.d.ts            # PluginState['hydra']: loaded, held, step, fired, doctor, lastAbility
    ├── tests/                      # *.test.ts for `claude plugin test`
    └── README.md                   # install, options, what each feature does
```

## Distribution

**Decision: ship the mod from this repository as a plugin marketplace, and install it
with Claude Code's own plugin commands. hydra does not install it.**

- **Users:**

  ```sh
  claude plugin marketplace add webteractive/hydra
  claude plugin install hydra@hydra
  ```

  This is a git-sourced marketplace, so updates arrive with `claude plugin update` and
  move with hydra's tags.
- **Author:** `claude plugin marketplace add ~/AI/hydra`. This is a folder marketplace
  with a relative entry, which the engine reads **in place**. Edits reach a session with
  `/reload-plugins`, with no reinstall. For hot-reload while building, use
  `claude --plugin-dir ~/AI/hydra/integrations/claude-code`.
- **Why not have `hydra init` write it?** Installing a plugin means editing Claude Code's
  settings (`enabledPlugins`, `extraKnownMarketplaces`). hydra's equivalent stance on
  shell profiles is "print the line, never edit it". Embedding the TypeScript in the Go
  binary would also tie mod fixes to binary releases.

  At most, `hydra ability init` prints a one-line hint when `~/.claude` exists. That is
  open question 5.
- **Version skew.** The mod reads `hydra --version` and turns off only what the installed
  binary can't do: below 0.3.0, feature 2 is disabled and `/hydra` says why.

## Test plan

**Static checks** (run locally and before every release):

- `claude plugin validate integrations/claude-code`. This checks the manifest, the hooks
  module, the events hooked, the `$` calls made, and the env names read (`HOME`).
- `tsc -p integrations/claude-code` against the declarations the engine lays into
  `.claude-plugin/types/`.

**`claude plugin test integrations/claude-code`**: the `*.test.ts` files, with hooks
beneath the plugin answering `process.run` and `fs.read` as fakes for hydra and the rule
files. Each test runs on `['terminal', 'desktop']` where UI is involved.

- *Ability router:*
  - A trigger match attaches exactly one `<hydra-ability>` context block and toasts.
  - `text` is unchanged.
  - A non-user origin is never routed.
  - A long prompt makes no spawn.
  - Several candidates are listed without bodies.
  - `$ability known` routes, keeping the trailing task text.
  - `$ability unknown` toasts and lists names.
  - A bare `$ability` passes through.
  - A spawn rejection or timeout passes through.
  - A body over the cap becomes a pointer.
- *Rules:*
  - Read of a matching path: the result gains context once, and a second Read of another
    matching file in the same loop adds nothing.
  - Edit in `enforce` mode: the first call is denied with the body, and the same call
    after a `turn.step` passes.
  - Two parallel Edits in one step are both denied, and the second's reason is short.
  - A subagent loop gets its own delivery.
  - A model Read of the rule file marks it loaded.
  - `session.compact` resets the state.
  - `always` rules never fire.
  - `ruleMode: status` never attaches or denies.
  - hydra below 0.3.0 disables the feature.
- *Doctor:*
  - No `.hydra/` means no project doctor.
  - A drift transition toasts once with `hydra sync --global` for the global scope.
  - A repeated failing run doesn't toast again.
  - Recovery toasts "in sync".
  - An edit under `<globalHome>` debounces to one run.
  - The status line composes rules and drift.
- *Command:* `/hydra`, `/hydra doctor`, and `/hydra match x` produce the expected text.

**Go (`go test -race ./...`)** for `hydra match`:

- Table-driven glob cases: `**` at zero, one, and many segments; relative and absolute;
  outside the root.
- Command boundary cases from the list above.
- A dotfiles-dialect library: `title:` is used, `when:` ignored, README skipped.
- One broken file in one library leaves the others matching.
- Exit codes 0, 1, and 2 with JSON on all three.
- The doctor `fix` field and the global detail string.

**Manual, in a session started with
`claude --debug --plugin-dir ~/AI/hydra/integrations/claude-code`:**

1. Type `Primetime!`. The toast reads `⚡ ability: prepare-for-production`, the transcript
   shows the prompt unchanged, and the model starts the workflow without reading
   ABILITY.md.
2. In this repo, ask for an edit to `main.go`. The first Edit is held with
   `cli-tools-serve-humans-and-agents-separately`, the retry succeeds, and the status
   line shows the rule.
3. Run `gh release list`. The `releases` rule is held once.
4. Hand-edit an ABILITY.md description outside Claude, wait more than
   `doctorIntervalSec`, then send a prompt. The toast names `hydra ability sync`.
5. Put `hydra` off `PATH` and restart. Everything passes through and one debug line
   explains.

## Risks

| Risk | Mitigation |
|---|---|
| The mod API is early access and moves between releases. | `claude plugin validate` in the release checklist. The README names the minimum Claude Code version. Types are regenerated, never hand-edited. |
| Duplicate delivery: the model reads the rule anyway because CLAUDE.md says so. | "Counts as read" wording, the static compose section, and Read-tracking. Measure in the manual run before defaulting `enforce`. |
| Holding calls feels like friction, or the model treats the deny as failure and gives up. | The deny text says explicitly to re-issue. Once per rule per loop. `inject` is one `/config` change away. |
| Command false positives (`echo "git tag"`). | Acceptable: the cost is one rule read. The status line makes it visible. |
| Prompt injection via a cloned repo's `.hydra/rules/`, now delivered as "rule text". | The same trust as its `CLAUDE.md`, which already loads. Blocks carry `source="project:<root>"`. Option `trustProjectRules` is open question 12. |
| Spawn cost on Read-heavy subagents. | Per-subject memo. About 15 ms per distinct file is small next to the tool itself. Batching `--path` is possible later. |
| The repo becomes polyglot, and CI may not have `claude`. | Go CI is unchanged. The plugin checks run locally and in the release checklist (open question 11). |
| `~/.hydra/rules/` holds 5 rules, though `rules/hydra.md` says keep it empty. | The mod reads it either way. Whether to move them is open question 3. |

## Open questions for the user

1. **Default `ruleMode`.** Should it be `enforce` (hold the first mutating call per rule
   per loop), `inject` (attach after the result), or `status` only? The spec recommends
   `enforce`.
2. **Dotfiles library.** Is setting `ruleLibraries` to `~/AI/dotfiles/rules` in
   `pluginConfigs` enough? Or should dotfiles rules eventually move to hydra's
   `triggers:` dialect, and should `dotfiles.sh --rules` render through hydra?
3. **`~/.hydra/rules/`.** It holds 5 rules (`releases`, `figma-plugin-*`,
   `browser-test-cleanup`, `cli-tools-serve-humans-and-agents-separately`), though
   `rules/hydra.md` says to keep it empty and put global rules in dotfiles. Should they be
   moved, or should the guidance change?
4. **CLAUDE.md wording when the mod is active.** Is the static compose section enough, or
   should the mod also shorten the "read the rule first" paragraph for Claude via
   `prompt.context`? Shortening forks the routing text, and the spec says no.
5. **Discovery.** Should `hydra ability init` / `hydra init` print a one-line
   `claude plugin install` hint when `~/.claude` exists, or stay silent?
6. **Command name.** `hydra match` (consistent with `hydra list/add/new`) or
   `hydra rules match`?
7. **Ability bodies inline.** Should a single-match ability's `ABILITY.md` be inlined,
   costing up to ~2k tokens on a match, or should the mod only point at it?
8. **`/ability <name>` via the router skill.** Should `skill.prompt` also inline the body
   there? `SkillPromptInput` carries no args, so the name would be parsed from the
   expanded text. That needs verification.
9. **Toasts.** The defaults are one toast per ability route, one per held rule, and one
   per drift transition. Are those right, or should rule holds be status-only?
10. **Bare `$ability`.** Keep the router skill's `AskUserQuestion` paging, or replace it
    in Claude with a mod-drawn picker (a Select in a pane, no 4-option paging)?
11. **CI.** Install `claude` in GitHub Actions to run `plugin validate`/`test`, or keep
    plugin checks local and in the release checklist?
12. **Untrusted project rules.** Should project-library injection be off by default
    outside repos the user owns (`trustProjectRules`)?

## Decisions

The user's answers to the open questions, 2026-10-04:

1. `ruleMode` defaults to `enforce`. All four modes ship, each with tests.
2. `ruleLibraries` carries `~/AI/dotfiles/rules`, set by the user. The dotfiles dialect
   stays as it is.
3. Moving the rules out of the global hydra library is the user's job (since done with
   `hydra relocate ~/AI/dotfiles/hydra`).
4. The static compose section only. The CLAUDE.md wording is untouched.
5. No install hint from `hydra init`.
6. `hydra match`.
7. A single match inlines its ABILITY.md body.
8. `skill.prompt` inlining is skipped for now.
9. Toast on an ability route and on drift. A rule hold is status-line only.
10. The router skill keeps its `AskUserQuestion` paging.
11. Plugin checks run locally and in the release checklist, not in CI.
12. `trustProjectRules` defaults to `true`.

Where the build departs from the design above:

- **Distribution.** The mod loads through `CLAUDE_CODE_PLUGIN_DIRS` (or `--plugin-dir`).
  There is no marketplace manifest yet.
- **Gating.** Rule delivery is gated on a capability probe, not a version number. A
  `hydra match --command=hydra-mod-probe --json` that answers with a library list also
  supplies the libraries and the global home, so a local build works before a release.
- **Module layout.**
  - Every `$`/`on` call is in `hooks/register.ts`, because the engine follows `$` only
    into functions declared at the top level of the hooks module.
  - `hooks/logic.ts` is pure.
  - There is no JSX yet, so the module is `.ts`, not `.tsx`.
- **Secret files.** A call on a `.env*` file or `~/.secrets` is never held. The warden mod
  denies those, so a hold would only buy a retry for warden to deny.

## Suggested build order

Each phase ships on its own and is useful on its own.

1. **Mod skeleton and the hydra helper.** Covers the manifest, marketplace entry, version
   probe, `runHydra`, `/hydra` showing version and homes, `validate` passing, and test
   scaffolding. This needs no hydra change.
2. **Feature 1, the ability router.** Works with hydra 0.2.7. It is the highest value per
   line of code, and it is easy to verify (`Primetime!`).
3. **Feature 3, the live doctor.** Works with 0.2.7, using the mod-side fix-command
   mapping. Adds the status line plumbing.
4. **hydra v0.3.0.** `hydra match`, matcher semantics, foreign-dialect tolerance, and the
   doctor `fix` field and global detail fix, with Go tests and README. Release.
5. **Feature 2 in `status` mode, then `inject`, then `enforce`.** Turn each on in daily
   use before making the next the default. Add the compose section with `inject`.
6. **Polish.** The `/hydra` pane, `skill.prompt` inlining, the bare-`$ability` picker
   (pending open questions 8 and 10), and removing the mod-side fix mapping once doctor
   carries `fix`.
