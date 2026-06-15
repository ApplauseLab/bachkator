#!/bin/sh
set -eu

prompt_path="${1:?missing generated Bach prompt path}"
if ! command -v opencode >/dev/null 2>&1; then
  printf 'opencode is required for this real-provider todo app fixture\n' >&2
  exit 127
fi

agent="${OPENCODE_AGENT:-general}"
title="${BACH_AGENT_TARGET:-bach-agent}"
if [ "${BACH_OPENCODE_SKIP_PERMISSIONS:-}" = "1" ]; then
  opencode run --dangerously-skip-permissions --agent "$agent" --title "$title" "$(cat "$prompt_path")"
else
  opencode run --agent "$agent" --title "$title" "$(cat "$prompt_path")"
fi
