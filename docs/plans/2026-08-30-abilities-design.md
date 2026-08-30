# Hydra abilities

**Date:** 2026-08-30
**Status:** approved

## Summary

Hydra will add **abilities**: global, reusable workflow bundles that are discovered and
loaded on demand. The feature addresses native skill registries whose descriptions can be
truncated when many skills are installed. Hydra keeps the full catalog outside standing
agent context and lets the agent decide whether an ability is useful.

Abilities and rules deliberately share indexing and instruction-wiring machinery, but
their contracts differ:

- A rule states what an agent must or must not do.
- An ability teaches an optional workflow the agent may choose to use.

Rules retain mandatory path, command, and situation matchers. Abilities are selected by
the agent's semantic judgment or invoked explicitly through `$ability <name>`.

## Scope

Abilities are global-only in the first release. Their fixed library is
`~/.hydra/abilities/`; project-local abilities and scope precedence are non-goals.

Every harness Hydra recognizes receives the catalog-discovery instruction. Harnesses with
a native skill mechanism also receive one generated router skill named `ability`. Harness
details are isolated behind adapters so additional harnesses do not affect the ability
format.

## Ability format

Each immediate child of `~/.hydra/abilities/` is one bundle:

```text
~/.hydra/abilities/
├── index.md
├── testing-notes/
│   ├── ABILITY.md
│   ├── references/
│   ├── scripts/
│   └── assets/
└── release-review/
    └── ABILITY.md
```

`ABILITY.md` has minimal YAML frontmatter followed by authored instructions:

```markdown
---
name: testing-notes
description: Generate manual QA notes from completed code changes.
---

# Testing notes

...
```

`name` and a one-line `description` are required. The name must equal the bundle directory
name and contain only lowercase letters, digits, and hyphens. Supporting files are loaded
only when the ability directs the agent to read or execute them.

The generated `index.md` contains each ability's name, description, and absolute
`ABILITY.md` path. It contains no ability bodies and is not inlined into agent instruction
files.

## Discovery and invocation

Hydra splices a short, separately delimited abilities block into recognized global agent
instruction files. It tells the agent to:

1. Decide whether a task may benefit from an ability.
2. Search or read `~/.hydra/abilities/index.md` when appropriate.
3. Select an ability semantically; Hydra performs no ranking or matching.
4. Read the selected `ABILITY.md` completely before acting.
5. Load referenced files only as directed by that ability.

For explicit invocation, the generated native router skill handles
`$ability <name>`. It validates the name, resolves the exact bundle, reads its complete
`ABILITY.md`, and follows it. With no name, the router lists available abilities. For an
unknown name, it reports the available exact names; it does not silently fuzzy-match.

Harnesses without native skills still receive automatic catalog-discovery instructions.

## CLI

Abilities use a dedicated, intrinsically global command group:

| Command | Behavior |
| --- | --- |
| `hydra ability init` | Create the library, generate the empty index, wire global instructions, and install routers. Idempotent. |
| `hydra ability new <name>` | Create `<name>/ABILITY.md` from a scaffold, then sync. Initialize first when needed. |
| `hydra ability sync` | Validate bundles, regenerate `index.md`, refresh instruction blocks, and refresh owned routers. |
| `hydra ability list [--json]` | Print names, descriptions, and absolute paths without loading bodies. |
| `hydra ability doctor [--json]` | Diagnose malformed abilities and stale or missing generated artifacts and wiring. |

There is no `ability add`: abilities are multi-file workflows, not short notes. Remote
installation, registries, package management, and Hydra-side selection are non-goals.

Although ability subcommands are explicitly available for existing installations, normal
`hydra init` also ensures the global ability system exists. This is an intentional
one-time cross-scope effect when `hydra init` is run for a project. Repeated initialization
only refreshes Hydra-owned generated artifacts and never replaces authored ability files.

## Validation and writes

`hydra ability sync` validates the complete library before writing generated artifacts. It
reports all validation errors together and leaves the current index and harness wiring
untouched when validation fails.

Hydra never modifies authored ability content except when `ability new` creates a new
bundle. It refuses to overwrite an existing bundle.

Ability names reject path separators and traversal segments. Generated router files carry
a Hydra ownership marker. Hydra updates routers it owns and refuses to replace a
user-authored native skill named `ability`. Managed ability blocks use their own
`hydra:abilities` sentinels, independent of the rules block and surrounding content.

If writing several generated targets fails partway through, no authored data is at risk;
`hydra ability doctor` reports drift and a subsequent sync repairs it.

## Internal components

- `Ability` model and YAML parser for `ABILITY.md`.
- Global `AbilityScope` or equivalent paths resolved from the user's home directory.
- Deterministic ability catalog renderer.
- Managed abilities instruction block renderer.
- Harness adapters describing instruction targets and optional native router locations.
- Embedded router skill template shared by every compatible adapter.
- Ability lifecycle commands and structured list/doctor output.

Existing rule parsing, sentinel splicing, target detection, and doctor-reporting patterns
should be reused where their semantics match. User-facing rule and ability models remain
separate.

## Testing

Tests cover:

- Frontmatter parsing and name/directory validation.
- Deterministic index generation for empty and populated libraries.
- Automatic-discovery and explicit-router instruction rendering.
- Harness adapter detection and router destinations.
- Preservation of foreign skills, colliding user-authored routers, and authored ability
  content.
- Idempotent `hydra init` and `hydra ability init`.
- Existing-install opt-in and fresh-install bootstrap flows.
- Aggregated validation failures without generated rewrites.
- Structured list and doctor output.
- End-to-end `init -> ability new -> sync -> list -> doctor` behavior.

Repository gates remain `gofmt -l .`, `go test ./...`, `go vet ./...`,
`govulncheck ./...`, and `goreleaser check`.
