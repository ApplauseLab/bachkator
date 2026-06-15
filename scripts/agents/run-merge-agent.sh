#!/bin/sh
set -eu

role="merge-agent"
stable_name=""
branch=""
source=""
integration_branch=""
log_dir=".bach/merge-runs"
checkpoint_file=""
prompt_file=""
verification_command=""
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
    --branch) branch="$2"; shift 2 ;;
    --source) source="$2"; shift 2 ;;
    --integration-branch) integration_branch="$2"; shift 2 ;;
    --log-dir) log_dir="$2"; shift 2 ;;
    --checkpoint) checkpoint_file="$2"; shift 2 ;;
    --prompt-file) prompt_file="$2"; shift 2 ;;
    --verification) verification_command="$2"; shift 2 ;;
    --full-test) full_test_command="$2"; shift 2 ;;
    --completed-marker) completed_marker="$2"; shift 2 ;;
    --failed-marker) failed_marker="$2"; shift 2 ;;
    --extra-rule) append_rule "$2"; shift 2 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
done

if [ -z "$branch" ]; then
  printf 'run-merge-agent requires --branch\n' >&2
  exit 2
fi
if [ -z "$stable_name" ]; then
  stable_name="$branch"
fi
if [ -z "$integration_branch" ]; then
  integration_branch="$branch"
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

mkdir -p "$log_dir" "$(dirname "$prompt_file")" "$(dirname "$checkpoint_file")"

if [ -n "$source" ] && [ -d "$source/.git" ]; then
  git fetch "$source" "$branch:refs/heads/$integration_branch"
fi

if ! git show-ref --verify --quiet "refs/heads/$integration_branch"; then
  printf 'merge branch %s does not exist after fetch/check\n' "$integration_branch" >&2
  exit 1
fi

merge_required=true
if git merge-base --is-ancestor "$integration_branch" HEAD; then
  merge_required=false
fi

{
  printf 'You are %s for Bachkator.\n\n' "$role"
  printf 'Requested branch: %s.\n' "$branch"
  printf 'Branch to merge: %s.\n' "$integration_branch"
  printf 'Merge required by preflight: %s.\n' "$merge_required"
  if [ -n "$source" ]; then
    printf 'Source repository: %s.\n' "$source"
  fi
  printf 'Checkpoint file: %s.\n\n' "$checkpoint_file"
  printf 'Rules:\n'
  printf '%s\n' '- Work in this checkout; this is the serialized merge-agent lane.'
  printf '%s\n' '- This invocation may be a resumed OpenCode session. Inspect checkpoint, git status, branch state, and recent logs before changing files.'
  printf '%s\n' '- Before merging, inspect git status, git diff, and git log --oneline -10.'
  printf '%s\n' "- Verify branch $integration_branch exists locally with plain git commands."
  printf '%s\n' "- Preflight determined merge_required=$merge_required using plain git merge-base --is-ancestor. Do not infer ancestry from empty command output."
  printf '%s\n' "- If merge_required=true, perform a real merge of $integration_branch. Prefer git merge --no-ff $integration_branch when possible."
  printf '%s\n' "- If merge_required=false, confirm with plain git merge-base --is-ancestor $integration_branch HEAD before skipping the merge."
  printf '%s\n' '- If conflicts occur, resolve them carefully and preserve existing target fields, runner behavior, and previously merged phase behavior.'
  printf '%s\n' '- After every merge or conflict resolution, run: go run ./cmd/bach affected'
  if [ -n "$verification_command" ]; then
    printf '%s\n' "- Run verification: $verification_command"
  fi
  if [ -n "$full_test_command" ]; then
    printf '%s\n' "- Run broader test: $full_test_command"
  fi
  printf '%s\n' '- Leave the worktree clean except for the merge commit created by this operation.'
  if [ -n "$completed_marker" ]; then
    printf '%s\n' "- When and only when the merge and required tests are complete, write final readiness marker exactly: $completed_marker"
  fi
  if [ -n "$failed_marker" ]; then
    printf '%s\n' "- If blocked, conflicts cannot be resolved, the branch is missing, or tests fail, write: $failed_marker: reason"
  fi
  if [ -n "$completed_marker" ] && [ -n "$failed_marker" ]; then
    printf '%s\n' "- Never write $completed_marker after writing $failed_marker."
  fi
  if [ -n "$extra_rules" ]; then
    printf '%s\n' "$extra_rules"
  fi
  printf '%s\n' '- Return a concise handoff with merge status, merge commit if any, affected output, and follow-ups.'
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

if [ "$merge_required" = "true" ] && ! git merge-base --is-ancestor "$integration_branch" HEAD; then
  if [ -n "$failed_marker" ]; then
    printf '%s: branch %s is still not merged into HEAD after merge agent completed\n' \
      "$failed_marker" "$integration_branch" >> "$current_log"
  fi
  cat "$current_log" >> "$log_file"
  exit 1
fi

if [ "$merge_required" = "false" ] && ! git merge-base --is-ancestor "$integration_branch" HEAD; then
  if [ -n "$failed_marker" ]; then
    printf '%s: branch %s was expected to be merged but is not an ancestor of HEAD\n' \
      "$failed_marker" "$integration_branch" >> "$current_log"
  fi
  cat "$current_log" >> "$log_file"
  exit 1
fi

if [ -n "$completed_marker" ] && ! grep -q "$completed_marker" "$current_log"; then
  cat "$current_log" >> "$log_file"
  exit 1
fi

printf 'checkpoint.status=completed\n' >> "$checkpoint_file"
cat "$current_log" >> "$log_file"
