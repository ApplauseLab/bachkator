#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "plan status loads markdown plans with optional frontmatter" {
  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

shell "test" {
  command = ["true"]
}
HCL

  mkdir -p "$E2E_PROJECT/plans"
  cat >"$E2E_PROJECT/plans/base.md" <<'MD'
# Base Plan

Implement base changes.
MD
  cat >"$E2E_PROJECT/plans/followup.md" <<'MD'
---
id: followup
title: Follow-up Plan
depends_on: [plans-base]
required_targets: [shell/test]
---

# Follow-up

Implement follow-up changes.
MD

  run bach plan status plans/base.md plans/followup.md
  assert_success
  assert_output_contains 'plans-base'
  assert_output_contains 'followup'
  assert_output_contains 'Planned waves:'

  run bach plan status --json plans/base.md plans/followup.md
  assert_success
  assert_output_contains '"schema_version": "bach.plan_status.v1"'
  assert_output_contains '"id": "plans-base"'
  assert_output_contains '"id": "followup"'
}

@test "plan implement runs generated agent and records lifecycle ledgers" {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial

  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
printf 'implemented\n' >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "plan implementation"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"fixture","provider_type":"agent","provider_command":["$BACH_PROJECT_ROOT/provider.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"implemented plan"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"

  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Implement the plan.\n' >"$E2E_PROJECT/prompts/implementer.md"
  cat >"$E2E_PROJECT/plans/implement.md" <<'MD'
---
id: implement-plan
title: Implement Plan
agent_template: feature
---

# Implement Plan

Update app.txt.
MD

  write_bachfile <<'HCL'
project "e2e" {
  root = "."
}

provider "fixture" {
  type    = "agent"
  command = ["$BACH_PROJECT_ROOT/provider.sh"]
}

prompt "implementer" {
  path = "prompts/implementer.md"
}

agent_template "feature" {
  provider = provider.fixture
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    mode = "clone"
    path = ".bach/agents/plans/${plan.id}"
  }

  git {
    branch = "bach/plans/${plan.id}"
    commit = "required"
  }
}
HCL

  run bach plan implement --yes plans/implement.md --json
  assert_success
  assert_output_contains '"result": "implemented"'
  assert_output_contains '"target": "agent/plan.implement-plan"'
  assert_output_contains '"status": "pending"'
  assert_output_contains '"status": "in_progress"'
  assert_output_contains '"status": "implemented"'

  run bach plan status plans/implement.md
  assert_success
  assert_output_contains 'implement-plan'
  assert_output_contains 'implemented'

  run bach plan implement plans/implement.md --json
  assert_success
  assert_output_contains '"result": "skipped"'

  cat >"$E2E_PROJECT/plans/dependent.md" <<'MD'
---
id: dependent-plan
title: Dependent Plan
depends_on: [implement-plan]
agent_template: feature
---

# Dependent Plan

Update app.txt after the first plan.
MD

  run bach plan implement --yes plans/dependent.md --json
  assert_success
  assert_output_contains '"result": "implemented"'
  assert_output_contains '"target": "agent/plan.dependent-plan"'
}
