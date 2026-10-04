# hydra for Claude Code

A Claude Code mod (a plugin of TypeScript function hooks) that does the mechanical half of
hydra's routing in code, for Claude Code only. The managed blocks in `CLAUDE.md` and
`AGENTS.md` stay exactly as they are — Codex and other harnesses still route by
instruction — and a session without the mod behaves as it always did.

Every decision is the hydra CLI's: the mod calls `hydra ability match`, `hydra match`, and
the doctors, and reimplements none of their logic. When hydra is missing, slow, or too old,
each hook passes its event through untouched.

## What it does

**Routes abilities.** When a prompt you type is exactly an ability's name or one of its
triggers (`Primetime!`), or `$ability <name>`, the mod attaches that ability's
`ABILITY.md` to the prompt as context the model reads and you never see, and toasts
`⚡ ability: <name>`. Your message is left as typed. Several abilities sharing a trigger are
listed for the model to choose between; a bare `$ability` is left to the router skill's
picker. Prompts too long to be a whole trigger never cost a `hydra` process.

**Delivers rules.** Every Read, Edit, Write, NotebookEdit, and Bash call is checked with
`hydra match` against rule `paths:` and `commands:`; matching rules are named on the status
line. What happens next is `ruleMode`:

| Mode | Read | Edit / Write / NotebookEdit / Bash |
|---|---|---|
| `off` | — | — |
| `status` | status line | status line |
| `inject` | rule text attached to the result | rule text attached to the result |
| `enforce` (default) | rule text attached to the result | **held once**, the rule text as the reason; the retry goes through |

A rule is delivered at most once per loop (the main conversation, and each subagent, which
starts with fresh context), and again after a compaction. A model that reads the rule file
itself counts as delivered. A hold is a delivery, not a policy: the retry is never held.
Parallel calls in the step that was held are held too, pointing at the sibling result that
carries the text. Holds are status-line only — no toast.

Calls on `.env*` files or `~/.secrets` are **never held**. The warden mod denies those
outright, so a hold would only buy a retry for warden to deny; the rule still shows on the
status line and rides any result that is not denied.

`triggers:` describe situations and stay with the model, as do the "grep the rules
directory" step and plan mode — the instructions that cover them are unchanged. In `inject`
and `enforce`, one static system-prompt section tells the model what a `<hydra-rule>`
block is.

**Runs the doctors.** At session start, after an edit under a hydra library or any `hydra`
command, and on a prompt when the last run is older than `doctorIntervalSec`, the mod runs
`hydra doctor` (project, only where `.hydra/rules` exists — elsewhere its advice to run
`hydra init` is wrong), `hydra doctor --global`, and `hydra ability doctor`. It toasts only
on a change — once when something drifts, naming the exact fix (`hydra sync --global`), and
once when it is clean again — and keeps the fix on the status line meanwhile. It never runs
the fix itself.

**`/hydra`** prints the hydra build, the libraries read, the doctor's verdict, the rules
fired and delivered, and the last ability routed. `/hydra doctor` re-runs the doctors;
`/hydra match <path-or-command>` shows what a file or command would fire.

## Install

Load the folder with `CLAUDE_CODE_PLUGIN_DIRS` (in the environment, or the `env` block of
`~/.claude/settings.json`), or for one session with `--plugin-dir`:

```bash
claude --plugin-dir ~/AI/hydra/integrations/claude-code
```

A session started from a terminal watches the folder and reloads the mod when a file in it
changes. Rule delivery needs a hydra build with `hydra match`; the mod checks for the
command itself rather than a version number, so a local build works before it is released.

## Options

Set under `pluginConfigs.hydra.options` in settings; each is also a `/config` row.

| Option | Default | Meaning |
|---|---|---|
| `hydraPath` | `hydra` | The binary; `~/.local/bin/hydra` is tried when it cannot start. |
| `hydraHome` | `""` | The global library for the mod's hydra processes, handed to them as `HYDRA_HOME`. Empty: hydra's own resolution, `HYDRA_HOME` else `~/.hydra`. |
| `abilityRouting` | `true` | Route abilities. |
| `ruleMode` | `enforce` | `off`, `status`, `inject`, or `enforce` (above). |
| `ruleLibraries` | `""` | More rules directories, `:`-separated, `~/` allowed — e.g. `~/AI/dotfiles/rules`. The project and global hydra libraries are always read. |
| `maxInjectChars` | `12000` | A longer rule or ability is delivered as a pointer to its file. |
| `doctorIntervalSec` | `300` | How stale a doctor run may get before a prompt re-runs it. |
| `trustProjectRules` | `true` | Off: rules from a project's own `.hydra/rules` are named, never delivered. |

The global library resolves the way hydra resolves it — the `hydraHome` option, else
`HYDRA_HOME`, else `~/.hydra` — because the mod hands the option to hydra as `HYDRA_HOME`
and takes the library's location from hydra's own answer. Set `HYDRA_HOME` where Claude
Code is started, or in the `env` block of `~/.claude/settings.json`; `/hydra` says which
source won. A global library that is not there is reported as one ("global library not
found at …"), and its doctors are not run: their advice would be `hydra init`, which after
a relocate scaffolds an empty library at the old place.

## Developing

Everything that touches `$` or `on` lives in `hooks/register.ts`: the engine follows `$`
only into functions declared at the top level of the hooks module, so a helper in another
file that takes `$` stops the module loading. `hooks/logic.ts` is pure and holds every
decision. Session state that must survive a hot reload is in `$.state`, declared in
`types/index.d.ts`.

```bash
claude plugin validate integrations/claude-code
claude plugin test integrations/claude-code
```

`tsc` checks the mod against the declarations Claude Code writes, with the `tsconfig.json`
from the header of `claude-code.d.ts`; Claude Code also lays `.claude-plugin/types/` and a
`tsconfig.json` beside the mod whenever it loads it from disk, which is why both are
gitignored here.

The design and the reasoning behind each choice:
`docs/superpowers/specs/2026-10-04-hydra-claude-mod-design.md`.
