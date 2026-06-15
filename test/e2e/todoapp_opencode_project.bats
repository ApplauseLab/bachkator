#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
  cp -R "$BACH_REPO_ROOT/test/fixtures/todoapp-opencode/." "$E2E_PROJECT/"
  chmod +x "$E2E_PROJECT/scripts/opencode-provider.sh" "$E2E_PROJECT/scripts/run-bats.sh"
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.test
  git -C "$E2E_PROJECT" config user.name "Todo OpenCode E2E"
  git -C "$E2E_PROJECT" add .
  git -C "$E2E_PROJECT" commit -q -m initial
}

@test "todo app OpenCode Bachfile exposes real agent workflow" {
  run bach list --generated
  assert_success
  assert_output_contains "agent/build-todoapp"
  assert_output_contains "agent/merge-todoapp"
  assert_output_contains "policy/acceptance"

  run bach run --dry-run agent.build-todoapp
  assert_success
  assert_output_contains "opencode-provider.sh"
}

@test "todo app OpenCode workflow can build real app when explicitly enabled" {
  if [[ "${BACH_RUN_OPENCODE_E2E:-}" != "1" ]]; then
    skip "set BACH_RUN_OPENCODE_E2E=1 to run the real OpenCode todo app workflow"
  fi

  run env BACH_OPENCODE_SKIP_PERMISSIONS=1 bash -c "cd '$E2E_PROJECT' && '$BACH_BIN' -f '$E2E_BACHFILE' run agent.build-todoapp"
  assert_success
  assert_file_contains "$E2E_PROJECT/.bach/agents/build-todoapp/.bach/artifacts/todo-bats.passed" "passed"

  run env BACH_OPENCODE_SKIP_PERMISSIONS=1 bash -c "cd '$E2E_PROJECT' && '$BACH_BIN' -f '$E2E_BACHFILE' run agent.merge-todoapp"
  assert_success

  run bach run shell.test
  assert_success
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/todo-bats.passed" "passed"
}
