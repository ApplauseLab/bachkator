#!/bin/sh
set -eu

BATS_BIN=${BATS_BIN:-tools/bats-core/bin/bats}
export BACH_E2E_BIN=${BACH_E2E_BIN:-"$(pwd)/dist/bach"}
RUN_DIRECTORY=${BACH_RUN_DIRECTORY:-${RUN_DIRECTORY:-.bach/runs/e2e-local}}
JUNIT_PATH=${BATS_JUNIT_PATH:-"$RUN_DIRECTORY/bats-junit.xml"}
JUNIT_WORK_DIR="$RUN_DIRECTORY/bats-junit"

if [ ! -x "$BATS_BIN" ]; then
  printf 'missing bats executable at %s\n' "$BATS_BIN" >&2
  exit 1
fi

if [ ! -x "$BACH_E2E_BIN" ]; then
  printf 'missing bach executable at %s\n' "$BACH_E2E_BIN" >&2
  exit 1
fi

failures=0
pids=""

mkdir -p "$JUNIT_WORK_DIR"
rm -f "$JUNIT_PATH"

run_file() {
  file=$1
  log=$2
  report_dir=$3
  mkdir -p "$report_dir"
  "$BATS_BIN" --formatter tap --report-formatter junit -o "$report_dir" "$file" >"$log" 2>&1
}

wait_one() {
  pid=$1
  log=$2
  if ! wait "$pid"; then
    failures=$((failures + 1))
  fi
  cat "$log"
  rm -f "$log"
}

for file in test/e2e/*.bats; do
  log=$(mktemp "${TMPDIR:-/tmp}/bach-bats.XXXXXX")
  report_name=$(basename "$file" .bats)
  report_dir="$JUNIT_WORK_DIR/$report_name"
  run_file "$file" "$log" "$report_dir" &
  pid=$!
  pids="$pids $pid:$log"
done

for item in $pids; do
  wait_one "${item%%:*}" "${item#*:}"
done

{
  printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>'
  printf '%s\n' '<testsuites>'
  for report in "$JUNIT_WORK_DIR"/*/report.xml; do
    while IFS= read -r line; do
      case "$line" in
        '<?xml '*|'<testsuites '*|'<testsuites>'|'</testsuites>') continue ;;
      esac
      printf '%s\n' "$line"
    done <"$report"
  done
  printf '%s\n' '</testsuites>'
} >"$JUNIT_PATH"

if [ "$failures" -ne 0 ]; then
  printf '%s bats file(s) failed\n' "$failures" >&2
  exit 1
fi
