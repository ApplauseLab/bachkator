#!/bin/sh
set -eu

for plan_number in "$@"; do
  log_file=".bach/agent-runs/plan-$plan_number.log"
  completed="PLAN_${plan_number}_COMPLETED"
  failed="PLAN_${plan_number}_FAILED"

  if [ ! -f "$log_file" ]; then
    printf 'missing log: %s\n' "$log_file" >&2
    exit 1
  fi

  if grep -q "$failed" "$log_file"; then
    printf 'worker reported failure: %s\n' "$failed" >&2
    exit 1
  fi

  if ! grep -q "$completed" "$log_file"; then
    printf 'missing completion marker: %s in %s\n' "$completed" "$log_file" >&2
    exit 1
  fi
done
