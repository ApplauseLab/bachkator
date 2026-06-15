#!/usr/bin/env bats

load factory_helpers

@test "factory provider trigger ingests and routes work items" {
  git -C "$E2E_PROJECT" init -q

  fixture_bin="$E2E_PROJECT/bach-trigger-fixture"
  (cd "$BACH_REPO_ROOT" && go build -o "$fixture_bin" ./cmd/bach-trigger-fixture)
  [[ -x "$fixture_bin" ]]

  items_path="$E2E_PROJECT/items.json"
  cursor_path="$E2E_PROJECT/cursor.txt"

  printf '[{"source":{"type":"issue","id":"1"},"title":"First issue","labels":["ship"]}]\n' >"$items_path"

  write_bachfile <<HCL
project "e2e" {
  root = "."
}

factory "sldc" {
  workflow "ship" {}
  workflow "hotfix" {}

  triggers {
    provider "fixture" {
      command = ["$fixture_bin"]
      poll_interval = "100ms"
      config = {
        items_path = "$items_path"
        cursor_path = "$cursor_path"
      }

      route {
        label    = "ship"
        workflow = "ship"
      }

      route {
        label    = "hotfix"
        workflow = "hotfix"
      }
    }
  }
}
HCL

  start_daemon sldc

  local work_item_id=""
  for ((i = 0; i < 100; i++)); do
    out="$(cd "$E2E_PROJECT" && "$BACH_BIN" -f "$E2E_BACHFILE" factory list sldc --status all --json 2>&1)" || true
    work_item_id="$(printf '%s\n' "$out" | awk -F'"' '/"id":/ { print $4; exit }')"
    if [[ -n "$work_item_id" ]]; then
      break
    fi
    sleep 0.1
  done
  [[ -n "$work_item_id" ]]

  run bach factory inspect sldc "$work_item_id" --json
  assert_success
  assert_output_contains '"title": "First issue"'
  assert_output_contains '"workflow": "ship"'
  assert_output_contains '"source_type": "issue"'

  printf '[{"source":{"type":"issue","id":"2"},"title":"Hotfix issue","labels":["hotfix"]}]\n' >"$items_path"

  local second_id=""
  for ((i = 0; i < 100; i++)); do
    out="$(cd "$E2E_PROJECT" && "$BACH_BIN" -f "$E2E_BACHFILE" factory list sldc --status all --json 2>&1)" || true
    second_id="$(printf '%s\n' "$out" | awk -F'"' '/"id":/ { print $4; exit }')"
    if [[ "$second_id" != "$work_item_id" && -n "$second_id" ]]; then
      break
    fi
    second_id=""
    sleep 0.1
  done
  [[ -n "$second_id" ]]

  run bach factory inspect sldc "$second_id" --json
  assert_success
  assert_output_contains '"title": "Hotfix issue"'
  assert_output_contains '"workflow": "hotfix"'

  kill -TERM "$DAEMON_PID"
  wait "$DAEMON_PID" || true
  DAEMON_PID=""
}
