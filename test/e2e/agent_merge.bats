#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

init_merge_project() {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Merge"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial
  mkdir -p "$E2E_PROJECT/prompts/agents" "$E2E_PROJECT/plans" "$E2E_PROJECT.ignored"
  cp "$BACH_REPO_ROOT/prompts/agents/merge.md" "$E2E_PROJECT/prompts/agents/merge.md"
  printf 'implement fixture\n' >"$E2E_PROJECT/prompts/agents/implement.md"
  printf 'review fixture\n' >"$E2E_PROJECT/prompts/agents/review.md"
  printf 'plan fixture\n' >"$E2E_PROJECT/plans/example.md"
  git clone -q "$E2E_PROJECT" "$E2E_PROJECT/.bach/agents/example"
  git -C "$E2E_PROJECT/.bach/agents/example" switch -q -c bach/e2e-agent
  printf 'subject change\n' >>"$E2E_PROJECT/.bach/agents/example/app.txt"
  git -C "$E2E_PROJECT/.bach/agents/example" add app.txt
  git -C "$E2E_PROJECT/.bach/agents/example" -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m subject
}

write_merge_bachfile() {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

provider "fixture" {
  type    = "agent"
  command = ["$BACH_PROJECT_ROOT/provider.sh"]
}

prompt "implementer" {
  path = "prompts/agents/implement.md"
}

prompt "merge" {
  path = "prompts/agents/merge.md"
}

prompt "review" {
  path = "prompts/agents/review.md"
}

agent "example" {
  mode     = "implement"
  provider = provider.fixture
  prompt   = prompt.implementer
  plan     = "plans/example.md"
  policy   = policy.merge

  workspace {
    mode = "clone"
    path = ".bach/agents/example"
  }

  git {
    branch = "bach/e2e-agent"
    commit = "required"
  }
}

agent "reviewer" {
  mode     = "review"
  provider = provider.fixture
  prompt   = prompt.review
  role     = "architecture-reviewer"
}

policy "merge" {
  reviewers = [agent.reviewer]
}

agent "merge-example" {
  mode     = "merge"
  provider = provider.fixture
  prompt   = prompt.merge
  subject  = agent.example
}
HCL
}

write_passed_policy() {
  run bach run --force agent.example
  assert_success
}

write_merge_provider() {
  local evidence="$1"
  local direct_merge=false
  case "$evidence" in
    *target_branch_commit*) direct_merge=true ;;
  esac
  cat >"$E2E_PROJECT/provider.sh" <<SH
#!/usr/bin/env sh
set -eu
if [ "\$BACH_AGENT_MODE" = "implement" ]; then
  printf 'implementation\n' >>app.txt
  git add app.txt
  git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m implementation
  commit="\$(git rev-parse HEAD)"
  cat >"\$BACH_AGENT_REPORT_PATH" <<JSON
{
  "target": "agent/example",
  "provider_name": "fixture",
  "provider_type": "agent",
  "provider_command": ["\$BACH_PROJECT_ROOT/provider.sh"],
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "\$BACH_AGENT_WORKSPACE",
  "branch": "bach/e2e-agent",
  "commit": "\$commit",
  "changed_files": ["app.txt"],
  "summary": "implemented fixture"
}
JSON
  exit 0
fi
if [ "\$BACH_AGENT_MODE" = "review" ]; then
  cat >"\$BACH_AGENT_REPORT_PATH" <<JSON
{
  "schema": "bach.agent_report.v1",
  "agent": {"role": "architecture-reviewer", "name": "fixture"},
  "mode": "review",
  "subject": {
    "target": "\$BACH_AGENT_SUBJECT_TARGET",
    "workspace": "\$BACH_AGENT_SUBJECT_WORKSPACE",
    "commit": "\$BACH_AGENT_SUBJECT_COMMIT"
  },
  "status": "passed",
  "summary": "reviewed fixture",
  "metrics": [],
  "findings": []
}
JSON
  exit 0
fi
mkdir -p "\$BACH_PROJECT_ROOT/.bach/artifacts/merge-test"
cp "\$BACH_AGENT_CONTEXT_PATH" "\$BACH_PROJECT_ROOT/.bach/artifacts/merge-test/merge-context.json"
cp "\$BACH_AGENT_PROMPT_PATH" "\$BACH_PROJECT_ROOT/.bach/artifacts/merge-test/merge-prompt.md"
if [ "$direct_merge" = true ]; then
  git fetch -q "\$BACH_AGENT_SUBJECT_WORKSPACE" "\$BACH_AGENT_SUBJECT_BRANCH"
  git merge --no-ff -q FETCH_HEAD -m 'merge subject'
