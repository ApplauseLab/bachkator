#!/bin/sh
set -eu

feature_number="$1"
slug="$2"

branch="bach/atelier-context-speed-$feature_number-$slug"
log_dir=".bach/atelier-context-speed/merge-runs"
log_file="$log_dir/feature-$feature_number-$slug.log"
current_log="$log_dir/feature-$feature_number-$slug.current.log"
checkpoint_file=".bach/atelier-context-speed/sessions/merge-agent-$feature_number-$slug.checkpoint"
prompt_file=".bach/atelier-context-speed/sessions/merge-agent-$feature_number-$slug.prompt.md"
completed="ATELIER_MERGE_${feature_number}_COMPLETED"
failed="ATELIER_MERGE_${feature_number}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are merge-agent-$feature_number for Bachkator's Atelier context-speed program.

Branch to merge: $branch.
Checkpoint file: $checkpoint_file.

Rules:
- Work in this checkout; this is a serialized merge lane.
- This invocation may resume an OpenCode session. Inspect the checkpoint file, git status, and whether $branch is already merged before changing files.
- Update the checkpoint file after each milestone: status inspected, branch verified, merge started, conflicts resolved, affected targets inspected, tests passed, blocked, or complete.
- Before merging, inspect git status and stop if unrelated local changes would be touched.
- Verify the branch exists before merging.
- Merge with a normal non-fast-forward merge unless the branch is already merged.
- If conflicts occur, resolve them carefully and preserve existing target fields and runner behavior.
- After every merge or conflict resolution, run: go run ./cmd/bach affected
- Then run: go run ./cmd/bach run shell/test
- Leave the worktree clean except for the merge commit created by this operation.
- When and only when the merge and tests are complete, write final readiness marker exactly: $completed
- If blocked, conflicts cannot be resolved, the branch is missing, or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! OPENCODE_RUN_FLAGS="--dangerously-skip-permissions" sh examples/plan-agents/scripts/run-opencode-session.sh "atelier-merge-$feature_number" "$slug" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
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
