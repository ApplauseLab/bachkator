<!-- Generated from docs/reference/*.md. Edit fragments, then run shell/docs-generate. -->

# Bachkator Reference

Bachkator reads `Bachfile` from the current directory by default. The file is HCL with a Vim-friendly modeline often used at the top:

```hcl
# vim: set ft=hcl :
```

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
bach runs inspect <run-id>
bach logs <run-id>
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
- `--failed`: with `logs`, show only failed target logs.
- `--last <n>`: with `logs`, show only the last N log lines.
- `--errors`: with `logs`, show only likely error/failure lines.
- `--dry-run`: print the planned operations without executing them.
- `-force`: run cacheable targets even when their fingerprints are fresh.
- `-yes`: confirm execution of targets marked `requires_confirmation = true`.
- `-env-file <path>`: load target operation environment from this file after project `.env`.
- `-profile <name>`: select an environment profile. May be repeated; later profiles win.
- `-log-only`: suppress command stdout/stderr in the terminal while keeping Bach progress and quality progress visible; full output is still written to `.bach/runs/.../*.log` files.
- `-j <n>`: maximum number of targets to run in parallel.
- `-var name=value`: set a Bachkator variable. May be repeated.
- `--json`: with `--dry-run run`, print a machine-readable execution plan; with `runs inspect`, print a machine-readable failure summary.
- `--format <mermaid|json>`: choose the `graph` output format.
- `--version`: print the Bachkator version.

`bach run` requires a target address such as `shell/test`, `image/app`, or `pipeline/release`.

`bach quality` reads parsed quality reports from the state database. `summary` shows recent reports,
quality gates, slow targets, and top failing tests. Use `metrics` for normalized values such as
`coverage.line.percent`, `findings` for parsed Checkstyle/JUnit-style findings, and `gates` for threshold
results.

## Affected Targets

`bach affected [path ...]` suggests the smallest useful configured targets for changed files. It is read-only and never runs targets.

With explicit paths, Bachkator matches those paths against each target's resolved `inputs`, including named inputs and plugin-provided inputs:

```sh
bach affected packages/api/src/foo.ts
```

With no paths, Bachkator uses the current Git staged, unstaged, and untracked files:

```sh
bach affected
```

Output is sorted by target name. Each line includes the target name, the number of matching inputs, and up to the first three matching inputs.

## Target Aliases

Use top-level `alias` blocks to preserve old command names while directing users and agents to canonical targets.

```hcl
alias "staging-kristiyan-deploy" {
  target      = "pipeline.deploy-kristiyan"
  deprecated = "Use pipeline.deploy-kristiyan."
}
```

Aliases resolve before planning and execution, so dry-runs, locks, cache keys, logs, and run history use the canonical target name. Aliases do not create executable target nodes.

`deprecated` is optional. When present, Bachkator prints the message when the alias is used and includes it in `bach explain <alias>`.

List aliases with:

```sh
bach list --aliases
```

Alias targets must point directly to real targets. Alias-to-alias chains are rejected at load time.

## Target Explain

`bach explain <target>` prints target guidance without running anything. It includes description, when to use the target, cost, risk flags, dependencies, pipeline steps, inputs, outputs, and produced resources. If `<target>` is an alias, explain also prints the alias name, canonical target, and optional deprecation message.

```sh
bach explain shell/test-api
```

Use explain before high-cost, remote, destructive, or unfamiliar targets.

## Target Metadata

Shell, image, and pipeline targets support optional guidance metadata:

```hcl
shell "deploy-staging" {
  description           = "Deploy staging API"
  when                  = "after image publish"
  cost                  = "high"
  remote                = true
  destructive           = true
  requires_confirmation = true
}
```

Fields:

- `description`: shown by `bach list` and `bach explain`.
- `when`: guidance for when humans or agents should run the target.
- `cost`: expected cost. Valid values are `low`, `medium`, or `high`.
- `remote`: set to `true` when the target talks to external services.
- `destructive`: set to `true` when the target can delete, overwrite, or irreversibly change state.
- `requires_confirmation`: set to `true` when operators should confirm intent before running.

Risk metadata is inherited through `depends_on` and pipeline `steps`, so aggregate targets show and enforce the risk of the targets they run. Dry-runs are always allowed. Real execution of a target whose effective risk includes `requires_confirmation` must use `-yes`.

## Environment Files

Bachkator loads `.env` from the project root into target operation environments when the file exists. Use `-env-file .env.local` to overlay another file on top of `.env`.

Environment precedence for operations is:

- parent process environment.
- project `.env` values.
- `-env-file` values.
- top-level Bachfile `env` values.
- selected `profile` env values, in CLI order.
- Bachkator runtime values such as `BACH_GIT_COMMIT`.
- target `env` entries.

Environment files support blank lines, `#` comments, optional `export`, `KEY=value`, and single- or double-quoted values.

## Project Environment

Top-level `env` blocks define project-wide operation environment. Entries are stored and fingerprinted in sorted key order.

```hcl
var bla {}

var "foo" {
  default = "foo"
}

var foobar {
  default = "${var.foo}bar"
}

env {
  ENV_1 = "b"
  ENV_2 = "${ENV_1} b ${var.bla}"
}
```

Environment values can reference variables with `var.name` and earlier resolvable environment keys directly by name. HCL string literals use double quotes.

## Environment Profiles

Profiles are named environment overlays for staging variants and operator-specific settings:

```hcl
profile "staging" {
  env {
    NAMESPACE   = "atelier-staging"
    AWS_PROFILE = "atelier-staging"
    PUBLIC_HOST = "staging.example.com"
  }
}

profile "staging-kristiyan" {
  env {
    NAMESPACE = "atelier-staging-kristiyan"
  }
}
```

Select profiles with `-profile`. The flag may be repeated, and later profiles override earlier profile values:

```sh
bach -profile staging -profile staging-kristiyan shell/render
```

Selected profile values overlay after top-level `env` and before target `env`. Unknown selected profiles are errors. Selected profile names and resolved profile values are included in target fingerprints.

## Embedded Reference

Bachkator embeds the `docs/` reference into the binary.

```sh
bach reference
bach reference project
bach reference plugins
```

`bach reference` lists all headings. `bach reference <query>` prints matching sections. It does not require a `Bachfile`, so agents can use it before project loading succeeds.

## Project

Every Bachfile needs one `project` block:

```hcl
project "example" {
  root    = "."
  default = "shell.test"
  state   = ".bach/state.db"
}
```

Fields:

- `root`: project working directory. Defaults to the Bachfile directory.
- `default`: target to run when no target is provided.
- `state`: SQLite state path. Defaults to `.bach/state.db` under `root`.

## Variables

Variables are declared with `var` blocks and referenced as `var.name`:

```hcl
var "release_version" {
  default = ""
}

shell "release" {
  command = ["gh", "release", "create", var.release_version]
}
```

Variable defaults can derive values with built-in computed functions:

```hcl
var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("bun.lock", "package.json")}"
}
```

Computed functions:

- `git_short_sha()`: first 12 hex characters of the current project-root Git commit.
- `git_dirty_suffix()`: `-dirty` when the project-root Git worktree has changes, otherwise an empty string.
- `file_hash(paths...)`: first 12 hex characters of a deterministic content hash for files, globs, or directories under the project root.

Value precedence:

- `-var release_version=v0.1.0`
- `BACH_VAR_release_version=v0.1.0`
- `BACH_VAR_RELEASE_VERSION=v0.1.0`
- `default` in the `var` block
- empty string when no default is set

## Computed Variables

Variable defaults can call built-in functions:

- `git_short_sha()`: first 12 hex characters of the project-root Git commit.
- `git_dirty_suffix()`: `-dirty` when the project-root Git worktree has changes, otherwise an empty string.
- `file_hash(paths...)`: first 12 hex characters of a deterministic content hash for files, globs, or directories under the project root.

```hcl
var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("bun.lock", "package.json")}"
}
```

## Inputs

Inputs name reusable file sets:

```hcl
input "file" "go_sources" {
  srcs = ["go.mod", "go.sum", "cmd", "internal"]
}
```

Fields:

- `src`: one file, directory, or glob.
- `srcs`: multiple files, directories, globs, or input references.

Targets can use raw paths or named inputs:

```hcl
shell "test" {
  command = ["go", "test", "./..."]
  inputs  = [input.file.go_sources]
}
```

Directories are walked and content-hashed. Resources are not hashed directly.

## Resources

Resources model produced capabilities or artifacts without hashing large output trees:

```hcl
resource "workspace_deps" {}

shell "install" {
  command  = ["bun", "install"]
  produces = [resource.workspace_deps]
  outputs  = ["node_modules"]
}

shell "test" {
  command = ["bun", "test"]
  inputs  = [resource.workspace_deps]
}
```

When a target consumes a produced resource, Bachkator automatically adds an implicit dependency on the producer.

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

## Completion Contracts

Completion contracts are generic execution evidence. They do not replace `outputs`, which remain cache evidence. Contracts are evaluated only after the target operation exits successfully. When both `success_when` and `fail_when` are omitted for a shell target, Bach relies on the command exit code exactly as before.

```hcl
shell "deploy" {
  command = ["./deploy.sh"]

  fail_when {
    output_contains = "ROLLBACK"
  }

  success_when {
    output_contains = "Deployment complete"
  }

  success_when {
    command = ["./scripts/smoke.sh"]
  }
}
```

Supported completion checks are:

- `output_contains`: match text in the target log after operation execution.
- `file_exists`: require a file path under the project root to exist.
- `command`: run a verification command from the target workdir with the target environment.

Each `success_when` or `fail_when` block must set exactly one check field.

## Group Targets

Group targets name an unordered collection of existing targets. Use a group when several
targets should run as one logical step but may execute in parallel subject to dependencies,
locks, cache state, and preflights:

```hcl
group "ci" {
  targets = [shell.lint, shell.test, shell.build]
}
```

Running `bach run group/ci` walks one deduplicated execution graph. Shared dependencies and
shared group members run at most once per run.

Groups can be used as pipeline steps:

```hcl
group "checks" {
  targets = [shell.lint, shell.test]
}

pipeline "release" {
  steps = [group.checks, shell.build-release, shell.github-release]
}
```

In that example, `shell.build-release` cannot start until all targets in `group.checks` and
their dependency closures complete. Inside the group, normal dependency and lock rules decide
what can run concurrently.

Fields:

- `description`: shown by `bach list`.
- `targets`: existing target references to include in the unordered group.
- `timeout`: optional Go-style duration that bounds the whole group invocation, including all
  members.
- `lock`: optional lock held for the whole group invocation.

Groups are scoped completion targets. They do not run their own command and do not have
independent cache inputs or outputs; cache and fingerprints remain attached to the member
targets. Group-level `timeout` and `lock` wrap member execution, while member-level runtime
fields still apply to each individual target.

## Pipeline Targets

Pipeline targets run existing targets in a declared sequence. Use them for deploy flows where dependency concurrency is unsafe:

```hcl
pipeline "deploy-staging" {
  timeout = "15m"
  steps = [
    shell.render-staging,
    shell.apply-staging,
    shell.rollout-staging,
    shell.smoke-staging,
  ]
}
```

Pipeline steps may reference shell, image, or pipeline targets. Nested pipelines let you name reusable ordered lanes and compose them into a higher-level program:

```hcl
pipeline "build-lane" {
  steps = [shell.build-a, shell.build-b]
}

pipeline "merge-lane" {
  steps = [shell.merge-a, shell.merge-b]
}

pipeline "delivery-program" {
  steps = [pipeline.build-lane, pipeline.merge-lane, shell.regression]
}
```

Pipeline cycles are rejected during config loading.

Fields:

- `description`: shown by `bach list`.
- `steps`: existing target names to run in order.
- `timeout`: optional Go-style duration that bounds the whole pipeline invocation, including all steps.

`bach -dry-run pipeline/deploy-staging` prints the pipeline and then each step in execution order. `bach pipeline/deploy-staging` stops at the first failed step, so later steps do not run. Step targets remain runnable directly for debugging.

Pipeline targets inherit risk metadata from their steps. If any step has `requires_confirmation = true`, running the pipeline requires `-yes`; `-dry-run` still works without confirmation.

Pipeline retry is not enabled by default; use retry on individual shell steps that are safe to repeat.

## Image Targets

Image targets synthesize OCI build commands:

```hcl
resource "base_image" {}

image "base" {
  builder    = "container"
  image      = "example/base"
  tags       = ["local"]
  dockerfile = "Dockerfile.base"
  context    = "."
  platform   = "linux/amd64"
  produces   = [resource.base_image]
}

image "app" {
  builder    = "container"
  image      = "example/app"
  tags       = ["latest"]
  dockerfile = "Dockerfile"
  push       = true
  build_args = {
    BASE_IMAGE = image.base.tag
  }
  inputs = [resource.base_image, "Dockerfile", "cmd"]
}
```

Fields:

- `builder`: build executable. Defaults to `OCI_BUILDER`, then `docker`.
- `image`: image name. Defaults to the image target name without `image/`.
- `tags`: tags to apply.
- `dockerfile`: Dockerfile path. Defaults to `Dockerfile`.
- `context`: build context. Defaults to `.`.
- `platform`: optional platform.
- `push`: push every resolved tag after a successful build. Docker-compatible builders run `<builder> push <tag>`; Apple `container` runs `container image push <tag>`.
- `build_args`: map of build args.
- `build_args_list`: list of already-rendered `KEY=value` build args.
- `lock`: optional in-run named lock. Useful for builders such as Apple `container` that share host resources.
- `inputs`, `depends_on`, `produces`: same behavior as shell targets.

`image.name.tag` resolves to the first full image tag for use in build args.

## GitHub Releases

Bachkator does not need a special release target type. Use a shell target with the GitHub CLI:

```hcl
var "github_repo" {
  default = "owner/repo"
}

var "release_version" {
  default = ""
}

shell "github-release" {
  depends_on = [shell.build]
  command = [
    "gh",
    "release",
    "create",
    var.release_version,
    "dist/app",
    "--repo",
    var.github_repo,
    "--target",
    "$BACH_GIT_COMMIT",
    "--title",
    "App ${var.release_version}",
    "--generate-notes",
    "--latest",
  ]
}
```

Run it with:

```sh
bach -var release_version=v0.1.0 shell/github-release
```

`gh release create <tag>` creates the Git tag when it does not exist. `--target "$BACH_GIT_COMMIT"` pins the tag to the current commit.

## Plugins

Plugins are typed external executables in any language. The plugin `type` determines when Bachkator runs the executable and which stdout contract it must satisfy.

Existing plugins default to `type = "graph"`. Graph plugins run while loading the Project and emit graph evidence JSON to stdout.

```hcl
plugin "ts_imports" {
  type    = "graph"
  command = ["bun", "examples/plugins/ts-import-graph.ts"]
  sources = {
    api_tests = ["packages/api/tests/**/*.test.ts"]
  }
}

