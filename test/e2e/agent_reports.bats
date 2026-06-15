#!/usr/bin/env bats

load helpers

setup() {
  setup_bach
  make_project
}

write_agent_project_files() {
  mkdir -p "$E2E_PROJECT/prompts/agents" "$E2E_PROJECT/scripts"
  printf 'implementer prompt\n' >"$E2E_PROJECT/prompts/agents/implementer.md"
  cat >"$E2E_PROJECT/scripts/write-agent-reports.sh" <<'SH'
#!/bin/sh
set -eu
mkdir -p .bach/artifacts/agents
cat >.bach/artifacts/agents/implementer.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"implementer","name":"fixture"},"subject":{"target":"shell/agent-fixture"},"status":"success","summary":"implemented fixture","metrics":[{"name":"implementation.changed_files.count","value":2,"unit":"count"}],"findings":[]}
JSON
cat >.bach/artifacts/agents/architecture.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"architecture-reviewer","name":"fixture"},"subject":{"target":"shell/agent-fixture"},"status":"success","summary":"architecture approved","metrics":[],"findings":[]}
JSON
cat >.bach/artifacts/agents/docs.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"docs-sweeper","name":"fixture"},"subject":{"target":"shell/agent-fixture"},"status":"success","summary":"docs current","metrics":[],"findings":[]}
JSON
cat >.bach/artifacts/agents/security.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"security-reviewer","name":"fixture"},"subject":{"target":"shell/agent-fixture"},"status":"success","summary":"security approved","metrics":[],"findings":[]}
JSON
printf 'org=1\n' >.bach/artifacts/org-policy.txt
SH
  chmod +x "$E2E_PROJECT/scripts/write-agent-reports.sh"
  cat >"$E2E_PROJECT/scripts/parse-org-policy.sh" <<'SH'
#!/bin/sh
set -eu
cat <<'JSON'
{"metrics":[{"name":"policy.org.required_reviews.count","value":3,"unit":"count"}],"findings":[{"kind":"org-policy","severity":"info","rule":"reviewers-present","message":"required reviewers reported"}]}
JSON
SH
  chmod +x "$E2E_PROJECT/scripts/parse-org-policy.sh"
}

