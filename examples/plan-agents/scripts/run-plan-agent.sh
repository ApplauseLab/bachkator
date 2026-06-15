#!/bin/sh
set -eu

plan_number="$1"
plan_name="$2"
acceptance_target="$3"
profile="${4:-}"

log_dir=".bach/agent-runs"
log_file="$log_dir/plan-$plan_number.log"
current_log="$log_dir/plan-$plan_number.current.log"
checkpoint_file=".bach/opencode-sessions/feature-agent-$plan_number.checkpoint"
prompt_file=".bach/opencode-sessions/feature-agent-$plan_number.prompt.md"
branch="bach/plan-$plan_number"
completed="PLAN_${plan_number}_COMPLETED"
failed="PLAN_${plan_number}_FAILED"

profile_args=""
if [ -n "$profile" ]; then
  profile_args="--profile $profile"
fi

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are feature-agent-$plan_number for Bachkator.

Scope: $plan_name.
Feature branch: $branch.
Verification file: examples/plan-agents/Bachfile.verification.
Acceptance gate: $acceptance_target.
Checkpoint file: $checkpoint_file.

Rules:
- Create or reuse a separate feature worktree outside this checkout on branch $branch.
- Leave the main checkout untouched.
- This invocation may be a resumed OpenCode session. Before changing files, inspect the checkpoint file and existing branch/worktree state.
- Update the checkpoint file after each milestone: worktree ready, dry-run inspected, edits complete, affected targets inspected, acceptance gate passed, committed, blocked, or complete.
- Start with: go run ./cmd/bach list
- Inspect the acceptance gate: go run ./cmd/bach --file examples/plan-agents/Bachfile.verification $profile_args run --dry-run $acceptance_target
- After edits, run: go run ./cmd/bach affected
- If affected targets include shell/lint, or before final handoff in this repository, run:
  go run ./cmd/bach run shell/lint
- Then run: go run ./cmd/bach --file examples/plan-agents/Bachfile.verification $profile_args run $acceptance_target
- Commit only intended feature files on branch $branch.
- When and only when ready, write final readiness marker exactly: $completed
- If blocked or tests fail, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! sh examples/plan-agents/scripts/run-opencode-session.sh "feature-agent-$plan_number" "$plan_name" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
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