shell "test-api" {
  command = ["bun", "test", "packages/api/tests"]
  inputs  = [plugin.ts_imports.api_tests]
}
```

Graph plugin environment:

- `BACH_PLUGIN_NAME`: plugin name.
- `BACH_PROJECT_ROOT`: project root.
- `BACH_PLUGIN_INPUTS`: resolved plugin input paths, newline-separated.
- `BACH_PLUGIN_SOURCES`: JSON-encoded `sources` map.

Graph plugin stdout contract:

```json
{
  "inputs": {
    "api_tests": ["packages/api/src/main.ts"]
  },
  "targets": {
    "shell.test-api": {
      "depends_on": ["shell.generated"],
      "inputs": ["generated.ts"]
    }
  }
}
```

Bachkator merges graph plugin-provided `depends_on` and `inputs` into existing targets before validation, fingerprinting, and scheduling.

Graph plugins should not perform side effects. They run on graph load, so `bach list` also runs graph plugins.

Quality plugins use `type = "quality"`. They do not run while loading the graph. They run only after a target command succeeds and a quality report declaration references them with `parser = plugin.<name>`.

```hcl
plugin "eslint_quality" {
  type    = "quality"
  command = ["bun", "scripts/bach/parse-eslint-quality.ts"]
  timeout = "10s"
  env     = ["MODE=strict"]
}
```

Quality plugins receive the report path as the first command argument and through environment metadata such as `BACH_QUALITY_REPORT_ABS_PATH`, `BACH_QUALITY_KIND`, `BACH_TARGET`, and `BACH_RUN_ID`. Stdout must contain only normalized quality JSON; stderr is diagnostics and is copied to the target log.

See `examples/plugins/quality-parser` for a complete quality parser plugin example. The normalized stdout schema lives at `docs/schemas/quality-plugin-report.schema.json`.

Quality plugin fields:

- `command` or `shell`: parser executable. One is required; they are mutually exclusive.
- `timeout`: optional parser timeout. Defaults to `30s`.
- `env`: optional environment entries layered onto Bach's runtime environment.

Quality plugins do not support `sources` or `inputs`. Graph plugins do not support `timeout`.

Quality plugin environment:

- `BACH_PLUGIN_NAME`: plugin name.
- `BACH_PLUGIN_TYPE`: `quality`.
- `BACH_PROJECT_ROOT`: project root.
- `BACH_RUN_ID`: current run ID.
- `BACH_RUN_DIRECTORY`: target run directory.
- `BACH_TARGET`: target address being parsed, such as `shell/lint`.
- `BACH_QUALITY_KIND`: report kind, such as `lint` or `tests`.
- `BACH_QUALITY_REPORT_PATH`: report path as declared in the Bachfile after env expansion.
- `BACH_QUALITY_REPORT_ABS_PATH`: absolute report path.

## TypeScript Import Graph Plugin

`examples/plugins/ts-import-graph.ts` is self-contained and only needs Bun. It does not require the `typescript` package.

It supports:

- glob source entries.
- relative imports.
- `.js` specifiers resolving to `.ts` and `.tsx` files.
- JSON leaf inputs.
- `@app/*` workspace imports through `package.json` exports.
- TSX parsing via Bun's `tsx` loader.
- shebang stripping before scanning imports.

## Bun Package Graph Plugin

`examples/plugins/bun-package-graph.ts` is an example workspace-level plugin for Bun monorepos that use `@app/*` packages.

```hcl
plugin "bun_packages" {
  command = ["bun", "examples/plugins/bun-package-graph.ts"]
  sources = {
    api = ["packages/api"]
  }
}

shell "test-api" {
  command = ["bun", "test", "packages/api"]
  inputs  = [plugin.bun_packages.api]
}
```

The plugin reads root `package.json` workspaces, package names, exports, dependencies, and dev dependencies. For each configured source package, it emits a deterministic plugin input containing that package and its transitive workspace dependency closure, so changes in shared packages affect dependent target suggestions through `bach affected`.

## Fingerprints And Cache

A target is cacheable when it declares `inputs` or `outputs`.

The fingerprint includes:

- operation configuration.
- target environment.
- dependency fingerprints.
- resolved input file contents.
- output existence.

Fingerprints are stored in SQLite table `target_state` in `.bach/state.db`.

When a cacheable target is stale, Bach prints the cache invalidation reasons before the operation. Dry-run JSON includes the same values in `targets[].cache.reasons` so agents can inspect why a target will run without executing it.

Stale reasons include changed inputs, changed environment, changed operation configuration, dependency fingerprint changes, missing outputs, forced runs, missing cache records, and legacy fingerprint changes from older cache state.

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

## Git Environment

Every target operation receives Git context from the project root:

```text
BACH_GIT_BRANCH
BACH_GIT_COMMIT
BACH_GIT_DIRTY
BACH_GIT_DIRTY_SUFFIX
BACH_GIT_STAGED_FILES
BACH_GIT_UNSTAGED_FILES
BACH_GIT_UNTRACKED_FILES
BACH_GIT_CHANGED_FILES
```

Command arrays and shell strings expand `$NAME` and `${NAME}` from the runtime environment before execution.

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

