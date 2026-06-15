#!/bin/sh
set -eu

phase_slug="$1"
branch="$2"

OPENCODE_RUN_FLAGS="${OPENCODE_RUN_FLAGS:---dangerously-skip-permissions}" \
  sh scripts/agents/run-merge-agent.sh \
  --role "overnight-merge-$phase_slug" \
  --stable-name "$branch" \
  --branch "$branch" \
  --source ".bach/overnight-clones/$phase_slug" \
  --integration-branch "bach/overnight-$phase_slug" \
  --log-dir ".bach/overnight-merge-runs" \
  --checkpoint ".bach/opencode-sessions/overnight-merge-$phase_slug.checkpoint" \
  --prompt-file ".bach/opencode-sessions/overnight-merge-$phase_slug.prompt.md" \
  --extra-rule "Do not run a full gate here unless conflict resolution makes it necessary; the final operator or automation can run the full gate after all merges."
