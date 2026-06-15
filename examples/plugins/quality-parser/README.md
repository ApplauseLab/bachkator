# Quality Parser Plugin Example

This example shows a project-local quality plugin. The `shell/lint` target writes a simple TODO report, and `plugin "todo_lint" { type = "quality" }` converts that report into Bach quality metrics and findings.

Run it from this directory:

```sh
bach run --log-only --force shell/lint
bach quality metrics
bach quality findings
bach quality gates
```

The first run fails the quality gate because `src/app.txt` contains a TODO. Remove the TODO line, then rerun:

```sh
bach run --log-only --force shell/lint
```

The parser receives the report path as its first argument and emits normalized JSON on stdout:

```json
{
  "metrics": [
    { "name": "issues.total.count", "value": 1, "unit": "count" }
  ],
  "findings": [
    {
      "kind": "issue",
      "file": "src/app.txt",
      "line": 3,
      "severity": "warning",
      "rule": "todo",
      "message": "TODO found"
    }
  ]
}
```

The full schema is documented in `../../../docs/schemas/quality-plugin-report.schema.json`.
