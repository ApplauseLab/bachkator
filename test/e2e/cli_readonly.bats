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

@test "validate reports human and json diagnostics" {
  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

shell "test" {
  command = ["true"]
}
HCL

  run bach validate
  assert_success
  assert_output_contains "Bachfile valid: 1 targets, 0 aliases, 0 inputs, 0 profiles"

  write_bachfile <<'HCL'
project "e2e" {
  default = "shell/test"
}

shell "test" {
  command = ["true"]
}
HCL

  run bach validate
  assert_failure
  assert_output_contains "obsolete target reference"

  run bach validate --json
  assert_failure
  assert_output_contains '"valid": false'
  assert_output_contains '"code": "obsolete-target-reference"'
}

@test "validate accepts agent templates inherited by concrete agents" {
  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Random validation prompt for templated agents.\n' >"$E2E_PROJECT/prompts/random-template.md"
  printf 'Validate template inheritance only.\n' >"$E2E_PROJECT/plans/random-template.md"
  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

prompt "random_template" {
  path = "prompts/random-template.md"
}

agent_template "random_implementer" {
  mode     = "implement"
  provider = provider.fixture
  role     = "random-validator"
  prompt   = prompt.random_template

  workspace {
    path = ".bach/agents/random-template"
  }

  git {
    branch = "bach/random-template"
    commit = "optional"
  }
}

agent "random_validation" {
  template = agent_template.random_implementer
  plan     = "plans/random-template.md"
}
HCL

  run bach validate
  assert_success
  assert_output_contains "Bachfile valid: 1 targets, 0 aliases, 0 inputs, 0 profiles"

  run bach list
  assert_success
  assert_output_contains "agent/random_validation"
  if [[ "$output" == *"agent_template"* || "$output" == *"random_implementer"* ]]; then
    echo "agent templates should stay hidden from bach list" >&2
    echo "$output" >&2
    return 1
  fi
}

@test "list, aliases, explain, graph, affected, and provenance expose project metadata" {
  mkdir -p "$E2E_PROJECT/src"
  printf 'hello\n' >"$E2E_PROJECT/src/app.txt"
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "old-build"
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

  run bach list --verbose
  assert_success
  assert_output_contains "TARGET"
  assert_output_contains "shell/build"

  run bach explain old-build
  assert_success
  assert_output_contains "shell/build"
  assert_output_contains "shell/prepare"

  run bach graph --format json
  assert_success
  assert_output_contains '"name": "shell/build"'
  assert_output_contains '"lock": "builder"'
  assert_output_contains '"type": "depends_on"'

  run bach affected src/app.txt
  assert_success
  assert_output_contains "shell/build 1 src/app.txt"

  run bach provenance dist/app.txt
  assert_success
  assert_output_contains "generated: true"
  assert_output_contains "- shell/build"
  assert_output_contains "regenerate: bach run shell/build"

  run bach provenance --json src/app.txt
  assert_success
  assert_output_contains '"path": "src/app.txt"'
  assert_output_contains '"target": "shell/build"'

  run bach provenance unknown.txt
  assert_success
  assert_output_contains "note: no declared producers or consumers"
}

@test "split Bachfile imports can list dry-run and run targets" {
  mkdir -p "$E2E_PROJECT/bach"
  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

import "./bach/go.bach"
HCL
  cat >"$E2E_PROJECT/bach/go.bach" <<'HCL'
shell "test" {
  command = ["sh", "-c", "mkdir -p dist && printf imported > dist/result.txt"]
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

@test "imported Bachfile diagnostics name imported source" {
  mkdir -p "$E2E_PROJECT/bach"
  write_bachfile <<'HCL'
project "e2e" {}

import "./bach/bad.bach"
HCL
  cat >"$E2E_PROJECT/bach/bad.bach" <<'HCL'
shell "bad" {
  command = ["true"]
HCL

  run bach list
  assert_failure
  assert_output_contains "bach/bad.bach"
}

@test "provenance reports producers consumers json and unknown paths" {
  mkdir -p "$E2E_PROJECT/src"
  printf 'hello\n' >"$E2E_PROJECT/src/app.txt"
  write_bachfile <<'HCL'
project "e2e" {
  root    = "."
  default = "shell.build"
}

shell "build" {
  inputs  = ["src"]
  outputs = ["dist/app.txt"]
  command = ["sh", "-c", "mkdir -p dist && cp src/app.txt dist/app.txt"]
}

shell "test" {
  inputs  = ["dist"]
  command = ["true"]
}
HCL

  run bach provenance dist/app.txt
  assert_success
  assert_output_contains "generated: true"
  assert_output_contains "shell/build"
  assert_output_contains "regenerate: bach run shell/build"
  assert_output_contains "shell/test"

  run bach provenance --json src/app.txt
  assert_success
  assert_output_contains '"path": "src/app.txt"'
  assert_output_contains '"source": true'
  assert_output_contains '"target": "shell/build"'

  run bach provenance README.md
  assert_success
  assert_output_contains "generated: false"
  assert_output_contains "note: no declared producers or consumers"
}
