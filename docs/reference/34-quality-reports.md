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
