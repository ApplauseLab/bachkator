## Quality Reports

Targets publish concrete report files as named `outputs`. Quality blocks attach parsers and thresholds to those outputs without bloating the target command definition:

```hcl
shell "test-api" {
  command = ["bun", "test", "--reporter", "junit", "--junit-path", "$(RUN_DIRECTORY)/junit.xml", "--coverage"]
  outputs = {
    junit = "$(RUN_DIRECTORY)/junit.xml"
    lcov  = "$(RUN_DIRECTORY)/lcov.info"
    lint  = "$(RUN_DIRECTORY)/checkstyle.xml"
  }
}

quality "test-api" {
  junit {
    path = shell.test-api.outputs.junit
  }

  cov {
    path = shell.test-api.outputs.lcov
  }

  lint {
    path   = shell.test-api.outputs.lint
  }

  quality_gate {
    metric = "tests.failed"
    max    = 0
  }

  quality_gate {
    metric = "coverage.line.percent"
    min    = 80
  }
}
```

For linters that can emit Checkstyle, let Bachkator own the failure decision by forcing the linter command to exit zero and enforcing the report through a quality gate:

```hcl
shell "lint" {
  command = [
    "golangci-lint",
    "run",
    "--issues-exit-code=0",
    "--output.checkstyle.path=$(RUN_DIRECTORY)/checkstyle.xml",
    "--output.text.path=stdout",
  ]
  outputs = {
    checkstyle = "$(RUN_DIRECTORY)/checkstyle.xml"
  }
}

quality "lint" {
  lint {
    path   = shell.lint.outputs.checkstyle
    format = "checkstyle-xml"
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}
```

Unqualified quality targets default to shell targets, so `quality "test-api"` attaches to `shell.test-api`. Use `quality "image.build"` or `quality "pipeline.release"` when targeting other target types.

After the target operation exits successfully, Bachkator parses declared reports into the SQLite state database, then evaluates gates. A failing gate marks the target and run as `quality-failed`.

Quality blocks can also attach Rego policies that evaluate already-normalized report evidence:

```hcl
quality "shell.security-scan" {
  reports = [
    { kind = "security", format = "agent-report-v1", path = "$(RUN_DIRECTORY)/security.json" },
  ]

  rego_policy {
    path    = "policies/no-critical-security.rego"
    package = "bach.policy"
  }
}
```

`rego_policy.path` is required and is project-relative. `package` is optional, but when set Bach defaults `allow` to `data.<package>.allow` and `findings` to `data.<package>.findings`. If `package` is omitted, set explicit `allow` and `findings` queries. Multiple `rego_policy` blocks may attach to one quality block; every policy must allow the target.

Bach evaluates Rego in-process after parsing declared reports and before final quality status. Rego receives normalized JSON only; raw report file contents are not included:

```json
{
  "schema": "bach.rego_input.v1",
  "run": { "id": "20260612T150325.460845000Z" },
  "target": { "name": "shell/security-scan", "kind": "shell" },
  "git": { "commit": "abc123", "branch": "main", "dirty": false },
  "reports": [
    {
      "kind": "security",
      "format": "agent-report-v1",
      "path": ".bach/runs/.../security.json",
      "status": "success",
      "message": "",
      "metrics": [],
      "findings": [
        {
          "kind": "security",
          "severity": "critical",
          "rule": "CVE-2026-1234",
          "message": "openssl has critical vulnerability",
          "file": "go.sum",
          "line": 42,
          "duration_ms": 0
        }
      ]
    }
  ],
  "metrics": { "policy.security.blocking_findings.count": 1 },
  "findings": [
    {
      "kind": "security",
      "severity": "critical",
      "rule": "CVE-2026-1234",
      "message": "openssl has critical vulnerability",
      "file": "go.sum",
      "line": 42,
      "duration_ms": 0
    }
  ]
}
```

Policy output must expose a single boolean `allow`. `findings` may be undefined, empty, or a collection of normalized finding objects with at least `kind`:

```rego
package bach.policy

default allow := true

allow := false if {
  some i
  f := input.findings[i]
  f.kind == "security"
  f.severity == "critical"
}

findings := [finding |
  some i
  f := input.findings[i]
  f.kind == "security"
  f.severity == "critical"
  finding := {
    "kind": "security-policy",
    "severity": "error",
    "rule": "no-critical-security-findings",
    "message": sprintf("Critical security finding: %s", [f.message]),
    "file": f.file,
    "line": f.line,
  }
]
```

