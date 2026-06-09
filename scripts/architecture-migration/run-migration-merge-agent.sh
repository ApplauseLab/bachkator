#!/bin/sh
set -eu

agent_number="$1"
branch="bach/architecture-phase-$agent_number"
log_dir=".bach/architecture-merge-runs"
log_file="$log_dir/phase-$agent_number.log"
current_log="$log_dir/phase-$agent_number.current.log"
checkpoint_file=".bach/opencode-sessions/architecture-merge-agent-$agent_number.checkpoint"
prompt_file=".bach/opencode-sessions/architecture-merge-agent-$agent_number.prompt.md"
completed="MERGE_PHASE_${agent_number}_COMPLETED"
failed="MERGE_PHASE_${agent_number}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are architecture migration merge-agent-$agent_number for Bachkator.

Branch to merge: $branch.
Verification file: Bachfile.architecture-migration.verification.
Checkpoint file: $checkpoint_file.

Rules:
- Work in this checkout; this is the serialized merge-agent lane.
- This invocation may be a resumed OpenCode session. Before changing files, inspect the checkpoint file, git status, and whether $branch is already merged.
- Update the checkpoint file after each milestone: status inspected, branch verified, merge started, conflicts resolved, affected targets inspected, regression passed, blocked, or complete.
- Before merging, inspect git status, git diff, and git log --oneline -10.
- Stop if unrelated local changes would be touched.
- Verify the branch exists before merging.
- Merge with a normal non-fast-forward merge unless the branch is already merged.
- If conflicts occur, resolve them carefully and preserve existing target fields and runner behavior.
- After every merge or conflict resolution, run: go run ./cmd/bach affected
- Then run: go run ./cmd/bach -f Bachfile.architecture-migration.verification pipeline/all-phase-tests as far as the current phase structure allows.
- Then run: go run ./cmd/bach shell/test
- Leave the worktree clean except for the merge commit created by this operation.
- When and only when the merge and tests are complete, write final readiness marker exactly: $completed
- If blocked, conflicts cannot be resolved, the branch is missing, or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! sh scripts/architecture-migration/run-opencode-session.sh "architecture-merge-agent-$agent_number" "$branch" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
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
