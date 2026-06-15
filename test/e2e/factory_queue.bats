#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "factory manual queue persists work items across CLI invocations" {
  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

factory "sldc" {
  workflow "ship" {}

  triggers {
    manual {}
  }
}
HCL

  run bach factory submit sldc --title "Ship billing webhook" --body "Implement it" --label billing --dedupe-key billing-webhook --json
  assert_success
  assert_output_contains '"created": true'
  assert_output_contains '"factory": "sldc"'
  assert_output_contains '"workflow": "ship"'
  work_item_id="$(printf '%s\n' "$output" | awk -F'"' '/"id":/ { print $4; exit }')"
  if [[ -z "$work_item_id" ]]; then
    echo "could not parse work item id" >&2
    echo "$output" >&2
    return 1
  fi
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/factory/$work_item_id/intake.json" '"schema_version": "bach.factory.intake.v1"'
  assert_file_contains "$E2E_PROJECT/.bach/artifacts/factory/$work_item_id/intake.json" 'Ship billing webhook'

  run bach factory submit sldc --title "Duplicate billing webhook" --body "Duplicate" --dedupe-key billing-webhook --json
  assert_success
  assert_output_contains '"created": false'
  assert_output_contains "\"id\": \"$work_item_id\""

  run bach factory list sldc --json
  assert_success
  assert_output_contains "\"id\": \"$work_item_id\""
  assert_output_contains '"lifecycle": "pending"'

  run bach factory inspect sldc "$work_item_id" --json
  assert_success
  assert_output_contains '"title": "Ship billing webhook"'
  assert_output_contains '"intake_evidence_uri"'

  run bach factory cancel sldc "$work_item_id" --reason duplicate --json
  assert_success
  assert_output_contains '"lifecycle": "cancelled"'
  assert_output_contains '"cancel_reason": "duplicate"'

  run bach factory list sldc --status cancelled --json
  assert_success
  assert_output_contains "\"id\": \"$work_item_id\""
  assert_output_contains '"lifecycle": "cancelled"'
}
