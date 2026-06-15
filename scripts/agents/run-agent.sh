#!/bin/sh
set -eu

role=""
stable_name=""
plan_path=""
branch=""
workspace=""
workspace_mode="clone"
log_dir=".bach/agent-runs"
checkpoint_file=""
prompt_file=""
dry_run_commands="go run ./cmd/bach run --dry-run shell/lint and go run ./cmd/bach run --dry-run shell/test"
gate_command="go run ./cmd/bach run --log-only --force group/gate"
acceptance_command=""
full_test_command=""
completed_marker=""
failed_marker=""
extra_rules=""

append_rule() {
  if [ -z "$extra_rules" ]; then
    extra_rules="- $1"
  else
    extra_rules="$extra_rules
- $1"
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --role) role="$2"; shift 2 ;;
    --stable-name) stable_name="$2"; shift 2 ;;
    --plan) plan_path="$2"; shift 2 ;;
    --branch) branch="$2"; shift 2 ;;
    --workspace) workspace="$2"; shift 2 ;;
    --workspace-mode) workspace_mode="$2"; shift 2 ;;
    --log-dir) log_dir="$2"; shift 2 ;;
    --checkpoint) checkpoint_file="$2"; shift 2 ;;
    --prompt-file) prompt_file="$2"; shift 2 ;;
    --dry-run-commands) dry_run_commands="$2"; shift 2 ;;
    --gate) gate_command="$2"; shift 2 ;;
    --acceptance) acceptance_command="$2"; shift 2 ;;
    --full-test) full_test_command="$2"; shift 2 ;;
    --completed-marker) completed_marker="$2"; shift 2 ;;
    --failed-marker) failed_marker="$2"; shift 2 ;;
    --extra-rule) append_rule "$2"; shift 2 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
done

if [ -z "$role" ] || [ -z "$stable_name" ] || [ -z "$plan_path" ] || [ -z "$branch" ]; then
  printf 'run-agent requires --role, --stable-name, --plan, and --branch\n' >&2
  exit 2
fi

safe_name=$(printf '%s' "$stable_name" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' '-' | sed 's/^-*//;s/-*$//;s/--*/-/g')
if [ -z "$checkpoint_file" ]; then
  checkpoint_file=".bach/opencode-sessions/$role-$safe_name.checkpoint"
fi
if [ -z "$prompt_file" ]; then
  prompt_file=".bach/opencode-sessions/$role-$safe_name.prompt.md"
fi
log_file="$log_dir/$safe_name.log"
current_log="$log_dir/$safe_name.current.log"
repo_root="$(pwd)"

mkdir -p "$log_dir" "$(dirname "$prompt_file")" "$(dirname "$checkpoint_file")"

workspace_rules="- Work in this checkout."
case "$workspace_mode" in
  clone)
    workspace_rules="- Do not edit $repo_root directly.
- Use the dedicated clone directory $repo_root/$workspace.
- If the clone directory does not exist, clone $repo_root into it.
- In the clone, create or reuse branch $branch from the current main branch."
    ;;
  worktree)
    workspace_rules="- Create or reuse a separate feature worktree outside this checkout on branch $branch.
- Leave the main checkout untouched."
    ;;
  main)
    workspace_rules="- Work in this checkout on branch $branch."
    ;;
  *) printf 'unknown workspace mode: %s\n' "$workspace_mode" >&2; exit 2 ;;
esac

{
  printf 'You are %s for Bachkator.\n\n' "$role"
  printf 'Plan path: %s.\n' "$plan_path"
  printf 'Branch: %s.\n' "$branch"
  if [ -n "$workspace" ]; then
    printf 'Workspace: %s/%s.\n' "$repo_root" "$workspace"
  fi
  printf 'Checkpoint file: %s.\n\n' "$checkpoint_file"
  printf 'Rules:\n'
  printf '%s\n' "$workspace_rules"
  printf '%s\n' '- This invocation may be a resumed OpenCode session. Inspect checkpoint, workspace state, branch state, and recent logs before changing files.'
  printf '%s\n' '- Start with: go run ./cmd/bach list'
  printf '%s\n' "- Dry-run likely gates: $dry_run_commands"
  printf '%s\n' '- Implement only the phase described by the plan path.'
  printf '%s\n' '- Use Bach targets only for fmt/test/lint/e2e/docs.'
  printf '%s\n' '- After edits, run: go run ./cmd/bach affected'
  if [ -n "$acceptance_command" ]; then
    printf '%s\n' "- Run acceptance: $acceptance_command"
  fi
  if [ -n "$full_test_command" ]; then
    printf '%s\n' "- Run broader test if feasible: $full_test_command"
  fi
  printf '%s\n' "- Run final gate: $gate_command"
  printf '%s\n' "- Commit only intended files on branch $branch."
  printf '%s\n' '- Do not merge back to the main checkout.'
  if [ -n "$completed_marker" ]; then
    printf '%s\n' "- When and only when ready, write final readiness marker exactly: $completed_marker"
  fi
  if [ -n "$failed_marker" ]; then
    printf '%s\n' "- If blocked or tests fail, write: $failed_marker: reason"
  fi
  if [ -n "$completed_marker" ] && [ -n "$failed_marker" ]; then
    printf '%s\n' "- Never write $completed_marker after writing $failed_marker."
  fi
  if [ -n "$extra_rules" ]; then
    printf '%s\n' "$extra_rules"
  fi
  printf '%s\n' '- Return a concise handoff with workspace path, branch, commit, gate run id, and follow-ups.'
} > "$prompt_file"

if ! sh scripts/agents/run-opencode-session.sh "$role" "$stable_name" "$log_file" "$current_log" "$checkpoint_file" "$prompt_file"; then
  if [ -n "$failed_marker" ]; then
    printf '%s: opencode exited non-zero\n' "$failed_marker" >> "$current_log"
  fi
  cat "$current_log" >> "$log_file"
  exit 1
fi

if [ -n "$failed_marker" ] && grep -q "$failed_marker" "$current_log"; then
  cat "$current_log" >> "$log_file"
  exit 1
fi

if [ -n "$completed_marker" ] && ! grep -q "$completed_marker" "$current_log"; then
  cat "$current_log" >> "$log_file"
  exit 1
fi

printf 'checkpoint.status=completed\n' >> "$checkpoint_file"
cat "$current_log" >> "$log_file"
