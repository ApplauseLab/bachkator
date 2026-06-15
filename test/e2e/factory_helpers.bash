load helpers

setup() {
  setup_bach
  export E2E_PROJECT="$BATS_TEST_TMPDIR/project-$BATS_TEST_NUMBER"
  mkdir -p "$E2E_PROJECT"
  export E2E_BACHFILE="$E2E_PROJECT/Bachfile"
  DAEMON_PID=""
}

teardown() {
  if [[ -n "${DAEMON_PID:-}" ]]; then
    kill -TERM "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
}

start_daemon() {
  local factory="${1:-sldc}"
  (
    cd "$E2E_PROJECT"
    exec "$BACH_BIN" -f "$E2E_BACHFILE" factory start "$factory" --yes --log-only --poll-interval 100ms --renew-interval 1h
  ) >"$E2E_PROJECT/daemon.log" 2>&1 &
  DAEMON_PID=$!
}

wait_for_status() {
  local factory="$1"
  local predicate="$2"
  local attempts="${3:-100}"
  local i
  for ((i = 0; i < attempts; i++)); do
    local out
    out="$(cd "$E2E_PROJECT" && "$BACH_BIN" -f "$E2E_BACHFILE" factory status "$factory" --json 2>&1)" || true
    if eval "$predicate"; then
      return 0
    fi
    sleep 0.1
  done
  echo "timed out waiting for status predicate" >&2
  echo "$out" >&2
  if [[ -f "$E2E_PROJECT/daemon.log" ]]; then
    echo "--- daemon.log ---" >&2
    cat "$E2E_PROJECT/daemon.log" >&2
  fi
  return 1
}

wait_for_item() {
  local factory="$1"
  local id="$2"
  local predicate="$3"
  local attempts="${4:-100}"
  local i
  for ((i = 0; i < attempts; i++)); do
    local out
    out="$(cd "$E2E_PROJECT" && "$BACH_BIN" -f "$E2E_BACHFILE" factory inspect "$factory" "$id" --json 2>&1)" || true
    if eval "$predicate"; then
      return 0
    fi
    sleep 0.1
  done
  echo "timed out waiting for inspect predicate" >&2
  echo "$out" >&2
  if [[ -f "$E2E_PROJECT/daemon.log" ]]; then
    echo "--- daemon.log ---" >&2
    cat "$E2E_PROJECT/daemon.log" >&2
  fi
  return 1
}