fi
current_commit="\$(git rev-parse HEAD)"
printf '%s\n' "subject=\$BACH_AGENT_SUBJECT_TARGET" "branch=\$BACH_AGENT_SUBJECT_BRANCH" "commit=\$BACH_AGENT_SUBJECT_COMMIT" "workspace=\$BACH_AGENT_SUBJECT_WORKSPACE" "policy=\$BACH_AGENT_POLICY_EVIDENCE" >"\$BACH_PROJECT_ROOT/.bach/artifacts/merge-test/provider-env.txt"
cat >"\$BACH_AGENT_REPORT_PATH" <<JSON
{
  "target": "agent/merge-example",
  "provider_name": "fixture",
  "provider_type": "agent",
  "provider_command": ["\$BACH_PROJECT_ROOT/provider.sh"],
  "mode": "merge",
  "status": "passed",
  "subject": {
    "target": "agent/example",
    "workspace": "\$BACH_AGENT_SUBJECT_WORKSPACE",
    "commit": "\$BACH_AGENT_SUBJECT_COMMIT"
  },
  $evidence
  "summary": "merged fixture"
}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

@test "merge agent refuses without passing applied policy verdict" {
  init_merge_project
  write_merge_bachfile
  write_merge_provider '"pr_url": "https://example.test/pr/1", "target_branch_commit": "$current_commit",'

  run bach run --force agent.merge-example
  assert_failure
  assert_output_contains "requires passing applied policy verdict"
}

@test "merge agent defaults to merge-lane lock" {
  init_merge_project
  write_merge_bachfile

  run bach graph --format json
  assert_success
  assert_output_contains '"name": "agent/merge-example"'
  assert_output_contains '"lock": "merge-lane"'
}

@test "merge provider receives subject and policy context" {
  init_merge_project
  write_merge_bachfile
  write_merge_provider '"pr_url": "https://example.test/pr/1", "target_branch_commit": "$current_commit",'
  write_passed_policy

  run bach run --force agent.merge-example
  assert_success
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/provider-env.txt" "subject=agent/example"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/provider-env.txt" "branch=bach/e2e-agent"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/provider-env.txt" "workspace=$E2E_PROJECT/.bach/agents/example"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/provider-env.txt" "policy=$E2E_PROJECT/.bach/artifacts/policies/"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/provider-env.txt" "/agent-example.json"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/merge-context.json" '"verdict": "passed"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/merge-test/merge-prompt.md" "Subject branch: bach/e2e-agent"
}

@test "merge completion requires outcome evidence" {
  init_merge_project
  write_merge_bachfile
  write_merge_provider ''
  write_passed_policy

  run bach run --force agent.merge-example
  assert_failure
  assert_output_contains "must include pr_url, target_branch_commit, or merge_commit evidence"
}

@test "merge requires pull-request commit evidence and exports JSON" {
  init_merge_project
  write_merge_bachfile
  write_merge_provider '"pr_url": "https://example.test/pr/1",'
  write_passed_policy

  run bach run --force agent.merge-example
  assert_failure
  assert_output_contains "pr_url evidence must include target_branch_commit or merge_commit"

  write_merge_provider '"pr_url": "https://example.test/pr/1", "target_branch_commit": "$current_commit",'
  run bach run --force agent.merge-example
  assert_success
  run_id="$(printf '%s\n' "$output" | awk '/^run / {print $2; exit}')"
  run bach runs inspect --json "$run_id"
  assert_success
  assert_output_contains '"targets"'
  assert_output_contains '"agent_reports"'
  assert_output_contains '"provider_name": "fixture"'
  assert_output_contains '"pr_url": "https://example.test/pr/1"'
  assert_output_contains '"log_path"'

  write_merge_provider '"target_branch_commit": "$current_commit", "merge_commit": "$current_commit",'
  run bach run --force agent.merge-example
  assert_success
  run_id="$(printf '%s\n' "$output" | awk '/^run / {print $2; exit}')"
  run bach runs inspect --json "$run_id"
  assert_success
  assert_output_contains '"target_branch_commit"'
  assert_output_contains '"merge_commit"'
}
