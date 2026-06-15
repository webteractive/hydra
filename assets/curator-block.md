<!-- hydra:curator:start -->
## Skill curation (hydra)

This project uses **hydra** to curate a library of reusable skills under `.hydra/skills/`.
Before any real, non-trivial task, run the curator loop (the `skill-curator` skill nudges
you via a `UserPromptSubmit` hook):

`scan → decide → (build | update | use | inline) → sync → log → do the task`

- Build a new skill only when the task is *plausibly recurring* and *non-trivial*; when
  unsure whether it is reusable, do it inline and create nothing.
- Scaffold with `hydra new <name>`, expose with `hydra sync`, record with
  `hydra log <CREATE|UPDATE|RENAME> <name> "reason"`, check health with `hydra doctor`.
- Source of truth is `.hydra/skills/<name>/SKILL.md`. Never edit the symlinked copies in
  `.claude/skills/` or `.agents/skills/`.
- Never auto-commit; the git diff is the review gate.
<!-- hydra:curator:end -->
