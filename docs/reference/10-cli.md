## CLI

```sh
bach list
bach run <target>
bach affected [path ...]
bach artifacts [run-id]
bach explain <target>
bach graph
bach quality [summary|reports|metrics|findings|gates|slow-targets|failing-tests]
bach reference [topic]
bach runs
```

Supported flags:

- `-f <path>`: load a different Bachfile. Default is `Bachfile`.
- `--aliases`: with `list`, include target aliases and their canonical targets.
- `--verbose`: with `list`, include cost and risk metadata; with `run`, stream quiet target output.
- `--runs-limit <n>`: maximum runs or artifacts to list. Defaults to `10`; use `0` for all.
- `--target <target>`: filter `runs` or `artifacts` by target address.
- `--status <status>`: filter `runs` or `artifacts` by run status.
- `--since <duration|time>`: filter `runs` or `artifacts` by age or RFC3339 timestamp.
- `--artifact <path>`: filter `runs` or `artifacts` by artifact path substring.
- `--dry-run`: print the planned operations without executing them.
- `-force`: run cacheable targets even when their fingerprints are fresh.
- `-yes`: confirm execution of targets marked `requires_confirmation = true`.
- `-env-file <path>`: load target operation environment from this file after project `.env`.
- `-profile <name>`: select an environment profile. May be repeated; later profiles win.
- `-log-only`: suppress command stdout/stderr in the terminal while keeping Bach progress and quality progress visible; full output is still written to `.bach/runs/.../*.log` files.
- `-j <n>`: maximum number of targets to run in parallel.
- `-var name=value`: set a Bachkator variable. May be repeated.
- `--json`: with `--dry-run run`, print a machine-readable execution plan.
- `--format <mermaid|json>`: choose the `graph` output format.
- `--version`: print the Bachkator version.

`bach run` requires a target address such as `shell/test`, `image/app`, or `pipeline/release`.

`bach quality` reads parsed quality reports from the state database. `summary` shows recent reports,
quality gates, slow targets, and top failing tests. Use `metrics` for normalized values such as
`coverage.line.percent`, `findings` for parsed Checkstyle/JUnit-style findings, and `gates` for threshold
results.
