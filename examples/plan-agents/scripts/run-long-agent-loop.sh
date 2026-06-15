#!/bin/sh
set -eu

loop_name="$1"
loop_goal="$2"

log_dir=".bach/agent-runs"
log_file="$log_dir/long-loop-$loop_name.log"
current_log="$log_dir/long-loop-$loop_name.current.log"
checkpoint_file=".bach/opencode-sessions/long-loop-$loop_name.checkpoint"
prompt_file=".bach/opencode-sessions/long-loop-$loop_name.prompt.md"
completed="LONG_LOOP_${loop_name}_COMPLETED"
failed="LONG_LOOP_${loop_name}_FAILED"

mkdir -p "$log_dir" "$(dirname "$prompt_file")"

cat > "$prompt_file" <<EOF
You are a long-running Bachkator maintenance agent loop.

Loop name: $loop_name.
Goal: $loop_goal
Checkpoint file: $checkpoint_file.

Rules:
- This is intentionally long-running and resumable. Inspect the checkpoint before changing files.
- Start with: go run ./cmd/bach list
- Inspect likely gates with dry-run before running them.
- Repeat this loop until the goal is complete or blocked:
  1. Inspect git status and recent context.
  2. Run: go run ./cmd/bach affected
  3. Make one coherent improvement.
  4. Run focused checks, then go run ./cmd/bach run shell/lint and go run ./cmd/bach run shell/test.
  5. Update docs when behavior, commands, config, examples, or workflows changed.
  6. Append a checkpoint entry describing what changed, what passed, and what remains.
- Keep commits small if committing is requested by the operator; otherwise leave a clean handoff.
- Never run a release target from this loop unless explicitly requested.
- When complete, write the final marker exactly: $completed
- If blocked, write: $failed: reason
- Never write $completed after writing $failed.
EOF

if ! sh examples/plan-agents/scripts/run-opencode-session.sh \
  "long-loop" \
  "$loop_name" \
  "$log_file" \
  "$current_log" \
  "$checkpoint_file" \
  "$prompt_file"; then
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
