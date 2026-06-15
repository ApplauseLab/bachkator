#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

@test "plan batch implements independent plans in parallel" {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial

  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
printf 'implemented %s\n' "$BACH_PLAN_ID" >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "plan $BACH_PLAN_ID"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"fixture","provider_type":"agent","provider_command":["$BACH_PROJECT_ROOT/provider.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"implemented $BACH_PLAN_ID"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"

  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Implement the plan.\n' >"$E2E_PROJECT/prompts/implementer.md"

  cat >"$E2E_PROJECT/plans/a.md" <<'MD'
---
id: plan-a
title: Plan A
agent_template: feature
---

# Plan A

Update app.txt.
MD

  cat >"$E2E_PROJECT/plans/b.md" <<'MD'
---
id: plan-b
title: Plan B
agent_template: feature
---

# Plan B

Update app.txt again.
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

  run bach plan implement --yes --parallelism 2 plans/a.md plans/b.md --json
  assert_success
  assert_output_contains '"schema_version": "bach.plan_batch.v1"'
  assert_output_contains '"state": "implemented"'
  assert_output_contains '"id": "plan-a"'
  assert_output_contains '"id": "plan-b"'

  run bach plan review plans/a.md plans/b.md --json
  assert_success
  assert_output_contains '"schema_version": "bach.plan_review.v1"'
  assert_output_contains '"implemented":'
}

@test "plan batch stops on failure and blocks dependents" {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial

  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
if [ "$BACH_PLAN_ID" = "plan-fail" ]; then
  exit 1
fi
printf 'implemented %s\n' "$BACH_PLAN_ID" >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "plan $BACH_PLAN_ID"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"fixture","provider_type":"agent","provider_command":["$BACH_PROJECT_ROOT/provider.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"implemented $BACH_PLAN_ID"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"

  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Implement the plan.\n' >"$E2E_PROJECT/prompts/implementer.md"

  cat >"$E2E_PROJECT/plans/fail.md" <<'MD'
---
id: plan-fail
title: Plan Fail
agent_template: feature
---

# Plan Fail

This plan fails.
MD

  cat >"$E2E_PROJECT/plans/dependent.md" <<'MD'
---
id: plan-dependent
title: Plan Dependent
depends_on: [plan-fail]
agent_template: feature
---

# Plan Dependent

Depends on failing plan.
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

  run bach plan implement --yes plans/fail.md plans/dependent.md --json
  assert_success
  assert_output_contains '"state": "failed"'
  assert_output_contains '"state": "blocked"'

  run bach plan review plans/fail.md plans/dependent.md --json
  assert_success
  assert_output_contains '"failed":'
  assert_output_contains '"blocked":'
}

@test "plan batch continues independent plans with stop-on never" {
  git -C "$E2E_PROJECT" init -q
  git -C "$E2E_PROJECT" config user.email e2e@example.com
  git -C "$E2E_PROJECT" config user.name "E2E Agent"
  printf 'base\n' >"$E2E_PROJECT/app.txt"
  git -C "$E2E_PROJECT" add app.txt
  git -C "$E2E_PROJECT" commit -q -m initial

  cat >"$E2E_PROJECT/provider.sh" <<'SH'
#!/usr/bin/env sh
set -eu
if [ "$BACH_PLAN_ID" = "plan-fail" ]; then
  exit 1
fi
printf 'implemented %s\n' "$BACH_PLAN_ID" >>app.txt
git add app.txt
git -c user.email=e2e@example.com -c user.name='E2E Agent' commit -q -m "plan $BACH_PLAN_ID"
commit="$(git rev-parse HEAD)"
branch="$(git branch --show-current)"
cat >"$BACH_AGENT_REPORT_PATH" <<JSON
{"target":"$BACH_AGENT_TARGET","provider_name":"fixture","provider_type":"agent","provider_command":["$BACH_PROJECT_ROOT/provider.sh"],"mode":"implement","status":"passed","attempt":1,"workspace":"$BACH_AGENT_WORKSPACE","branch":"$branch","commit":"$commit","changed_files":["app.txt"],"summary":"implemented $BACH_PLAN_ID"}
JSON
SH
  chmod +x "$E2E_PROJECT/provider.sh"

  mkdir -p "$E2E_PROJECT/prompts" "$E2E_PROJECT/plans"
  printf 'Implement the plan.\n' >"$E2E_PROJECT/prompts/implementer.md"

  cat >"$E2E_PROJECT/plans/fail.md" <<'MD'
---
id: plan-fail
title: Plan Fail
agent_template: feature
---

# Plan Fail

This plan fails.
MD

  cat >"$E2E_PROJECT/plans/independent.md" <<'MD'
---
id: plan-independent
title: Plan Independent
agent_template: feature
---

# Plan Independent

Independent of failing plan.
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

  run bach plan implement --yes --stop-on never plans/fail.md plans/independent.md --json
  assert_success
  assert_output_contains '"state": "failed"'
  assert_output_contains '"state": "implemented"'
}
