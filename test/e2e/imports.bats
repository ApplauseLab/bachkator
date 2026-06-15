#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "split Bachfile lists and runs imported target" {
  mkdir -p "$E2E_PROJECT/bach"
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "shell.test"
}

import "./bach/go.bach"
HCL
  cat >"$E2E_PROJECT/bach/go.bach" <<'HCL'
shell "test" {
  shell = "mkdir -p dist && printf imported > dist/result.txt"
}
HCL

  run bach list
  assert_success
  assert_output_contains "shell/test"

  run bach run --dry-run shell/test
  assert_success
  assert_output_contains "shell/test"

  run bach run shell/test
  assert_success
  assert_file_contains "$E2E_PROJECT/dist/result.txt" "imported"
}

@test "invalid imported target diagnostic names imported file" {
  mkdir -p "$E2E_PROJECT/bach"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

import "./bach/bad.bach"
HCL
  cat >"$E2E_PROJECT/bach/bad.bach" <<'HCL'
shell "bad" {
  timeout = "later"
  shell   = "true"
}
HCL

  run bach list
  assert_failure
  assert_output_contains "bad.bach"
}
