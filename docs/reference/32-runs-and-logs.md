## Runs And Logs

Every non-dry-run invocation records a run in SQLite table `runs`. Every non-dry-run target execution records a row in `target_runs`. Dry-runs are read-only with respect to persistent state: they may read existing cache records to explain freshness or staleness, but they do not create `.bach/state.db` and do not persist runs, target runs, artifacts, fingerprints, or cache state.

Output is streamed to both the terminal and per-target log files:

```text
.bach/runs/<run-id>/<target>.log
```

Use `bach runs` to list the 10 most recent prior non-dry-run runs, statuses, timestamps, and log directories. Use `bach runs --runs-limit 0` to list all recorded non-dry-run runs.

Use `bach runs inspect <run-id>` to inspect one completed run. The human output highlights failed targets, exit codes, log paths, parsed quality reports, failed quality gates, and declared preflight/tool fixes. Use `--json` for an agent-readable summary:

```sh
bach runs inspect --json 20260608T120000.000000000Z
```

The JSON object includes:

- `run_id`, `requested_target`, `status`, `started_at`, `finished_at`, and `log_dir`.
- `failed_targets[]` with `target`, `status`, optional `exit_code`, `operation`, `log_path`, `artifacts`, `quality`, preflight/tool diagnostics, and a bounded `log_excerpt`.
- `quality.reports[]` with report path, parser status, metric count, finding count, and parser message when present.
- `quality.failed_gates[]` with metric, operator, threshold, actual value, and message.
- `suggested_fixes[]` from declared tool or preflight `fix` fields.

Use `bach logs <run-id>` for concise log slices without opening large files manually:

```sh
bach logs 20260608T120000.000000000Z --failed --last 80
bach logs 20260608T120000.000000000Z --target shell/test --errors
```

`--failed` limits output to failed target logs. `--target` selects one target. `--last N` bounds output. `--errors` keeps lines containing likely error/failure terms.

Each run ends with a concise terminal summary, including the run ID, status, requested target, duration, log directory, and target status counts:

```text
run 20260608T120000.000000000Z success target=shell/test duration=1.2s logs=.bach/runs/20260608T120000.000000000Z
targets: success=3 cached=2 failed=0 dry-run=0 running=0
```

`-log-only` suppresses command stdout/stderr in the terminal while keeping Bach progress, quality progress, and the final summary visible. `quiet = true` targets suppress their command output and target progress unless `-verbose` is set. Full target output is still written to the target log so agents can report the outcome and log location. Failed runs include the last 20 non-empty lines from the first failed target log; successful runs do not print log excerpts.