Bach stores each Rego decision as a synthetic quality report with `kind = "policy"` and `format = "rego-policy-v1"`. Compile errors, query errors, non-boolean `allow`, invalid findings, and `allow = false` mark the synthetic report failed and make the target `quality-failed` when the target command otherwise succeeded. Rego-emitted findings are visible through `bach quality findings` and `bach runs inspect --json <run-id>`. Rego `path` values resolve from the project root, not a shell target's `work_dir`. Rego source files and query configuration (`package`, `allow`, and `findings`) participate in target fingerprints, so policy edits and query changes invalidate cached targets. Bach evaluates policies with network and runtime-introspection builtins such as `http.send` and `opa.runtime` disabled.

Use quality plugins to convert raw project-specific report files into normalized metrics and findings. Use Rego policies to decide whether the normalized evidence across one or more reports is acceptable.

If the target command exits non-zero but declared report files already exist, Bachkator still attempts best-effort parsing before finalizing the target as `failed`. Parsed reports and gate results are saved as diagnostic evidence, so commands such as `bach quality failing-tests` and `bach runs inspect --json <run-id>` can help diagnose the failed command. Missing report files are tolerated on this path because the command failure may have prevented report creation. Parser and gate failures after a command failure are recorded as secondary diagnostics; they do not change the primary target status from `failed` to `quality-failed`.

`$(RUN_DIRECTORY)` is a Bach runtime placeholder expanded before operation execution. It points at a per-target directory under the current run, for example `.bach/runs/<run-id>/shell__test-api`. Bach also exposes the same value as `BACH_RUN_DIRECTORY` and `RUN_DIRECTORY` environment variables.

Supported initial formats:

- `junit-xml`: test counts, durations, and failing tests.
- `checkstyle-xml`: lint/static-analysis findings and severity counts.
- `lcov`: line coverage, common in JavaScript/Bun projects.
- `cobertura-xml`: generic line and branch coverage.
- `jacoco-xml`: JVM coverage and complexity counters.
- `codecov-json`: coverage JSON with top-level or `totals` coverage values.
- `go-cover`: Go coverage profiles, used by Bachkator's own test target.
- `gocyclo`: optional complexity reports.
- `agent-report-v1`: Bach Agent Report evidence from implementation or reviewer agents.

Use the generic `reports` list when a target emits multiple evidence files or a format does not fit one of the shorthand blocks:

```hcl
quality "shell.agent-fixture" {
  reports = [
    { kind = "agent", format = "agent-report-v1", path = ".bach/artifacts/agents/implementer.json" },
    { kind = "docs", format = "agent-report-v1", path = ".bach/artifacts/agents/docs.json" },
    { kind = "org-policy", parser = "org_policy", path = ".bach/artifacts/org-policy.txt" },
  ]
}
```

Each report entry must set `kind`, `path`, and exactly one of `format` or `parser`. In list form, `parser` is the plugin name as a string. In shorthand blocks such as `lint {}`, use `parser = plugin.<name>`.

Project-specific report formats can use a quality plugin:

```hcl
plugin "atelier_lint" {
  type    = "quality"
  command = ["bun", "scripts/bach/parse-lint-quality.ts"]
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.json"
    parser = plugin.atelier_lint
  }

  quality_gate {
    metric = "issues.total.count"
    max    = 0
  }
}
```

Quality plugin stdout must be normalized JSON:

```json
{
  "metrics": [
    { "name": "issues.total.count", "value": 0, "unit": "count" }
  ],
  "findings": []
}
```

The JSON must include `metrics`, `findings`, or both. Unknown top-level fields are rejected. Metric `name` and `value` are required; finding `kind` is required.

The JSON Schema lives at `docs/schemas/quality-plugin-report.schema.json` and is embedded in the `bach` binary. Agents can read it with `bach reference quality-plugin-report-schema`.

Use `format = "junit-xml"` and other built-in format strings for built-in parsers. Use `parser = plugin.<name>` for quality plugins. A report declaration must not set both `format` and `parser`.

`agent-report-v1` expects this JSON envelope:

