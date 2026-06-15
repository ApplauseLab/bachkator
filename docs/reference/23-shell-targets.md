## Shell Targets

Shell targets run commands:

```hcl
shell "build" {
  description           = "Build the binary"
  when                  = "after tests pass, before packaging"
  cost                  = "medium"
  remote                = false
  destructive           = false
  requires_confirmation = false
  depends_on            = [shell.test]
  quiet                 = true
  lock                  = "container-builder"
  timeout               = "5m"
  retry {
    attempts = 3
    backoff  = "2s"
  }
  env {
    CGO_ENABLED = "0"
    GOARCH      = "arm64"
    GOOS        = "linux"
  }
  command     = ["go", "build", "-o", "dist/bach", "./cmd/bach"]
  tools       = [{ name = "go", command = ["go", "version"], version = "1.24+" }]
  preflights  = [{ name = "package registry session", command = ["bun", "pm", "whoami"], fix = "Run bun login." }]
  inputs      = [input.file.go_sources]
  outputs     = {
    binary = "dist/bach"
  }

  success_when {
    file_exists = "dist/bach"
  }
}
```

Fields:

- `description`: shown by `bach list` and `bach explain`.
- `when`: guidance for when humans or agents should run the target. This is metadata only; use `depends_on` or pipeline `steps` to enforce order.
- `cost`: expected cost. Valid values are `low`, `medium`, or `high`.
- `remote`: set to `true` when the target talks to external services.
- `destructive`: set to `true` when the target can delete, overwrite, or irreversibly change state.
- `requires_confirmation`: set to `true` when operators should confirm intent before running. Real execution then requires `-yes`; dry-run is still allowed.
- `depends_on`: explicit target dependencies.
- `lock`: optional in-run named lock. Ready targets with the same lock do not run concurrently in one Bachkator invocation.
- `timeout`: optional Go-style duration such as `30s`, `5m`, or `1h`. The timeout bounds target operation execution and completion-contract checks.
- `retry`: optional operation retry policy. `attempts` is the total number of attempts; `backoff` is an optional duration to wait between failed attempts.
- `command`: argv array executed directly.
- `shell`: shell string executed via `/bin/sh -c`.
- `tools`: required host tools checked before execution. Each entry has `name` plus optional `command`, `version`, and `fix` fields. `command` is an exact probe command; Bachkator does not parse versions yet.
- `preflights`: credential or session probes checked after required tools and before target execution. Each entry has `command` plus `name` or `kind`, and optional `fix`. Bachkator treats these as generic host checks and does not hardcode provider-specific auth behavior.
- `quiet`: when `true`, write this target's progress and operation output only to `.bach/runs/.../*.log` unless `-verbose` is set.
- `workdir`: working directory relative to project root.
- `env { ... }`: sorted target environment block. Values can reference top-level env entries, earlier resolvable target env entries, process env, and `var.name`. Commands reference target env at runtime with `$NAME` or `${NAME}`.
- `inputs`: file paths, input references, plugin input references, or resources.
- `outputs`: paths expected after execution. Use a list for unnamed cache evidence or an object for named outputs such as `{ junit = "reports/junit.xml" }`.
- `produces`: inputs or resources produced by this target.
- `success_when { ... }`: optional post-execution checks. When omitted, command exit code is the success signal.
- `fail_when { ... }`: optional post-execution failure checks. Matching checks make the target fail even when the command exits successfully.

Use either `command` or `shell`, not both.

Targets with only `depends_on` are aggregate targets.

Dry-run reports required tools and preflights but does not fail when they are missing locally. Normal runs fail before starting target operations when a required tool is missing, a required tool probe exits non-zero, or a preflight exits non-zero. Preflight failures use `preflight-failed` in the run summary so expired sessions are distinct from target operation failures. Aggregate and pipeline targets require the union of child target tools and preflights.

Retry applies only to operation failures and completion-contract failures. Required tools, preflight failures, and quality gate failures are not retried.

Quality reports and gates are configured in top-level `quality` blocks so shell targets stay focused on commands.

`outputs` are concrete file paths. `produces` are logical input/resource identities made available to other targets. Quality reports consume `outputs`, not `produces`.

Use `$(RUN_DIRECTORY)` for per-run generated files such as test reports. Bach expands it before executing the operation and also exposes the value as `BACH_RUN_DIRECTORY` and `RUN_DIRECTORY`.

Use locks for shared local or remote resources while keeping high overall parallelism:

```hcl
shell "test-db" {
  lock    = "postgres"
  command = ["bun", "test", "db"]
}
```

Locks are process-local to a single Bachkator run. They do not coordinate separate `bach` processes.

See `bach reference completion-contracts` for generic post-execution checks using `success_when` and `fail_when`.
