#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

run_id_from_output() {
  while IFS= read -r line; do
    case "$line" in
      run\ *)
        set -- $line
        printf '%s\n' "$2"
        return 0
        ;;
    esac
  done <<<"$output"
  return 1
}

@test "failed command still records JUnit quality evidence for inspection" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "test" {
  shell = "mkdir -p .bach/artifacts && printf '%s\n' '<testsuite tests=\"1\" failures=\"1\" errors=\"0\" skipped=\"0\" time=\"0.01\"><testcase classname=\"Example\" name=\"fails\" time=\"0.01\"><failure message=\"nope\">failed</failure></testcase></testsuite>' > .bach/artifacts/junit.xml && exit 1"
  outputs = {
    junit = ".bach/artifacts/junit.xml"
  }
}

quality "shell.test" {
  junit {
    path   = shell.test.outputs.junit
    format = "junit-xml"
  }
}
HCL

  run bach --log-only run shell/test
  assert_failure
  assert_output_contains "quality report .bach/artifacts/junit.xml parsed"
  run_id="$(run_id_from_output)"

  run bach --json runs inspect "$run_id"
  assert_success
  assert_output_contains '"exit_code": 1'
  assert_output_contains '"path": ".bach/artifacts/junit.xml"'
  assert_output_contains '"parsed": true'
}

@test "logs command slices failed target logs" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "fail" {
  shell = "printf 'one\ntwo error\nthree\n' && exit 1"
}
HCL

  run bach run shell/fail
  assert_failure
  run_id="$(run_id_from_output)"

  run bach logs "$run_id" --failed --last 1
  assert_success
  assert_output_contains "three"
}

@test "runs inspect includes declared preflight fix" {
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "deploy" {
  shell = "true"
  preflights = [
    { name = "docker", kind = "session", command = ["sh", "-c", "exit 1"], fix = "Start Docker Desktop." },
  ]
}
HCL

  run bach run shell/deploy
  assert_failure
  run_id="$(run_id_from_output)"

  run bach --json runs inspect "$run_id"
  assert_success
  assert_output_contains "Start Docker Desktop."
}
