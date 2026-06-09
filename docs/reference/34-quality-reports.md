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

Unqualified quality targets default to shell targets, so `quality "test-api"` attaches to `shell/test-api`. Use `quality "image/build"` or `quality "pipeline/release"` when targeting other target types.

After the target operation exits successfully, Bachkator parses declared reports into the SQLite state database, then evaluates gates. A failing gate marks the target and run as `quality-failed`.

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