```json
{
  "schema": "bach.agent_report.v1",
  "agent": { "role": "docs-sweeper", "name": "fixture" },
  "subject": { "target": "shell/api" },
  "status": "success",
  "summary": "docs are current",
  "metrics": [{ "name": "documentation.changed_files.count", "value": 1, "unit": "count" }],
  "findings": []
}
```

`schema` must be `bach.agent_report.v1`, `agent.role` is required, and finding `kind` is required. `status` may be omitted in hand-authored compatibility artifacts and ingests as `success`; reports written by `bach report` always use explicit `success` or `failed`. Agent-supplied custom metrics must have non-empty names and must not use the reserved `agent.*` or `policy.*` namespaces. Bach emits standard metrics such as `agent.<role>.status.success`, `agent.<role>.report.count`, `agent.<role>.findings.count`, `policy.<role>.findings.count`, and `policy.<role>.blocking_findings.count`. Roles are normalized by replacing `/`, spaces, and `-` with `.` or `_` as needed for metric names.

Agent report findings with severity `blocker`, `blocking`, `critical`, `error`, `failure`, or `failed` count as blocking findings. A non-`success` agent report status marks the parsed evidence as failed and makes the target `quality-failed` unless the target had already failed for another reason.

Agents can create and update this envelope without hand-writing JSON:

```sh
export BACH_AGENT_QUALITY_REPORT_PATH=.bach/artifacts/agents/docs.json
bach report init --role docs-sweeper --name opencode --summary "Review started"
bach report finding --kind docs --severity error --rule stale-reference --message "CLI docs are stale"
bach report metric --name review.docs.checked_files.count --value 12 --unit count
bach report status success --summary "Docs review passed"
```

`bach report finding --stdin` accepts one strict JSON finding object with fields such as `kind`, `severity`, `rule`, `message`, `file`, `line`, and `duration_ms`; unknown fields and trailing JSON are rejected. Destination resolution is `--path`, then `BACH_AGENT_QUALITY_REPORT_PATH`, then `$BACH_RUN_DIRECTORY/agent-report-v1.json`. Report paths must stay under the current workspace or `BACH_RUN_DIRECTORY` unless `--allow-external-path` is set. The helper writes artifacts only; the target must still declare the file with `format = "agent-report-v1"` for quality ingestion and State Store persistence.

Reviewer policy aggregation uses the internal `agent-report-json` format. It is distinct from the
public `agent-report-v1` quality envelope: reviewers write `mode`, `role`, `status`, `subject`,
`metrics`, `findings`, and `message` fields to the path injected by `BACH_AGENT_REPORT_PATH`. Bach
generates the exact reviewer report schema in each reviewer prompt.

Metric names are policy-owned and must be unambiguous. Duplicate metric names within one report fail with `policy-metric-collision`. Duplicate metric names across reports also fail, except repeated `agent-report-v1` count metrics ending in `.count`, which Bach aggregates for repeated same-role evidence. Use organization-specific plugin metrics under a unique namespace such as `policy.org.*`.

Bach writes an applied policy artifact for each quality-enabled target at:

```text
.bach/artifacts/policies/<run-id>/<sanitized-target>.json
```

Sanitization replaces `/`, `:`, and spaces with `-`.

The artifact uses schema `bach.applied_policy.v1` and records the run id, target, producing `policy_target` when applicable, `passed` or `failed` verdict, minimized report evidence, gate results, and creation time. For generated policy targets, `target` is the reviewed subject target while `policy_target` is the generated policy target that produced the verdict; merge agents require that exact `policy_target` to have succeeded in the recorded run. It intentionally stores counts and statuses rather than full finding messages to avoid creating an extra broad evidence export surface.

Quality parser failures and quality gate failures mark the target `quality-failed`. Cached targets do not rerun quality parsers or gates; use `--force` when fresh quality evidence is required.

Targets can opt into retrying gate failures:

```hcl
retry {
  attempts                      = 3
  backoff                       = "5s"
  retry_on_quality_gate_failure = true
}
```

Only gate failures retry. Parser failures do not retry because they usually indicate a broken parser/report contract.

Query stored quality data:

```sh
bach quality summary
bach quality metrics
bach quality findings
bach quality gates
bach quality slow-targets
bach quality failing-tests
```

Useful gate metrics include `tests.failed`, `tests.duration.ms`, `coverage.line.percent`, `coverage.branch.percent`, `issues.total.count`, `issues.error.count`, `issues.warning.count`, `complexity.max`, and `complexity.avg`.
