#!/bin/sh
set -eu

phase_number="$1"
phase_slug="$2"
branch="$3"

repo_root="$(pwd)"

OPENCODE_RUN_FLAGS="${OPENCODE_RUN_FLAGS:---dangerously-skip-permissions}" \
  sh scripts/agents/run-agent.sh \
  --role "overnight-agent-$phase_slug" \
  --stable-name "$phase_slug" \
  --plan "$repo_root/plans/phase-$phase_number-$phase_slug.md" \
  --branch "$branch" \
  --workspace ".bach/overnight-clones/$phase_slug" \
  --workspace-mode "clone" \
  --log-dir ".bach/overnight-agent-runs" \
  --checkpoint ".bach/opencode-sessions/overnight-$phase_slug.checkpoint" \
  --prompt-file ".bach/opencode-sessions/overnight-$phase_slug.prompt.md" \
  --gate "go run ./cmd/bach run --log-only --force group/gate"
