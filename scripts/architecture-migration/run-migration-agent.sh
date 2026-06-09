#!/bin/sh
set -eu

agent_number="$1"
phase_number="$2"
phase_name="$3"
acceptance_target="$4"

log_dir=".bach/architecture-agent-runs"
log_file="$log_dir/phase-$phase_number.log"
current_log="$log_dir/phase-$phase_number.current.log"
checkpoint_file=".bach/opencode-sessions/architecture-phase-agent-$agent_number.checkpoint"
prompt_file=".bach/opencode-sessions/architecture-phase-agent-$agent_number.prompt.md"
branch="bach/architecture-phase-$agent_number"
plan_path="/Users/kris/scratchpad/bachkator/plans/phase-$phase_number-$phase_name.md"
completed="PHASE_${agent_number}_COMPLETED"
failed="PHASE_${agent_number}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are architecture migration phase-agent-$agent_number for Bachkator.

Scope: phase $phase_number, $phase_name.
Plan path: $plan_path.
Feature branch: $branch.
Verification file: Bachfile.architecture-migration.verification.
Acceptance gate: $acceptance_target.
Checkpoint file: $checkpoint_file.

Rules:
- Create or reuse a separate feature worktree outside this checkout on branch $branch.
- Leave the main checkout untouched.
- This invocation may be a resumed OpenCode session. Before changing files, inspect the checkpoint file and existing branch/worktree state.
- Update the checkpoint file after each milestone: worktree ready, dry-run inspected, edits complete, affected targets inspected, acceptance gate passed, full test passed or skipped with reason, committed, blocked, or complete.
- Start with: go run ./cmd/bach -list
- Inspect the acceptance gate: go run ./cmd/bach -f Bachfile.architecture-migration.verification -dry-run $acceptance_target
- Implement only the phase described by the plan path above.
- After edits, run: go run ./cmd/bach affected
- Then run: go run ./cmd/bach -f Bachfile.architecture-migration.verification $acceptance_target
- Run go run ./cmd/bach shell/test if feasible for the slice.
- Commit only intended feature files on branch $branch.
- When and only when ready, write final readiness marker exactly: $completed
- If blocked or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! sh scripts/architecture-migration/run-opencode-session.sh "architecture-phase-agent-$agent_number" "$phase_name" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
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
