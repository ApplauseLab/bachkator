#!/usr/bin/env bats

load factory_helpers

@test "rejected approval re-enqueues item with feedback and retry completes" {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial

  mkdir -p "$E2E_PROJECT/prompts"
  printf 'Implement the request.\n' >"$E2E_PROJECT/prompts/planner.md"
  printf 'Implement the plan.\n' >"$E2E_PROJECT/prompts/implementer.md"

  cat >"$E2E_PROJECT/planner.sh" <<'SH'
#!/usr/bin/env sh
set -eu
mkdir -p "$(dirname "$BACH_PLAN_OUTPUT_PATH")"
cat >"$BACH_PLAN_OUTPUT_PATH" <<'PLAN'
---
id: factory-request
title: Factory Request
agent_template: implementer
---

# Factory Request

Update app.txt.
PLAN
SH
  chmod +x "$E2E_PROJECT/planner.sh"

  cat >"$E2E_PROJECT/implementer.sh" <<'SH'
#!/usr/bin/env sh
set -eu
printf 'implemented\n' >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "factory implementation"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"implementer","provider_type":"agent","provider_command":["$BACH_PROJECT_ROOT/implementer.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"implemented factory request"}
JSON
SH
  chmod +x "$E2E_PROJECT/implementer.sh"

  fixture_bin="$E2E_PROJECT/bach-approval-fixture"
  (cd "$BACH_REPO_ROOT" && go build -o "$fixture_bin" ./cmd/bach-approval-fixture)

  write_bachfile <<HCL
project "e2e" {
  root = "."
}

provider "fixture" {
  type    = "agent"
  command = ["\$BACH_PROJECT_ROOT/planner.sh"]
}

provider "implementer" {
  type    = "agent"
  command = ["\$BACH_PROJECT_ROOT/implementer.sh"]
}

prompt "planner" {
  path = "prompts/planner.md"
}

prompt "implementer" {
  path = "prompts/implementer.md"
}

agent_template "planner" {
  provider = provider.fixture
  role     = "planner"
  prompt   = prompt.planner

  workspace {
    mode = "clone"
    path = ".bach/agents/factory/\${work_item.id}/plan"
  }
}

agent_template "implementer" {
  provider = provider.implementer
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    mode = "clone"
    path = ".bach/agents/factory/\${work_item.id}/implement"
  }

  git {
    branch = "bach/factory/\${work_item.id}/implement"
    commit = "required"
  }
}

factory "sldc" {
  workflow "ship" {
    plan {
      agent_template    = agent_template.planner
      path              = "plans/factory/\${work_item.id}.md"
      requires_approval = true
    }

    implement {
      agent_template = agent_template.implementer
    }
  }

  triggers {
    manual {}
  }

  approvals {
    provider "fixture" {
      command       = ["$fixture_bin"]
      poll_interval = "1s"

      config = {
        items_path = ".bach/fixtures/approvals.json"
      }
    }
  }
}
HCL

  run bach factory submit sldc --title "Ship feature" --body "Implement it" --json
  assert_success
  work_item_id="$(printf '%s\n' "$output" | awk -F'"' '/"id":/ { print $4; exit }')"
  [[ -n "$work_item_id" ]]

  start_daemon sldc

  wait_for_item sldc "$work_item_id" '[[ "$out" == *"\"lifecycle\": \"waiting_approval\""* ]]' 300

  mkdir -p "$E2E_PROJECT/.bach/fixtures"
  cat >"$E2E_PROJECT/.bach/fixtures/approvals.json" <<JSON
[
  {
    "work_item_id": "$work_item_id",
    "phase": "plan",
    "rejected": true,
    "approver": "kris",
    "reason": "use dark theme and validate titles"
  }
]
JSON

  wait_for_item sldc "$work_item_id" '[[ "$out" == *"\"lifecycle\": \"failed\""* ]]' 300

  run bach factory list sldc --status all --json
  assert_success
  retry_id="$(printf '%s\n' "$output" | jq -r --arg orig "$work_item_id" '
    .items[]
    | select(.id != $orig and .lifecycle == "pending"
             and (.body | contains("use dark theme and validate titles")))
    | .id' | head -n1)"
  [[ -n "$retry_id" ]]

  run bach factory inspect sldc "$retry_id" --json
  assert_success
  assert_output_contains '"rejection_count": "1"'
  assert_output_contains 'use dark theme and validate titles'

  cat >"$E2E_PROJECT/.bach/fixtures/approvals.json" <<JSON
[
  {
    "work_item_id": "$retry_id",
    "phase": "plan",
    "approver": "kris",
    "reason": "feedback addressed"
  }
]
JSON

  wait_for_item sldc "$retry_id" '[[ "$out" == *"\"lifecycle\": \"completed\""* ]]' 300

  run bach factory inspect sldc "$work_item_id" --json
  assert_success
  assert_output_contains '"rejected: use dark theme and validate titles'

  kill -TERM "$DAEMON_PID"
  wait "$DAEMON_PID" || true
  DAEMON_PID=""
}
