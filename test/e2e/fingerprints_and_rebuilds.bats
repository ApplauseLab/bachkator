#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

write_cache_project() {
  local command="$1"
  cat >"$E2E_BACHFILE" <<HCL
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "build" {
  shell   = "$command"
  inputs  = ["input.txt"]
  outputs = ["dist/app.txt"]
}
HCL
}

write_env_project() {
  local mode="$1"
  cat >"$E2E_BACHFILE" <<HCL
project "e2e" {
  root  = "."
  state = ".bach/state.db"
}

shell "build" {
  command = ["sh", "-c", "mkdir -p dist && printf \"$mode\" > dist/app.txt && printf 'run\\n' >> runs.txt"]
  env {
    MODE = "$mode"
  }
  inputs  = ["input.txt"]
  outputs = ["dist/app.txt"]
}
HCL
}

@test "fresh target is skipped until an input changes" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_cache_project "mkdir -p dist && cat input.txt >> dist/app.txt && printf 'run\\n' >> runs.txt"

  run bach run shell/build
  assert_success
  assert_output_contains "stale:"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  run bach run shell/build
  assert_success
  assert_output_contains "[shell/build] up to date"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  printf 'two\n' >"$E2E_PROJECT/input.txt"
  run bach run shell/build
  assert_success
  assert_output_contains "changed input"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "two"
}

@test "--force rebuilds a fresh target" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_cache_project "mkdir -p dist && cat input.txt > dist/app.txt && printf 'run\\n' >> runs.txt"

  run bach run shell/build
  assert_success
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  run bach --force run shell/build
  assert_success
  assert_output_contains "forced run"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
}

@test "operation changes invalidate the fingerprint" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_cache_project "mkdir -p dist && printf 'v1:' > dist/app.txt && cat input.txt >> dist/app.txt && printf 'run\\n' >> runs.txt"

  run bach run shell/build
  assert_success
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  write_cache_project "mkdir -p dist && printf 'v2:' > dist/app.txt && cat input.txt >> dist/app.txt && printf 'run\\n' >> runs.txt"
  run bach run shell/build
  assert_success
  assert_output_contains "changed operation"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "v2:"
}

@test "environment changes invalidate the fingerprint" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_env_project "dev"

  run bach run shell/build
  assert_success
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  write_env_project "prod"
  run bach run shell/build
  assert_success
  assert_output_contains "changed env var"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "prod"
}

@test "missing declared output rebuilds even when inputs are unchanged" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_cache_project "mkdir -p dist && cat input.txt > dist/app.txt && printf 'run\\n' >> runs.txt"

  run bach run shell/build
  assert_success
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  rm "$E2E_PROJECT/dist/app.txt"
  run bach run shell/build
  assert_success
  assert_output_contains "missing output"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
}

@test "dry-run reports stale reasons without updating cache state" {
  printf 'one\n' >"$E2E_PROJECT/input.txt"
  write_cache_project "mkdir -p dist && cat input.txt > dist/app.txt && printf 'run\\n' >> runs.txt"

  run bach run shell/build
  assert_success
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1

  printf 'two\n' >"$E2E_PROJECT/input.txt"
  run bach --dry-run run shell/build
  assert_success
  assert_output_contains "changed input"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 1
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "one"

  run bach run shell/build
  assert_success
  assert_output_contains "changed input"
  assert_line_count "$E2E_PROJECT/runs.txt" "run" 2
  assert_file_contains "$E2E_PROJECT/dist/app.txt" "two"
}
