#!/bin/sh
set -eu

role="$1"
stable_name="$2"
log_file="$3"
current_log="$4"
checkpoint_file="$5"
prompt_file="$6"

session_dir=".bach/opencode-sessions"
prompt_hash=$(shasum -a 256 "$prompt_file" | cut -d ' ' -f 1 | cut -c 1-16)
slug=$(printf '%s' "$stable_name" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' '-' | sed 's/^-*//;s/-*$//;s/--*/-/g' | cut -c 1-48)
if [ -z "$slug" ]; then
  slug="agent"
fi

session_title="bachkator/$role/$slug/$prompt_hash"
session_file="$session_dir/$role-$slug-$prompt_hash.env"
session_id=""
opencode_run_flags="${OPENCODE_RUN_FLAGS:-}"

mkdir -p "$session_dir" "$(dirname "$log_file")" "$(dirname "$checkpoint_file")"

if [ -f "$session_file" ]; then
  session_id=$(sed -n 's/^OPENCODE_SESSION_ID=//p' "$session_file" | head -n 1)
fi

lookup_session_id() {
  opencode session list --format json --max-count 200 2>/dev/null | awk -v title="$session_title" '
    /"id":/ {
      id = $2
      gsub(/[",]/, "", id)
    }
    /"title":/ {
      line = $0
      sub(/^[[:space:]]*"title": "/, "", line)
      sub(/",[[:space:]]*$/, "", line)
      if (line == title) {
        print id
        exit
      }
    }
  '
}

if [ -z "$session_id" ]; then
  session_id=$(lookup_session_id || true)
fi

started_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
{
  printf '\n===== checkpoint run %s =====\n' "$started_at"
  printf 'checkpoint.started_at=%s\n' "$started_at"
  printf 'checkpoint.role=%s\n' "$role"
  printf 'checkpoint.stable_name=%s\n' "$stable_name"
  printf 'checkpoint.prompt_hash=%s\n' "$prompt_hash"
  printf 'checkpoint.session_title=%s\n' "$session_title"
  if [ -n "$session_id" ]; then
    printf 'checkpoint.session_id=%s\n' "$session_id"
    printf 'checkpoint.mode=resume\n'
    printf 'checkpoint.status=resuming\n'
  else
    printf 'checkpoint.mode=start\n'
    printf 'checkpoint.status=starting\n'
  fi
} >> "$checkpoint_file"

{
  printf '\n===== %s %s =====\n' "$started_at" "$session_title"
  if [ -n "$session_id" ]; then
    printf 'Resuming OpenCode session: %s\n' "$session_id"
  else
    printf 'Starting OpenCode session title: %s\n' "$session_title"
  fi
  printf 'Checkpoint file: %s\n\n' "$checkpoint_file"
} > "$current_log"

if [ -n "$session_id" ]; then
  if ! opencode run $opencode_run_flags --session "$session_id" "$(cat "$prompt_file")" >> "$current_log" 2>&1; then
    printf 'checkpoint.status=opencode_failed\n' >> "$checkpoint_file"
    exit 1
  fi
else
  if ! opencode run $opencode_run_flags --title "$session_title" "$(cat "$prompt_file")" >> "$current_log" 2>&1; then
    printf 'checkpoint.status=opencode_failed\n' >> "$checkpoint_file"
    exit 1
  fi
fi

discovered_session_id=$(lookup_session_id || true)
if [ -n "$discovered_session_id" ]; then
  session_id="$discovered_session_id"
  printf 'OPENCODE_SESSION_ID=%s\n' "$session_id" > "$session_file"
  printf 'checkpoint.session_id=%s\n' "$session_id" >> "$checkpoint_file"
fi

printf 'checkpoint.status=opencode_completed\n' >> "$checkpoint_file"