@test "agent report policy parses implementation and reviewer evidence" {
  write_agent_project_files
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

prompt "implementer" {
  path        = "prompts/agents/implementer.md"
  description = "Fixture implementer prompt"
  version     = "v1"
}

plugin "org_policy" {
  type    = "quality"
  command = ["sh", "scripts/parse-org-policy.sh"]
}

shell "agent-fixture" {
  command = ["sh", "scripts/write-agent-reports.sh"]
}

quality "shell.agent-fixture" {
  reports = [
    { kind = "agent", format = "agent-report-v1", path = ".bach/artifacts/agents/implementer.json" },
    { kind = "architecture", format = "agent-report-v1", path = ".bach/artifacts/agents/architecture.json" },
    { kind = "docs", format = "agent-report-v1", path = ".bach/artifacts/agents/docs.json" },
    { kind = "security", format = "agent-report-v1", path = ".bach/artifacts/agents/security.json" },
    { kind = "org-policy", parser = "org_policy", path = ".bach/artifacts/org-policy.txt" },
  ]

  quality_gate {
    metric = "agent.implementer.status.success"
    min    = 1
  }

  quality_gate {
    metric = "policy.docs_sweeper.blocking_findings.count"
    max    = 0
  }

  quality_gate {
    metric = "policy.org.required_reviews.count"
    min    = 3
  }
}
HCL

  run bach validate
  assert_success

  run bach run --log-only shell/agent-fixture
  assert_success
  assert_output_contains "quality report .bach/artifacts/agents/implementer.json parsed: metrics=6 findings=0"
  assert_output_contains "quality report .bach/artifacts/org-policy.txt parsed: metrics=1 findings=1"
  policy_files=("$E2E_PROJECT"/.bach/artifacts/policies/*/shell-agent-fixture.json)
  assert_file_contains "${policy_files[0]}" '"schema": "bach.applied_policy.v1"'
  assert_file_contains "${policy_files[0]}" '"verdict": "passed"'

  run bach quality metrics
  assert_success
  assert_output_contains "agent.implementer.status.success"
  assert_output_contains "policy.org.required_reviews.count"
}

@test "duplicate default and custom metrics fail policy evaluation" {
  mkdir -p "$E2E_PROJECT/prompts/agents" "$E2E_PROJECT/scripts"
  printf 'implementer prompt\n' >"$E2E_PROJECT/prompts/agents/implementer.md"
  cat >"$E2E_PROJECT/scripts/write-reports.sh" <<'SH'
#!/bin/sh
set -eu
mkdir -p .bach/artifacts
cat >.bach/artifacts/implementer.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"implementer"},"status":"success","metrics":[],"findings":[]}
JSON
printf 'collision=1\n' >.bach/artifacts/collision.txt
SH
  chmod +x "$E2E_PROJECT/scripts/write-reports.sh"
  cat >"$E2E_PROJECT/scripts/parse-collision.sh" <<'SH'
#!/bin/sh
set -eu
cat <<'JSON'
{"metrics":[{"name":"agent.implementer.report.count","value":1,"unit":"count"}],"findings":[]}
JSON
SH
  chmod +x "$E2E_PROJECT/scripts/parse-collision.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

prompt "implementer" {
  path = "prompts/agents/implementer.md"
}

plugin "collision" {
  type    = "quality"
  command = ["sh", "scripts/parse-collision.sh"]
}

shell "agent-fixture" {
  command = ["sh", "scripts/write-reports.sh"]
}

quality "shell.agent-fixture" {
  reports = [
    { kind = "agent", format = "agent-report-v1", path = ".bach/artifacts/implementer.json" },
    { kind = "collision", parser = "collision", path = ".bach/artifacts/collision.txt" },
  ]
}
HCL

  run bach run --log-only shell/agent-fixture
  assert_failure
  assert_output_contains "policy-metric-collision"
  assert_output_contains "quality-failed=1"
}

@test "bach report helper output flows through quality ingestion" {
  mkdir -p "$E2E_PROJECT/scripts"
  cat >"$E2E_PROJECT/scripts/write-bach-report.sh" <<SH
#!/bin/sh
set -eu
export BACH_AGENT_QUALITY_REPORT_PATH="\$BACH_RUN_DIRECTORY/docs.json"
"$BACH_BIN" report finding --role docs-sweeper --name fixture --kind docs --severity error --rule stale-reference --message "CLI docs are stale"
SH
  chmod +x "$E2E_PROJECT/scripts/write-bach-report.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

shell "docs-review" {
  command = ["sh", "scripts/write-bach-report.sh"]
}

quality "shell.docs-review" {
  reports = [
    { kind = "docs", format = "agent-report-v1", path = "$(RUN_DIRECTORY)/docs.json" },
  ]

  quality_gate {
    metric = "policy.docs_sweeper.blocking_findings.count"
    max    = 0
  }
}
HCL

  run bach run --log-only shell/docs-review
  assert_failure
  assert_output_contains 'quality report $(RUN_DIRECTORY)/docs.json parsed: metrics=5 findings=1'
  assert_output_contains "policy.docs_sweeper.blocking_findings.count actual 1.000 must be <= 0.000"

  run bach quality findings
  assert_success
  assert_output_contains "stale-reference"
}

@test "docs sweeper stale docs finding blocks applied policy" {
  mkdir -p "$E2E_PROJECT/prompts/agents" "$E2E_PROJECT/scripts"
  printf 'implementer prompt\n' >"$E2E_PROJECT/prompts/agents/implementer.md"
  cat >"$E2E_PROJECT/scripts/write-docs-report.sh" <<'SH'
#!/bin/sh
set -eu
mkdir -p .bach/artifacts
cat >.bach/artifacts/docs.json <<'JSON'
{"schema":"bach.agent_report.v1","agent":{"role":"docs-sweeper"},"status":"success","summary":"CLI changed without docs","metrics":[],"findings":[{"kind":"missing-docs","severity":"error","file":"README.md","rule":"api-cli-docs","message":"CLI/API change lacks documentation update"}]}
JSON
SH
  chmod +x "$E2E_PROJECT/scripts/write-docs-report.sh"
  write_bachfile <<'HCL'
project "e2e" {
  root  = "."
}

prompt "implementer" {
  path = "prompts/agents/implementer.md"
}

shell "docs-policy" {
  command = ["sh", "scripts/write-docs-report.sh"]
}

quality "shell.docs-policy" {
  reports = [
    { kind = "docs", format = "agent-report-v1", path = ".bach/artifacts/docs.json" },
  ]

  quality_gate {
    metric = "policy.docs_sweeper.blocking_findings.count"
    max    = 0
  }
}
HCL

  run bach run --log-only shell/docs-policy
  assert_failure
  assert_output_contains "quality gate failed"
  assert_output_contains "policy.docs_sweeper.blocking_findings.count actual 1.000 must be <= 0.000"
  policy_files=("$E2E_PROJECT"/.bach/artifacts/policies/*/shell-docs-policy.json)
  assert_file_contains "${policy_files[0]}" '"verdict": "failed"'
}
