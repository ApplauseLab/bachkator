#!/bin/sh
set -eu

agent_number="$1"
branch="bach/architecture-phase-$agent_number"
completed="MERGE_PHASE_${agent_number}_COMPLETED"
failed="MERGE_PHASE_${agent_number}_FAILED"

sh scripts/agents/run-merge-agent.sh \
  --role "architecture-merge-agent-$agent_number" \
  --stable-name "$branch" \
  --branch "$branch" \
  --log-dir ".bach/architecture-merge-runs" \
  --checkpoint ".bach/opencode-sessions/architecture-merge-agent-$agent_number.checkpoint" \
  --prompt-file ".bach/opencode-sessions/architecture-merge-agent-$agent_number.prompt.md" \
  --verification "go run ./cmd/bach --file Bachfile.architecture-migration.verification run pipeline/all-phase-tests as far as the current phase structure allows" \
  --full-test "go run ./cmd/bach run shell/test" \
  --completed-marker "$completed" \
  --failed-marker "$failed" \
  --extra-rule "Verification file: Bachfile.architecture-migration.verification." \
  --extra-rule "Stop if unrelated local changes would be touched." \
  --extra-rule "Update the checkpoint file after each milestone: status inspected, branch verified, merge started, conflicts resolved, affected targets inspected, regression passed, blocked, or complete."
