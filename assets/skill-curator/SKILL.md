---
name: skill-curator
description: Use at the start of any real, non-trivial task to curate the skill library before doing the work — scan existing skills, build a missing reusable skill, update a demonstrably weak one, or just use what exists. Skip for trivial or conversational prompts.
---

# Skill Curator

A self-improving skill builder/curator. Before doing real work, make sure the right
skill exists and is good — then do the work.

## When this runs

A `UserPromptSubmit` hook nudges you toward this skill on every prompt. **You decide
whether to engage:**

- **Engage** when the prompt is a real task: building, implementing, fixing, automating,
  documenting, or applying a multi-step workflow.
- **Skip** for trivial or conversational prompts: greetings, thanks, quick factual
  questions, one-line lookups, "what does this do", clarifications. Just answer.

If you skip, do nothing further here — answer the user normally.

## The loop

```
scan → decide → (build | update | use | inline) → sync → log → do the task
```

### 1. Scan
Look at the skills already listed in your injected skills metadata. Identify any whose
description plausibly covers the current task.

### 2. Decide (the bar)

| Situation | Action |
|---|---|
| A skill covers the task well | **Use it.** Invoke it and proceed. |
| No skill matches, AND the task is a *plausibly recurring* workflow AND *non-trivial* | **Build** a new skill, then do the task. |
| A skill matches but is *demonstrably weak* for this task (missing a step, stale, or wrong) | **Update** it, then do the task. |
| Task is trivial, one-off, or unlikely to recur | **Do it inline.** Create nothing. |

Guard against sprawl: when unsure whether something is reusable, do it inline. The bar
for creating is "I would plausibly want this again." The bar for updating is "this skill
concretely failed to cover what the task needed."

### 3. Build / Update
- **New skill:** run `hydra new <name>` to scaffold `.hydra/skills/<name>/SKILL.md`, then
  fill in the frontmatter `description` (say WHEN to use it) and the body. If
  `skill-creator` is available, use it to author the content.
- **Existing skill:** edit the source file in `.hydra/skills/<name>/SKILL.md`. If
  `superpowers:writing-skills` is available, use it.
- Keep skills focused, imperative, and lean. One skill = one coherent capability.

### 4. Sync
After creating or editing a skill, expose it to every runtime:

```bash
hydra sync
```

This symlinks `.hydra/skills/*` into `.claude/skills/` and `.agents/skills/`. A newly
created skill may not appear in the metadata list until the next session, but you can
still invoke it by name once synced.

### 5. Log
Record what you did and why:

```bash
hydra log CREATE <skill-name> "one-line reason"
hydra log UPDATE <skill-name> "what gap was closed"
```

## Hard rules

- **Never auto-commit or push.** Write and sync skill files freely, but committing to git
  requires asking the user first.
- **Source of truth is `.hydra/skills/`.** Never edit the symlinked copies in
  `.claude/skills/` or `.agents/skills/` directly — edit `.hydra/skills/<name>/` and
  re-run `hydra sync`.
- **Do the task.** Curating the library is the setup, not the deliverable.
