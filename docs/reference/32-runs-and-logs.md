## Runs And Logs

Every non-dry-run invocation records a run in SQLite table `runs`. Every non-dry-run target execution records a row in `target_runs`. Dry-runs are read-only with respect to persistent state: they may read existing cache records to explain freshness or staleness, but they do not create `.bach/state.db` and do not persist runs, target runs, artifacts, fingerprints, or cache state.

Output is streamed to both the terminal and per-target log files:

```text
.bach/runs/<run-id>/<target>.log
```

Use `bach runs` to list the 10 most recent prior non-dry-run runs, statuses, timestamps, and log directories. Use `bach runs --runs-limit 0` to list all recorded non-dry-run runs.

Each run ends with a concise terminal summary, including the run ID, status, requested target, duration, log directory, and target status counts:

```text
run 20260608T120000.000000000Z success target=shell/test duration=1.2s logs=.bach/runs/20260608T120000.000000000Z
targets: success=3 cached=2 failed=0 dry-run=0 running=0
```

`-log-only` suppresses command stdout/stderr in the terminal while keeping Bach progress, quality progress, and the final summary visible. `quiet = true` targets suppress their command output and target progress unless `-verbose` is set. Full target output is still written to the target log so agents can report the outcome and log location. Failed runs include the last 20 non-empty lines from the first failed target log; successful runs do not print log excerpts.
