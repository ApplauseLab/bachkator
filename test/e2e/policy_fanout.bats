#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "policy required targets run in subject workspace with normal target identity" {
  mkdir -p "$E2E_PROJECT/subject"
  (cd "$E2E_PROJECT/subject" && git init >/dev/null && git config user.email e2e@example.com && git config user.name E2E && printf subject > input.txt && git add input.txt && git commit -m subject >/dev/null)
  subject_commit="$(cd "$E2E_PROJECT/subject" && git rev-parse HEAD)"
  write_bachfile <<HCL
project "e2e" {
  root  = "."
}

shell "test" {
  shell   = "printf subject-run > result.txt"
  inputs  = ["input.txt"]
  outputs = ["result.txt"]
}

group "gate" {
  targets = [shell.test]
}

policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = "$E2E_PROJECT/subject"
  subject_commit    = "$subject_commit"
  required_targets  = [group.gate]
}
HCL

  run bach run --dry-run policy.merge@agent.checkout_refactor
  assert_success
  assert_output_contains "policy.merge@agent.checkout_refactor generated policy fan-out"
  assert_output_contains "[shell/test]"
  assert_output_contains "[group/gate]"

  run bach run policy.merge@agent.checkout_refactor
  assert_success
  assert_file_contains "$E2E_PROJECT/subject/result.txt" "subject-run"
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" '"policy": "policy.merge@agent.checkout_refactor"'
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" '"status": "passed"'

  run env BACH_E2E_BIN="$BACH_BIN" bash -c "cd '$E2E_PROJECT/subject' && '$BACH_BIN' -f '$E2E_BACHFILE' runs list --target group/gate"
  assert_success
  assert_output_contains "group/gate"
}

@test "policy does not reuse main checkout cache for subject workspace" {
  printf main > "$E2E_PROJECT/input.txt"
  mkdir -p "$E2E_PROJECT/subject"
  (cd "$E2E_PROJECT/subject" && git init >/dev/null && git config user.email e2e@example.com && git config user.name E2E && printf subject > input.txt && git add input.txt && git commit -m subject >/dev/null)
  subject_commit="$(cd "$E2E_PROJECT/subject" && git rev-parse HEAD)"
  write_bachfile <<HCL
project "e2e" {
  root  = "."
}

shell "test" {
  shell   = "cp input.txt result.txt"
  inputs  = ["input.txt"]
  outputs = ["result.txt"]
}

group "gate" {
  targets = [shell.test]
}

policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = "$E2E_PROJECT/subject"
  subject_commit    = "$subject_commit"
  required_targets  = [group.gate]
}
HCL

  run bach run group/gate
  assert_success
  assert_file_contains "$E2E_PROJECT/result.txt" "main"

  run bach run policy.merge@agent.checkout_refactor
  assert_success
  assert_file_contains "$E2E_PROJECT/subject/result.txt" "subject"
}

@test "policy reports required target failure and workspace mutation finding" {
  mkdir -p "$E2E_PROJECT/subject"
  (cd "$E2E_PROJECT/subject" && git init >/dev/null && git config user.email e2e@example.com && git config user.name E2E && printf subject > input.txt && git add input.txt && git commit -m subject >/dev/null)
  subject_commit="$(cd "$E2E_PROJECT/subject" && git rev-parse HEAD)"
  write_bachfile <<HCL
project "e2e" {
  root  = "."
}

shell "mutate" {
  shell = "printf changed > changed.txt"
}

shell "fail" {
  depends_on = [shell.mutate]
  shell      = "exit 7"
}

group "gate" {
  targets = [shell.fail]
}

policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = "$E2E_PROJECT/subject"
  subject_commit    = "$subject_commit"
  required_targets  = [group.gate]
}
HCL

  run bach run policy.merge@agent.checkout_refactor
  assert_failure
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" '"status": "failed"'
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" "required_target_error"
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" "policy-required-target-mutated-workspace"
}

@test "generated policy nodes are visible only when requested" {
  mkdir -p "$E2E_PROJECT/subject"
  write_bachfile <<HCL
project "e2e" {
  root = "."
}

shell "test" {
  shell = "printf ok"
}

group "gate" {
  targets = [shell.test]
}

policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = "$E2E_PROJECT/subject"
  required_targets  = [group.gate]
}
HCL

  run bach list
  assert_success
  [[ "$output" != *"policy.merge@agent.checkout_refactor"* ]]

  run bach list --generated
  assert_success
  assert_output_contains "policy.merge@agent.checkout_refactor"

  run bach explain policy.merge@agent.checkout_refactor
  assert_success
  assert_output_contains "generated: true"
  assert_output_contains "required_targets: group/gate"
}

@test "policy fails when subject commit does not match workspace HEAD" {
  mkdir -p "$E2E_PROJECT/subject"
  (cd "$E2E_PROJECT/subject" && git init >/dev/null && git config user.email e2e@example.com && git config user.name E2E && printf subject > input.txt && git add input.txt && git commit -m subject >/dev/null)
  write_bachfile <<HCL
project "e2e" {
  root = "."
}

shell "test" {
  shell = "printf should-not-run > ran.txt"
}

group "gate" {
  targets = [shell.test]
}

policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = "subject"
  subject_commit    = "0000000000000000000000000000000000000000"
  required_targets  = [group.gate]
}
HCL

  run bach run policy.merge@agent.checkout_refactor
  assert_failure
  assert_output_contains "policy-subject-commit-mismatch"
  [[ ! -e "$E2E_PROJECT/subject/ran.txt" ]]
  assert_file_contains "$E2E_PROJECT/subject/.bach/artifacts/policy.merge_agent.checkout_refactor.json" "policy-subject-commit-mismatch"
}
