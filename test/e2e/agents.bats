#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

init_agent_repo() {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial
}

write_success_provider() {
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
prompt_path="${1:?missing prompt path}"
artifact_dir="$BACH_PROJECT_ROOT/.bach/artifacts/agent-fixture"
mkdir -p "$artifact_dir"
printf '%s\n' "$@" >"$artifact_dir/provider-argv.txt"
cp "$prompt_path" "$artifact_dir/generated-prompt.md"
cp "$BACH_AGENT_CONTEXT_PATH" "$artifact_dir/generated-context.json"
count_file="$artifact_dir/provider-count.txt"
count=0
if [ -f "$count_file" ]; then
  count="$(cat "$count_file")"
fi
count=$((count + 1))
printf '%s\n' "$count" >"$count_file"
printf 'change %s\n' "$count" >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "agent change $count"
commit="$(git rev-parse HEAD)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{
  "target": "agent/example",
  "provider_name": "fixture",
  "provider_type": "agent",
  "provider_command": ["\$BACH_PROJECT_ROOT/provider.sh"],
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "$BACH_AGENT_WORKSPACE",
  "branch": "bach/e2e-agent",
  "commit": "$commit",
  "changed_files": ["app.txt"],
  "summary": "changed app"
}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
}

write_common_agent_files() {
  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Follow the fixture rules.\n' >"$E2E_PROJECT/prompts/implementer.md"
  printf 'Update app.txt from the fixture plan.\n' >"$E2E_PROJECT/plans/example.md"
}

write_success_bachfile() {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

provider "fixture" {
  type    = "agent"
  command = ["$BACH_PROJECT_ROOT/provider.sh"]
}

prompt "implementer" {
  path = "prompts/implementer.md"
}

agent "example" {
  mode     = "implement"
  provider = provider.fixture
  role     = "implementer"
  prompt   = prompt.implementer
  plan     = "plans/example.md"

  workspace {
    mode = "clone"
    path = ".bach/agents/example"
  }

  git {
    branch = "bach/e2e-agent"
    commit = "required"
  }
}
HCL
}

@test "agent provider receives prompt path, writes artifacts, commits, and reuses clone" {
  init_agent_repo
  write_common_agent_files
  write_success_provider
  write_success_bachfile

  run bach run --force agent.example
  assert_success
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/provider-argv.txt" "$E2E_PROJECT/.bach/runs/"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-prompt.md" "Plan path:"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-prompt.md" "Workspace: $E2E_PROJECT/.bach/agents/example"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-prompt.md" "Report path:"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-prompt.md" "Context path:"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-prompt.md" "Commit instructions: create at least one git commit"
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"mode": "implement"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"attempt": 1'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"provider": "fixture"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"prompt": "prompts/implementer.md"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"plan": "plans/example.md"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"workspace":'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"branch": "bach/e2e-agent"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/generated-context.json" '"report_path":'
  [[ -d "$E2E_PROJECT/.bach/agents/example/.git" ]]

  first_clone_git_dir="$(git -C "$E2E_PROJECT/.bach/agents/example" rev-parse --git-dir)"
  run bach run --force agent.example
  assert_success
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/agent-fixture/provider-count.txt" "2"
  second_clone_git_dir="$(git -C "$E2E_PROJECT/.bach/agents/example" rev-parse --git-dir)"
  [[ "$first_clone_git_dir" == "$second_clone_git_dir" ]]
}

@test "dirty reused agent clone fails at run start" {
  init_agent_repo
  write_common_agent_files
  write_success_provider
  write_success_bachfile

  run bach run --force agent.example
  assert_success
  printf 'dirty\n' >"$E2E_PROJECT/.bach/agents/example/dirty.txt"

  run bach run --force agent.example
  assert_failure
  assert_output_contains "is dirty"
}

@test "missing and malformed agent reports fail completion" {
  init_agent_repo
  write_common_agent_files
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
printf 'change\n' >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m change
SH
  chmod +x "$E2E_PROJECT/provider.sh"
  write_success_bachfile

  run bach run --force agent.example
  assert_failure
  assert_output_contains "agent report"
  assert_output_contains "is missing"

  rm -rf "$E2E_PROJECT/.bach/agents/example"
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
printf 'change\n' >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m change
printf 'not-json\n' >"$BACH_AGENT_REPORT_PATH"
SH
  chmod +x "$E2E_PROJECT/provider.sh"

  run bach run --force agent.example
  assert_failure
  assert_output_contains "invalid JSON"
}

@test "required agent commit is enforced" {
  init_agent_repo
  write_common_agent_files
  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
commit="$(git rev-parse HEAD)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{
  "target": "agent/example",
  "provider_name": "fixture",
  "provider_type": "agent",
  "provider_command": ["\$BACH_PROJECT_ROOT/provider.sh"],
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "$BACH_AGENT_WORKSPACE",
  "branch": "bach/e2e-agent",
  "commit": "$commit",
  "changed_files": [],
  "summary": "no commit"
}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"
  write_success_bachfile

  run bach run --force agent.example
  assert_failure
  assert_output_contains "requires provider to create a commit"
}
