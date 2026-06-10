#!/usr/bin/env bash

set -euo pipefail

e2e_repo_root() {
  cd "${BATS_TEST_DIRNAME}/../.." && pwd
}

setup_bach() {
  BACH_REPO_ROOT="$(e2e_repo_root)"
  BACH_BIN="${BACH_E2E_BIN:-${BACH_REPO_ROOT}/dist/bach}"
  if [[ ! -x "$BACH_BIN" ]]; then
    BACH_BIN="${BACH_REPO_ROOT}/dist/bach"
  fi
  if [[ ! -x "$BACH_BIN" ]]; then
    echo "missing executable Bach binary at $BACH_BIN; run: go run ./cmd/bach run shell/build" >&2
    return 1
  fi
}

make_project() {
  export E2E_PROJECT="$BATS_TEST_TMPDIR/project"
  mkdir -p "$E2E_PROJECT"
  export E2E_BACHFILE="$E2E_PROJECT/Bachfile"
}

write_bachfile() {
  cat >"$E2E_BACHFILE"
}

bach() {
  (cd "$E2E_PROJECT" && "$BACH_BIN" -f "$E2E_BACHFILE" "$@")
}

assert_success() {
  if [[ "$status" -ne 0 ]]; then
    echo "expected success, got status $status" >&2
    echo "$output" >&2
    return 1
  fi
}

assert_failure() {
  if [[ "$status" -eq 0 ]]; then
    echo "expected failure, got success" >&2
    echo "$output" >&2
    return 1
  fi
}

assert_output_contains() {
  local needle="$1"
  if [[ "$output" != *"$needle"* ]]; then
    echo "expected output to contain: $needle" >&2
    echo "$output" >&2
    return 1
  fi
}

assert_file_contains() {
  local path="$1"
  local needle="$2"
  if [[ ! -f "$path" ]]; then
    echo "missing file: $path" >&2
    return 1
  fi
  if [[ "$(<"$path")" != *"$needle"* ]]; then
    echo "expected $path to contain: $needle" >&2
    printf '%s\n' "$(<"$path")" >&2
    return 1
  fi
}

assert_line_before() {
  local path="$1"
  local before="$2"
  local after="$3"
  local before_line=0
  local after_line=0
  local line_number=0
  local line
  while IFS= read -r line; do
    line_number=$((line_number + 1))
    if [[ "$line" == "$before" && "$before_line" -eq 0 ]]; then
      before_line=$line_number
    fi
    if [[ "$line" == "$after" && "$after_line" -eq 0 ]]; then
      after_line=$line_number
    fi
  done <"$path"
  if [[ "$before_line" -eq 0 || "$after_line" -eq 0 || "$before_line" -ge "$after_line" ]]; then
    echo "expected $before before $after in $path" >&2
    printf '%s\n' "$(<"$path")" >&2
    return 1
  fi
}

assert_line_count() {
  local path="$1"
  local needle="$2"
  local want="$3"
  local count=0
  local line
  while IFS= read -r line; do
    if [[ "$line" == "$needle" ]]; then
      count=$((count + 1))
    fi
  done <"$path"
  if [[ "$count" -ne "$want" ]]; then
    echo "expected $needle to appear $want time(s) in $path, got $count" >&2
    printf '%s\n' "$(<"$path")" >&2
    return 1
  fi
}
