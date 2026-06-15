#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf '# implementer\n' >"$E2E_PROJECT/prompts/implementer.md"
  printf '# reviewer\n' >"$E2E_PROJECT/prompts/reviewer.md"
  printf '# plan\n' >"$E2E_PROJECT/plans/change.md"
}

init_project_git() {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" add .
  git -C "$E2E_PROJECT" -c user.name="E2E" -c user.email="e2e@example.invalid" commit -m initial >/dev/null
}

write_policy_bachfile() {
  local name="$1"
  write_bachfile <<HCL
project "e2e" {
  root  = "."
}

provider "local" {
  type    = "agent"
  command = ["sh", "provider.sh"]
}

prompt "implementer" { path = "prompts/implementer.md" }
prompt "reviewer" { path = "prompts/reviewer.md" }

agent "reviewer" {
  mode     = "review"
  provider = provider.local
  role     = "reviewer"
  prompt   = prompt.reviewer
}

policy "quality" {
  reviewers = [agent.reviewer]
}

agent "$name" {
  mode     = "implement"
  provider = provider.local
  role     = "implementer"
  prompt   = prompt.implementer
  plan     = "plans/change.md"
  policy   = policy.quality

  improve {
    max_attempts = 2
    until        = "policy.passed"
  }
}
HCL
}

write_retry_bachfile() {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

provider "local" {
  type    = "agent"
  command = ["sh", "provider.sh"]
}

prompt "implementer" { path = "prompts/implementer.md" }

agent "flaky" {
  mode     = "implement"
  provider = provider.local
  role     = "implementer"
  prompt   = prompt.implementer
  plan     = "plans/change.md"

  retry {
    attempts = 2
  }
}
HCL
}

write_fix_provider() {
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/bin/sh
set -eu
if [ "$BACH_AGENT_MODE" = "review" ]; then
  status="pass"
  finding=""
  if ! grep -qx fixed app.txt; then
    status="failed"
    finding=',"findings":[{"Kind":"quality","Severity":"error","Rule":"app-fixed","Message":"quality gate: app.txt must be fixed"}]'
  fi
  cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"mode":"review","role":"$BACH_AGENT_ROLE","status":"$status","subject":{"target":"$BACH_AGENT_SUBJECT_TARGET","workspace":"$BACH_AGENT_SUBJECT_WORKSPACE","commit":"$BACH_AGENT_SUBJECT_COMMIT","plan":"$BACH_AGENT_SUBJECT_PLAN"}$finding}
JSON
  exit 0
fi
printf 'attempt=%s feedback=%s\n' "$BACH_AGENT_ATTEMPT" "$BACH_AGENT_FEEDBACK_BUNDLE" >>"$BACH_PROJECT_ROOT/attempts.log"
if [ -n "$BACH_AGENT_FEEDBACK_BUNDLE" ]; then
  printf fixed > app.txt
else
  printf broken > app.txt
fi
git add app.txt
git -c user.name='E2E Agent' -c user.email='e2e@example.invalid' commit -q -m "attempt $BACH_AGENT_ATTEMPT"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"local","provider_type":"agent","provider_command":["sh","provider.sh"],"mode":"implement","status":"passed","attempt":$BACH_AGENT_ATTEMPT,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"attempt $BACH_AGENT_ATTEMPT"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

write_never_provider() {
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/bin/sh
set -eu
if [ "$BACH_AGENT_MODE" = "review" ]; then
  cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"mode":"review","role":"$BACH_AGENT_ROLE","status":"failed","subject":{"target":"$BACH_AGENT_SUBJECT_TARGET","workspace":"$BACH_AGENT_SUBJECT_WORKSPACE","commit":"$BACH_AGENT_SUBJECT_COMMIT","plan":"$BACH_AGENT_SUBJECT_PLAN"},"findings":[{"Kind":"quality","Severity":"error","Rule":"app-fixed","Message":"quality gate: latest app.txt is still broken"}]}
JSON
  exit 0
