#!/bin/sh
set -eu

plan_number="$1"
branch="bach/plan-$plan_number"
log_dir=".bach/merge-runs"
log_file="$log_dir/plan-$plan_number.log"
current_log="$log_dir/plan-$plan_number.current.log"
checkpoint_file=".bach/opencode-sessions/merge-agent-$plan_number.checkpoint"
prompt_file=".bach/opencode-sessions/merge-agent-$plan_number.prompt.md"
completed="MERGE_PLAN_${plan_number}_COMPLETED"
failed="MERGE_PLAN_${plan_number}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are merge-agent-$plan_number for Bachkator.

Branch to merge: $branch.
Verification file: examples/plan-agents/Bachfile.verification.
Checkpoint file: $checkpoint_file.

Rules:
- Work in this checkout; this is the serialized merge-agent lane.
- This invocation may be a resumed OpenCode session. Before changing files, inspect the checkpoint file, git status, and whether $branch is already merged.
- Update the checkpoint file after each milestone: status inspected, branch verified, merge started, conflicts resolved, affected targets inspected, regression passed, blocked, or complete.
- Before merging, inspect git status and stop if unrelated local changes would be touched.
- Verify the branch exists before merging.
- Merge with a normal non-fast-forward merge unless the branch is already merged.
- If conflicts occur, resolve them carefully and preserve existing target fields and runner behavior.
- After every merge or conflict resolution, run: go run ./cmd/bach affected
- If affected targets include shell/lint, run: go run ./cmd/bach run shell/lint
- Then run: go run ./cmd/bach -f examples/plan-agents/Bachfile.verification pipeline/all-plan-tests
- Leave the worktree clean except for the merge commit created by this operation.
- When and only when the merge and tests are complete, write final readiness marker exactly: $completed
- If blocked, conflicts cannot be resolved, the branch is missing, or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! sh examples/plan-agents/scripts/run-opencode-session.sh "merge-agent-$plan_number" "$branch" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
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
