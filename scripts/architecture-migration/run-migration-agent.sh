#!/bin/sh
set -eu

agent_number="$1"
phase_number="$2"
phase_name="$3"
acceptance_target="$4"

branch="bach/architecture-phase-$agent_number"
plan_path="/Users/kris/scratchpad/bachkator/plans/phase-$phase_number-$phase_name.md"
completed="PHASE_${agent_number}_COMPLETED"
failed="PHASE_${agent_number}_FAILED"

sh scripts/agents/run-agent.sh \
  --role "architecture-phase-agent-$agent_number" \
  --stable-name "$phase_name" \
  --plan "$plan_path" \
  --branch "$branch" \
  --workspace-mode "worktree" \
  --log-dir ".bach/architecture-agent-runs" \
  --checkpoint ".bach/opencode-sessions/architecture-phase-agent-$agent_number.checkpoint" \
  --prompt-file ".bach/opencode-sessions/architecture-phase-agent-$agent_number.prompt.md" \
  --dry-run-commands "go run ./cmd/bach --file Bachfile.architecture-migration.verification run --dry-run $acceptance_target" \
  --acceptance "go run ./cmd/bach --file Bachfile.architecture-migration.verification run $acceptance_target" \
  --full-test "go run ./cmd/bach run shell/test if feasible for the slice" \
  --gate "go run ./cmd/bach --file Bachfile.architecture-migration.verification run $acceptance_target" \
  --completed-marker "$completed" \
  --failed-marker "$failed" \
  --extra-rule "Verification file: Bachfile.architecture-migration.verification." \
  --extra-rule "Update the checkpoint file after each milestone: worktree ready, dry-run inspected, edits complete, affected targets inspected, acceptance gate passed, full test passed or skipped with reason, committed, blocked, or complete."
