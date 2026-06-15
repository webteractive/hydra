#!/usr/bin/env bash
# UserPromptSubmit hook — nudges the skill-curator protocol on real tasks.
# stdout is injected into the conversation as additional context.
cat <<'EOF'
[skill-curator] If this prompt is a real, non-trivial task, invoke the skill-curator skill BEFORE doing the work: scan existing skills, then build a missing reusable skill, update a demonstrably weak one, or just use what fits. Skip silently for greetings, quick questions, and one-off/trivial requests.
EOF
exit 0