fi
printf 'attempt=%s\n' "$BACH_AGENT_ATTEMPT" >>"$BACH_PROJECT_ROOT/attempts.log"
printf 'broken %s' "$BACH_AGENT_ATTEMPT" > app.txt
git add app.txt
git -c user.name='E2E Agent' -c user.email='e2e@example.invalid' commit -q -m "attempt $BACH_AGENT_ATTEMPT"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"local","provider_type":"agent","provider_command":["sh","provider.sh"],"mode":"implement","status":"passed","attempt":$BACH_AGENT_ATTEMPT,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"attempt $BACH_AGENT_ATTEMPT"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

write_flaky_provider() {
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/bin/sh
set -eu
printf 'agent-attempt=%s feedback=%s\n' "$BACH_AGENT_ATTEMPT" "$BACH_AGENT_FEEDBACK_BUNDLE" >>"$BACH_PROJECT_ROOT/attempts.log"
if [ ! -f "$BACH_PROJECT_ROOT/failed-once" ]; then
  touch "$BACH_PROJECT_ROOT/failed-once"
  exit 1
fi
printf fixed > app.txt
git add app.txt
git -c user.name='E2E Agent' -c user.email='e2e@example.invalid' commit -q -m fixed
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"local","provider_type":"agent","provider_command":["sh","provider.sh"],"mode":"implement","status":"passed","attempt":$BACH_AGENT_ATTEMPT,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"fixed"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

@test "agent improve passes feedback bundle to next attempt and preserves history" {
  write_fix_provider
  write_policy_bachfile "fix"
  init_project_git

  run bach run agent/fix
  assert_success
  assert_file_contains "$E2E_PROJECT/attempts.log" "attempt=1 feedback="
  assert_file_contains "$E2E_PROJECT/attempts.log" "attempt=2 feedback=$E2E_PROJECT/.bach/runs/"
  assert_file_contains "$E2E_PROJECT/.bach/agents/fix/app.txt" "fixed"
  run_dirs=("$E2E_PROJECT"/.bach/runs/*/agent__fix)
  run_dir="${run_dirs[0]}"
  assert_file_contains "$run_dir/attempt-history.json" '"policy_passed": false'
  assert_file_contains "$run_dir/attempt-history.json" '"policy_passed": true'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"verdict": "failed"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"failed_gates"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"findings"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"required_target_failures"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"reviewer_summaries"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"report_paths"'
  assert_file_contains "$run_dir/attempt-1/feedback-bundle.json" '"log_paths"'

  run git -C "$E2E_PROJECT/.bach/agents/fix" log --oneline --grep "attempt"
  assert_success
  assert_output_contains "attempt 1"
  assert_output_contains "attempt 2"
}

@test "agent policy evaluates latest attempt and exhaustion reports evidence" {
  write_never_provider
  write_policy_bachfile "never"
  init_project_git

  run bach run agent/never
  assert_failure
  assert_output_contains "failed after 2 attempts"
  assert_file_contains "$E2E_PROJECT/attempts.log" "attempt=1"
  assert_file_contains "$E2E_PROJECT/attempts.log" "attempt=2"
  run_dirs=("$E2E_PROJECT"/.bach/runs/*/agent__never)
  run_dir="${run_dirs[0]}"
  assert_file_contains "$run_dir/attempt-history.json" '"attempt": 1'
  assert_file_contains "$run_dir/attempt-history.json" '"attempt": 2'
  assert_file_contains "$run_dir/attempt-2/feedback-bundle.json" "quality gate: latest app.txt is still broken"
}

@test "agent retry is separate from improve feedback attempts" {
  write_flaky_provider
  write_retry_bachfile
  init_project_git

  run bach run agent/flaky
  assert_success
  assert_line_count "$E2E_PROJECT/attempts.log" "agent-attempt=1 feedback=" 2
  run_dirs=("$E2E_PROJECT"/.bach/runs/*/agent__flaky)
  run_dir="${run_dirs[0]}"
  [[ ! -e "$run_dir/attempt-1/feedback-bundle.json" ]]
}
