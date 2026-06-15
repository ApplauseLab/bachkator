#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf '# implementer\n' >"$E2E_PROJECT/prompts/implementer.md"
  printf '# architecture\n' >"$E2E_PROJECT/prompts/architecture.md"
  printf '# docs\n' >"$E2E_PROJECT/prompts/docs.md"
  printf '# security\n' >"$E2E_PROJECT/prompts/security.md"
  printf '# plan\n' >"$E2E_PROJECT/plans/change.md"
  write_provider
}

write_provider() {
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/bin/sh
set -eu
  event_dir="$(dirname "$BACH_AGENT_REPORT_PATH")/agent-events"
  mkdir -p "$event_dir"
if [ "$BACH_AGENT_MODE" = "implement" ]; then
  printf 'implemented\n' > implementation.txt
  git add implementation.txt
  git -c user.name='Bach Test' -c user.email='bach@example.test' commit -m 'Implement agent change' >/dev/null
  commit="$(git rev-parse HEAD)"
  branch="$(git branch --show-current)"
  cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"local","provider_type":"agent","provider_command":["sh","provider.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["implementation.txt"],"summary":"implemented change"}
JSON
  exit 0
fi
printf '%s start\n' "$BACH_AGENT_ROLE" >> "$event_dir/review.log"
touch "$event_dir/start-$BACH_AGENT_ROLE"
while [ "$(find "$event_dir" -name 'start-*' | wc -l | tr -d ' ')" -lt 3 ]; do
  sleep 0.1
done
finding=""
if [ "$BACH_AGENT_ROLE" = "docs-sweeper" ] && [ -f docs_missing ]; then
  finding=',"findings":[{"Kind":"docs","Severity":"error","Rule":"docs-missing","Message":"docs were not updated for user-visible change","File":"docs/agent-guide.md","Line":1}]'
fi
if [ "$BACH_AGENT_ROLE" = "security-reviewer" ] && [ -f security_risk ]; then
  finding=',"findings":[{"Kind":"security","Severity":"error","Rule":"unsafe-shell","Message":"unsafe shell command construction","File":"scripts/deploy.sh","Line":7}]'
fi
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"mode":"review","role":"$BACH_AGENT_ROLE","status":"pass","subject":{"target":"$BACH_AGENT_SUBJECT_TARGET","workspace":"$BACH_AGENT_SUBJECT_WORKSPACE","commit":"$BACH_AGENT_SUBJECT_COMMIT","plan":"$BACH_AGENT_SUBJECT_PLAN"}$finding}
JSON
printf '%s finish\n' "$BACH_AGENT_ROLE" >> "$event_dir/review.log"
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

write_agents_bachfile() {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

provider "local" {
  type    = "agent"
  command = ["sh", "provider.sh"]
}

prompt "implementer" { path = "prompts/implementer.md" }
prompt "architecture" { path = "prompts/architecture.md" }
prompt "docs" { path = "prompts/docs.md" }
prompt "security" { path = "prompts/security.md" }

shell "required" {
  shell = "mkdir -p .bach/artifacts && printf required > .bach/artifacts/required-ran"
}

agent "architecture" {
  mode     = "review"
  provider = provider.local
  role     = "architecture-reviewer"
  prompt   = prompt.architecture
}

agent "docs" {
  mode     = "review"
  provider = provider.local
  role     = "docs-sweeper"
  prompt   = prompt.docs
}

agent "security" {
  mode     = "review"
  provider = provider.local
  role     = "security-reviewer"
  prompt   = prompt.security
}

policy "merge" {
  reviewers = [agent.architecture, agent.docs, agent.security]
  required_targets = [shell.required]

  quality_gate {
    metric = "findings.error.open.count"
    max    = 0
  }
}

agent "subject" {
  mode     = "implement"
  provider = provider.local
  role     = "implementer"
  prompt   = prompt.implementer
  plan     = "plans/change.md"
  policy   = policy.merge
}
HCL
  init_project_git
}

init_project_git() {
  (cd "$E2E_PROJECT" && git init -q && git add . && git -c user.name='Bach Test' -c user.email='bach@example.test' commit -m 'Initial project' >/dev/null)
}

commit_project_change() {
  (cd "$E2E_PROJECT" && git add . && git -c user.name='Bach Test' -c user.email='bach@example.test' commit -m 'Update project fixture' >/dev/null)
}

@test "agent policy runs reviewers in parallel and records subject metadata" {
  write_agents_bachfile

  run bach run agent/subject
  assert_success
  review_log=("$E2E_PROJECT"/.bach/runs/*/policy__merge@agent.subject/reviews/agent-events/review.log)
  review_log="${review_log[0]}"
  assert_file_contains "$review_log" "architecture-reviewer start"
  assert_file_contains "$review_log" "docs-sweeper start"
  assert_file_contains "$review_log" "security-reviewer start"
  assert_line_before "$review_log" "docs-sweeper start" "architecture-reviewer finish"
  assert_file_contains "$E2E_PROJECT/.bach/agents/subject/.bach/artifacts/required-ran" "required"

  run bach quality summary
  assert_success
  assert_output_contains "findings.error.open.count"
  assert_output_contains "success"

  run bach runs list --target agent/subject
  assert_success
  assert_output_contains "agent/subject"
  assert_output_contains "success"

  run bach runs list --target policy/merge@agent.subject
  assert_success
  assert_output_contains "policy/merge@agent.subject"
  assert_output_contains "success"
}

@test "docs sweeper finding fails policy when docs are missing" {
  write_agents_bachfile
  touch "$E2E_PROJECT/docs_missing"
  commit_project_change

  run bach run agent/subject
  assert_failure
  assert_output_contains "agent policy failed"

  run bach quality findings
  assert_success
  assert_output_contains "docs-missing"
  assert_output_contains "docs were not updated"
}

@test "docs sweeper passes when relevant docs are updated" {
  write_agents_bachfile
  printf 'updated docs\n' >"$E2E_PROJECT/docs_updated"
  commit_project_change

  run bach run agent/subject
  assert_success

  run bach quality findings
  assert_success
  if [[ "$output" == *"docs-missing"* ]]; then
    echo "unexpected docs finding" >&2
    echo "$output" >&2
    return 1
  fi
}

@test "security reviewer finding fails policy and aggregation includes finding" {
  write_agents_bachfile
  touch "$E2E_PROJECT/security_risk"
  commit_project_change

  run bach run agent/subject
  assert_failure
  assert_output_contains "agent policy failed"

  run bach quality findings
  assert_success
  assert_output_contains "unsafe-shell"
  assert_output_contains "unsafe shell command construction"
}
