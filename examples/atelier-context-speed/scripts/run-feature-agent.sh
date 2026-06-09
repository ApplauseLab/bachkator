#!/bin/sh
set -eu

feature_number="$1"
slug="$2"
title="$3"
acceptance="$4"

branch="bach/atelier-context-speed-$feature_number-$slug"
log_dir=".bach/atelier-context-speed/agent-runs"
log_file="$log_dir/feature-$feature_number-$slug.log"
current_log="$log_dir/feature-$feature_number-$slug.current.log"
checkpoint_file=".bach/atelier-context-speed/sessions/feature-agent-$feature_number-$slug.checkpoint"
prompt_file=".bach/atelier-context-speed/sessions/feature-agent-$feature_number-$slug.prompt.md"
completed="ATELIER_FEATURE_${feature_number}_COMPLETED"
failed="ATELIER_FEATURE_${feature_number}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are feature-agent-$feature_number for Bachkator's Atelier context-speed program.

Feature: $title.
Branch: $branch.
Source request file: atelier-context-speed-feature-requests.md.
Checkpoint file: $checkpoint_file.

Acceptance notes:
$acceptance

Rules:
- Create or reuse a separate worktree outside this checkout on branch $branch.
- Leave this main checkout untouched.
- This invocation may resume an OpenCode session. Inspect the checkpoint file and branch/worktree state before editing.
- Update the checkpoint file after each milestone: worktree ready, design inspected, edits complete, focused checks selected, focused checks passed, full test passed, committed, blocked, or complete.
- Start with: go run ./cmd/bach -list
- Before expensive work, run a dry-run for the smallest relevant target when one exists.
- After edits, run: go run ./cmd/bach affected
- Run focused tests repeatedly, then run: go run ./cmd/bach shell/test
- Commit only intended files on branch $branch.
- When and only when ready, write final readiness marker exactly: $completed
- If blocked or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! OPENCODE_RUN_FLAGS="--dangerously-skip-permissions" sh examples/plan-agents/scripts/run-opencode-session.sh "atelier-feature-$feature_number" "$slug" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
  printf '%s: opencode exited non-zero\n' "$failed" >> "$current_log"
  cat "$current_log" >> "$log_file"
  exit 1
fi

if grep -q "$failed" "$current_log"; then
  cat "$current_log" >> "$log_file"
  exit 1
fi

if ! grep -q "$completed" "$current_log"; then
  cat "$current_log" >> "$log_file"
  exit 1
fi

printf 'checkpoint.status=completed\n' >> "$checkpoint_file"
cat "$current_log" >> "$log_file"
