#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "run executes default alias, records runs, and lists artifacts" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
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

shell "build" {
  shell   = "mkdir -p dist && cp input.txt dist/app.txt && printf manifest > \"$BACH_RUN_DIRECTORY/deploy.yaml\""
  inputs  = ["input.txt"]
  outputs = ["dist/app.txt"]
}
HCL

  run bach run old-build
  assert_success
  assert_output_contains "alias \"old-build\" resolves to \"shell/build\""
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "one"

  run bach runs --target shell/build
  assert_success
  assert_output_contains "success"
  assert_output_contains "shell/build"

  run bach artifacts --target shell/build
  assert_success
  assert_output_contains "artifact"
  assert_output_contains "dist/app.txt"
  assert_output_contains "manifest"
}

@test "dry-run json reports plans without creating state" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

profile "staging" {
  env {
    HOST = "staging.example.com"
  }
}

shell "prepare" {
  shell = "printf prepare"
}

shell "deploy" {
  depends_on = [shell.prepare]
  shell      = "printf deploy-${HOST}"
  inputs     = ["input.txt"]
  outputs    = ["dist/app.txt"]
  lock       = "deploy"
  remote     = true
  destructive = true
  requires_confirmation = true
}
HCL

  run bach --profile staging --dry-run --json run shell/deploy
  assert_success
  assert_output_contains '"target": "shell/deploy"'
  assert_output_contains '"selected_profiles": ['
  assert_output_contains '"staging"'
  assert_output_contains '"lock": "deploy"'
  assert_output_contains '"requires_confirmation": true'

  [[ ! -e "$E2E_PROJECT/.bach/state.db" ]]
}

@test "confirmation gates block risky targets unless --yes is supplied" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "deploy" {
  requires_confirmation = true
  command               = ["sh", "-c", "printf deployed > deployed.txt"]
}
HCL

  run bach run shell/deploy
  assert_failure
  assert_output_contains "requires confirmation"
  [[ ! -e "$E2E_PROJECT/deployed.txt" ]]

  run bach --yes run shell/deploy
  assert_success
  assert_file_contains "$E2E_PROJECT/deployed.txt" "deployed"
}
