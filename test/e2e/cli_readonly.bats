#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "version and reference commands do not require a project" {
  run "$BACH_BIN" --version
  assert_success
  assert_output_contains "bach "

  run "$BACH_BIN" reference shell-targets
  assert_success
  assert_output_contains "shell"
}

@test "list, aliases, explain, graph, and affected expose project metadata" {
  mkdir -p "$E2E_PROJECT/src"
  printf 'hello\n' >"$E2E_PROJECT/src/app.txt"
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "old-build"
  state   = ".bach/state.db"
}

alias "old-build" {
  target      = "shell.build"
  deprecated = "Use shell/build."
}

shell "prepare" {
  description = "Prepare inputs"
  command     = ["true"]
}

shell "build" {
  description = "Build app"
  depends_on  = [shell.prepare]
  inputs      = ["src/app.txt"]
  outputs     = ["dist/app.txt"]
  lock        = "builder"
  command     = ["sh", "-c", "mkdir -p dist && cp src/app.txt dist/app.txt"]
}
HCL

  run bach list --aliases
  assert_success
  assert_output_contains "shell/build"
  assert_output_contains "old-build -> shell/build"

  run bach --verbose list
  assert_success
  assert_output_contains "TARGET"
  assert_output_contains "shell/build"

  run bach explain old-build
  assert_success
  assert_output_contains "shell/build"
  assert_output_contains "shell/prepare"

  run bach --format json graph
  assert_success
  assert_output_contains '"name": "shell/build"'
  assert_output_contains '"lock": "builder"'
  assert_output_contains '"type": "depends_on"'

  run bach affected src/app.txt
  assert_success
  assert_output_contains "shell/build 1 src/app.txt"
}
