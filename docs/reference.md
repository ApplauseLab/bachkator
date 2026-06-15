<!-- Generated from docs/reference/*.md. Edit fragments, then run shell/docs-generate. -->

# Bachkator Reference

Bachkator reads `Bachfile` from the current directory by default. The file is HCL with a Vim-friendly modeline often used at the top:

```hcl
# vim: set ft=hcl :
```

## CLI

```sh
bach list
bach run <target> [target ...]
bach affected [path ...]
bach init [--provider opencode]
bach factory submit <factory> --title <title>
bach factory list <factory>
bach factory inspect <factory> <work-item-id>
bach factory cancel <factory> <work-item-id> --reason <text>
bach factory approve <factory> <work-item-id> --phase <phase>
bach factory start <factory>
bach factory status <factory>
bach plan status <plan-file> [plan-file ...]
bach plan implement <plan-file>
bach backend sqlite
bach provenance <path> [path ...]
bach artifacts [run-id]
bach explain <target>
bach graph
bach quality [summary|reports|metrics|findings|gates|slow-targets|failing-tests]
bach report init --role <role> [--name <name>] [--summary <text>] [--path <file>]
bach report finding --kind <kind> [--stdin] [--file <path>] [--line <n>] [--duration-ms <n>]
bach report metric --name <name> --value <number> [--scope <scope>] [--unit <unit>]
bach report status <success|failed> [--summary <text>]
bach validate
bach reference [topic]
bach runs list
bach runs inspect <run-id>
bach logs <run-id>
```

Supported flags:

- `-f, --file <path>`: load a different Bachfile. With `init`, choose the Bachfile path to create and write `AGENTS.md` beside it. Default is `Bachfile`.
- `-a, --aliases`: with `list`, include target aliases and their canonical targets.
- `--generated`: with `list`, include generated policy fan-out nodes.
- `-v, --verbose`: with `list`, include cost and risk metadata; with `run`, stream quiet target output.
- `--runs-limit <n>`: maximum runs or artifacts to list. Defaults to `10`; use `0` for all.
- `--target <target>`: filter `runs list` or `artifacts` by target address.
- `--status <status>`: filter `runs list` or `artifacts` by run status; with `factory list`, filter Work Items by lifecycle. Factory list defaults to `pending` and `waiting_approval`; use `all` for every lifecycle.
- `--since <duration|time>`: filter `runs list` or `artifacts` by age or RFC3339 timestamp.
- `--artifact <path>`: filter `runs list` or `artifacts` by artifact path substring.
- `--failed`: with `logs`, show only failed target logs.
- `--last <n>`: with `logs`, show only the last N log lines.
- `--errors`: with `logs`, show only likely error/failure lines.
- `-d, --dry-run`: print the planned operations without executing them.
- `-p, --provider opencode`: with `init`, delegate starter Bachfile and agent-instruction generation to OpenCode.
- `--force`: run cacheable targets even when their fingerprints are fresh.
- `-y, --yes`: confirm execution of targets marked `requires_confirmation = true`, including generated `bach plan implement` agent targets.
- `--env-file <path>`: load target operation environment from this file after project `.env`.
- `--profile <name>`: select an environment profile. May be repeated; later profiles win.
- `--log-only`: suppress command stdout/stderr in the terminal while keeping Bach progress and quality progress visible; full command output is still written to `.bach/runs/.../*.log` files. Provider adapters may redact normal progress logs and keep complete provider telemetry in private run artifacts.
- `-j <n>`: maximum number of targets to run in parallel.
- `--var name=value`: set a Bachkator variable. May be repeated.
- `--json`: with `run --dry-run`, print a machine-readable execution plan; with `runs inspect`, print a machine-readable run export for failures, agent reports, policy evidence, and control-plane ingestion; with `provenance`, print path provenance records; with `validate`, print machine-readable diagnostics; with `factory`, print command-specific Factory JSON output; with `plan status`, print Plan status JSON; with `plan implement`, print Plan implementation JSON.
- `--format <mermaid|json>`: choose the `graph` output format.
- `--version`: print the Bachkator version.
- `--workflow <name>`: with `factory submit`, select a workflow when a Factory has more than one workflow; with `factory list`, filter by workflow.
- `--title <text>`: with `factory submit`, set the Work Item title.
- `--body <text>`: with `factory submit`, set the Work Item body.
- `--body-file <path>`: with `factory submit`, read the Work Item body from a project-relative file.
- `--priority <value>`: with `factory submit`, set the priority. Defaults to `normal`.
- `--label <value>`: with `factory submit`, attach a label. May be repeated.
- `--dedupe-key <value>`: with `factory submit`, reuse an existing open item with the same factory, workflow, and key.
- `--plan <value>`: with `factory submit`, store an opaque submitted plan reference.
- `--reason <text>`: with `factory cancel`, record the cancellation reason; with `factory approve`, record the approval reason.
- `--phase <phase>`: with `factory approve`, select the gated phase to approve, such as `plan` or `deploy.production`.
- `--poll-interval <duration>`: with `factory start`, set the queue poll interval. Defaults to `5s`.
- `--renew-interval <duration>`: with `factory start`, set the daemon lease renewal interval. Defaults to `10s`.
- `--lease-ttl <duration>`: with `factory start`, set the daemon lease expiration time. Defaults to `30s`.

Global flags (`--file`, `--var`, `--profile`, `--json`, `--verbose`, `--version`) may appear before the subcommand. Command-specific flags must appear after the subcommand they apply to, for example `bach run --dry-run shell/test` and `bach plan implement --yes plans/example.md`.

### Exit codes

- `0`: success.
- `1`: general failure, including target failures, validation errors, and uncaught runtime errors.
- `2`: invalid arguments or command usage, such as missing required positional arguments.

`bach report` writes public `bach.agent_report.v1` quality report artifacts for review agents and scripts. It does not load a Bachfile and does not write the State Store directly; normal quality declarations ingest the artifact after the target finishes.

`bach backend sqlite` starts the bundled SQLite Backend Provider as a low-level JSON-RPC stdio process.
It is launched by Bach from resolved `project.backend` configuration for Backend writes, is not a Target,
is not listed by `bach list`, and waits for `bach.backend.v1` protocol messages on stdin when run directly.

Destination resolution for every `bach report` subcommand is `--path`, then `BACH_AGENT_QUALITY_REPORT_PATH`, then `$BACH_RUN_DIRECTORY/agent-report-v1.json`. Report paths must stay under the current workspace or `BACH_RUN_DIRECTORY` unless `--allow-external-path` is set. `BACH_AGENT_REPORT_PATH` is reserved for Agent Target completion and merge reports and is not used by `bach report`.

Identity flags are shared across `init`, `finding`, and `status`: `--role`, `--name`, and `--summary`. `report metric --name` is the metric name; use `BACH_REPORT_AGENT_NAME` when metric-only calls need agent identity. Missing reports are auto-created with `status = "success"`, summary `Report initialized by bach report.`, `agent.role` from `--role`, `BACH_REPORT_AGENT_ROLE`, `BACH_AGENT_ROLE`, or `reporter`, and `agent.name` from `--name` or `BACH_REPORT_AGENT_NAME`. Subject metadata is populated from `BACH_AGENT_TARGET`, `BACH_AGENT_SUBJECT_WORKSPACE`, and `BACH_AGENT_SUBJECT_COMMIT` when present.

Use `bach report finding --stdin` to read one strict JSON finding object from stdin. Unknown fields are rejected. Adding a blocking finding records evidence but does not change final status; use `bach report status failed` or a quality gate over parsed policy metrics when the target should fail.

`bach run` accepts one or more target addresses such as `shell/test`, `image/app`,
`pipeline/release`, or `agent/example`. Multiple requested targets are planned as one deduplicated run, so shared
dependencies execute once. Requested targets are not ordered left-to-right; use a `pipeline` target
when strict sequencing is required.

`bach run` also accepts generated policy nodes such as `policy.merge@agent.checkout_refactor`.
Policy nodes must be run separately from normal targets. They fan out their `required_targets` in
the subject workspace and write policy evaluation JSON under that workspace's `.bach/artifacts`.

With `--dry-run --json`, the plan includes `requested_targets` with the canonical target addresses
that seeded the run. The legacy `target` field is the space-joined requested target list.

`bach init` creates starter Bach adoption files in a repository that does not already have them. Plain
`bach init` writes a minimal valid `Bachfile` and an `AGENTS.md` beside it with Bach-first operating
guidance. It refuses to overwrite either file. `bach init --provider opencode` writes a provider prompt
under `.bach/init/`, runs `opencode run <prompt>`, verifies staged provider files under
`.bach/init/outputs/`, then installs them without overwriting existing files. Use `bach init --dry-run`
to print planned writes without creating files, or `bach init --dry-run --provider opencode` to print the
provider command without writing the prompt or invoking OpenCode. Dry-run still reports overwrite
preflight failures.

`bach provenance <path> [path ...]` explains whether each path is generated by declared target outputs
or consumed by declared target inputs. Paths are relative to the project root unless absolute. Missing
declared outputs still map to their producer. Unknown paths succeed with empty producers and consumers.
Use `bach provenance --json <path>` for strict JSON output.

`bach quality` reads parsed quality reports from the state database. `summary` shows recent reports,
quality gates, slow targets, and top failing tests. Use `metrics` for normalized values such as
`coverage.line.percent`, `findings` for parsed Checkstyle/JUnit-style findings, and `gates` for threshold
results.

`bach validate` parses, decodes, and validates the configured Bachfile without planning or running
targets. It also skips graph plugin execution so editor and agent checks are fast and side-effect free.
Successful human output summarizes configured targets, aliases, inputs, and profiles:

```text
Bachfile valid: 14 targets, 2 aliases, 3 inputs, 2 profiles
```

Invalid human output uses problem-matcher-friendly `file:line:column: message` lines and exits non-zero.
With `--json`, `bach validate` writes strict JSON to stdout and exits non-zero when diagnostics contain
errors:

```json
{
  "valid": false,
  "files": ["Bachfile"],
  "summary": {
    "targets": 0,
    "aliases": 0,
    "inputs": 0,
    "profiles": 0
  },
  "diagnostics": [
    {
      "severity": "error",
      "file": "Bachfile",
      "range": {
        "start": {"line": 2, "column": 13},
        "end": {"line": 2, "column": 25}
      },
      "message": "obsolete target reference \"shell/test\": use type.name, for example shell.test",
      "code": "obsolete-target-reference"
    }
  ]
}
```

Diagnostic severities are strings and may be `error`, `warning`, `info`, or `hint`; current Bachfile
validation emits `error` and HCL `warning` severities. Diagnostic codes are stable identifiers such as
`hcl-parse-error`, `hcl-decode-error`, `obsolete-target-reference`, `unknown-target-reference`, and
`duplicate-target`.

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

## File Provenance

Use `bach provenance <path> [path ...]` to explain which declared targets generate or consume files.

```sh
bach provenance docs/reference.md
bach provenance --json internal/runner/plan.go
```

Paths are interpreted relative to the project root unless they are absolute. Missing files can still report provenance when they match declared target outputs.

Human output is optimized for agents deciding whether to edit a file directly or regenerate it from sources. For broad source declarations, generated files may also list consumers; this example is abridged:

```text
docs/reference.md
generated: true
source: false
generated_by:
  - shell/docs-generate
    operation: go run ./cmd/bach-docs-gen
    regenerate: bach run shell/docs-generate
consumed_by:
  - shell/build
    operation: sh -c mkdir -p dist && go build -ldflags '-X main.version=dev' -o dist/bach ./cmd/bach
status: unknown
```

JSON output emits one record per queried path:

```json
{
  "paths": [
    {
      "path": "docs/reference.md",
      "generated": true,
      "source": false,
      "producers": [
        {
          "target": "shell/docs-generate",
          "operation": "go run ./cmd/bach-docs-gen",
          "regenerate_command": "bach run shell/docs-generate",
          "outputs": ["docs/reference.md"],
          "inputs": ["cmd/bach-docs-gen", "docs/reference"]
        }
      ],
      "consumers": [
        {
          "target": "shell/build",
          "operation": "sh -c mkdir -p dist && go build -ldflags '-X main.version=dev' -o dist/bach ./cmd/bach",
          "regenerate_command": "bach run shell/build",
          "outputs": ["dist/bach"],
          "inputs": ["cmd", "docs", "internal"]
        }
      ],
      "status": "unknown",
      "reasons": []
    }
  ]
}
```

Directory inputs and outputs match files beneath them. Unknown paths return success with empty `producers` and `consumers`.

## Target Aliases

Use top-level `alias` blocks to preserve old command names while directing users and agents to canonical targets.

```hcl
alias "staging-kristiyan-deploy" {
  target      = "pipeline.deploy-kristiyan"
  deprecated = "Use pipeline.deploy-kristiyan."
}
```

Aliases resolve before planning and execution, so dry-runs, locks, cache keys, logs, and run history use canonical target names. Aliases do not create executable target nodes.

`deprecated` is optional. When present, Bachkator prints the message when the alias is used and includes it in `bach explain <alias>`.

List aliases with:

```sh
bach list --aliases
```

Alias targets must point directly to real targets. Alias-to-alias chains are rejected at load time.

## Target Explain

`bach explain <target>` prints target guidance without running anything. It includes description, when to use the target, cost, risk flags, dependencies, pipeline steps, inputs, outputs, and produced resources. If `<target>` is an alias, explain also prints the alias name, canonical target, and optional deprecation message. If `<target>` is a generated policy node, explain prints `generated: true`, the policy subject, subject workspace/commit scope, and required targets.

```sh
bach explain shell/test-api
bach explain policy.merge@agent.checkout_refactor
```

Use explain before high-cost, remote, destructive, or unfamiliar targets.

## Target Metadata

Targets support optional guidance metadata:

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

Risk metadata is inherited through `depends_on` and pipeline `steps`, so aggregate targets show and enforce the risk of the targets they run. Dry-runs are always allowed. Real execution of a target whose effective risk includes `requires_confirmation` must use `--yes`.

## Environment Files

Bachkator loads `.env` from the project root into target operation environments when the file exists. Use `--env-file .env.local` to overlay another file on top of `.env`.

Environment precedence for operations is:

- parent process environment.
- project `.env` values.
- `--env-file` values.
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

Select profiles with `--profile`. The flag may be repeated, and later profiles override earlier profile values:

```sh
bach --profile staging --profile staging-kristiyan run shell/render
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

## Bachfile Imports

Use top-level `import "path"` declarations to split a Bachfile into local reusable fragments.

```hcl
project "example" {
  default = "shell.test"
}

import "./bach/go.bach"
import "./bach/docs.bach"
```

Import paths are string literals resolved relative to the file containing the import. Imported files share the root project's scope, and their declarations behave as if pasted at the import site.

Imported fragments can contain variables, environment blocks, profiles, inputs, resources, plugins, aliases, policy blocks, quality blocks, and targets. They must not introduce a second effective `project` block.

Each canonical local file is loaded at most once. Re-importing the same file is allowed and deduped, while duplicate declarations from different files remain errors. Import cycles, missing files, and invalid imported files fail project loading with diagnostics that include the importing location or imported file path.

Remote imports, glob imports, environment-expanded paths, registries, and version pinning are not supported.

## Project

Every Bachfile needs one `project` block:

```hcl
project "example" {
  root    = "."
  default = "shell.test"
}
```

Fields:

- `root`: project working directory. Defaults to the Bachfile directory.
- `default`: target to run when no target is provided.

`project.state` is no longer supported. Omit `backend` to use the default bundled SQLite Backend Provider at `.bach/state.db`.

The omitted `backend` is equivalent to:

```hcl
project "example" {
  root = "."

  backend {
    type    = "stdio"
    command = ["bach", "backend", "sqlite"]
    config = {
      path = ".bach/state.db"
    }
  }
}
```

Backend fields:

- `type`: Backend transport. Phase 1 supports only `stdio`.
- `command`: argv array for the Backend Provider. Phase 1 supports only `["bach", "backend", "sqlite"]`.
- `config`: provider-owned object. The SQLite provider supports `path`, resolved relative to the project root.

## Bachfile Imports

Root Bachfiles can import local Bachfile fragments to keep reusable target packs in separate files:

```hcl
import "./bach/go.bach"
import "./bach/docs.bach"

project "example" {}
```

Import paths must be string literals and are resolved relative to the file that contains the import declaration. Imported files are local files only; HTTP, Git, registry, glob, environment-expanded, and computed import paths are not supported.

Imported files share the same project scope as the root Bachfile. Targets, aliases, policies, variables, inputs, resources, plugins, profiles, and quality blocks from imported files participate in the same validation rules as root declarations, so duplicate declarations are errors. The root Bachfile must own the `project` block; imported files must not declare one.

The same canonical file can be imported more than once and is loaded only once. Import cycles fail during project loading with a diagnostic that includes the cycle path.

## Backend Provider Protocol

Bach Backend Providers speak `bach.backend.v1` over stdio JSON-RPC with `Content-Length` framing. Headers are capped at 16 KiB and payloads are capped at 8 MiB.

The bundled provider entrypoint is:

```sh
bach backend sqlite
```

Provider stdin and stdout are protocol-only. Diagnostics belong on stderr.

Initialization uses `backend.initialize` with the protocol version, Project name/root, and provider config. The bundled SQLite provider supports the capabilities `runs`, `evidence_refs`, `quality_reports`, `findings`, `factory_queue`, and `plan_ledger`.

Current method names are:

- `backend.initialize`
- `backend.shutdown`
- `runs.create`
- `runs.startTarget`
- `runs.finishTarget`
- `runs.finish`
- `runs.get`
- `runs.list`
- `evidence.recordRef`
- `evidence.listRefs`
- `quality.recordReport`
- `quality.recordReports`
- `findings.recordObservation`
- `findings.get`
- `findings.listCurrent`
- `findings.listEvents`
- `factory.enqueueWorkItem`
- `factory.getWorkItem`
- `factory.listWorkItems`
- `factory.cancelWorkItem`
- `factory.acquireDaemonLease`
- `factory.renewDaemonLease`
- `factory.releaseDaemonLease`
- `factory.claimWorkItem`
- `factory.updateWorkItemPhase`
- `factory.completeWorkItem`
- `factory.failWorkItem`
- `factory.getDaemonStatus`
- `plans.recordLedger`
- `plans.getLedger`

`runs.finish` accepts a run finish payload with:

- `run`: the final Run record.
- `targets`: changed target fingerprint records keyed by Target Address.
- `target_runs`: per-target execution records keyed by Target Address.
- `evidence`: evidence references associated with the completed Run.

Evidence references used by `runs.finish`, `evidence.recordRef`, and `evidence.listRefs` follow
`bach.backend.evidence_ref.v1`. They may include `created_at` as an RFC3339Nano timestamp; providers stamp
omitted values at write time.

`quality.recordReports` accepts a batch payload with `reports` and `gates` arrays. `quality.recordReport` accepts a single report.

`factory.enqueueWorkItem` accepts a Work Item, initial attempt, submitted event, and optional dedupe event.
`factory.getWorkItem` and `factory.cancelWorkItem` operate by factory name and Work Item ID.
`factory.listWorkItems` operates by factory name with an optional lifecycle status filter.
Daemon methods acquire and renew Factory leases, claim pending Work Items, record phase status, complete or
fail Work Items, and read daemon status without exposing table-level writes.
Work Item JSON follows `docs/schemas/backend-factory-work-item-v1.schema.json`.

`plans.recordLedger` appends one immutable Plan ledger and its evidence entries transactionally. Repeating the same `ledger_id` with an identical payload is idempotent; repeating it with a different payload returns `conflict`. `plans.getLedger` takes `plan_id` and returns the latest ledger by `recorded_at`, then `ledger_id`, or `not_found` when no ledger exists. Plan ledger JSON follows `docs/schemas/backend-plan-ledger-v1.schema.json`.

The `approvals` capability name is reserved for future approval operations; the current bundled provider does
not advertise it.

Public provider DTOs and helpers are available from:

- `github.com/applauselab/bachkator/pkg/backendprotocol`
- `github.com/applauselab/bachkator/pkg/jsonrpcstdio`

Machine-readable schemas live under `docs/schemas/backend-*.schema.json`.

Domain errors are JSON-RPC errors with Bach error codes in `error.data.code`, including `invalid_request`, `not_initialized`, `unsupported_capability`, `not_found`, `conflict`, `validation_failed`, and `internal`.

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

- `--var release_version=v0.1.0`
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

## Prompt Blocks

Top-level `prompt` blocks register prompt files that agent/provider workflows can reference by name:

```hcl
prompt "implementer" {
  path        = "prompts/agents/implementer.md"
  description = "Default implementation-agent prompt"
  version     = "v1"
}
```

`path` is required and must be project-relative. It must point to an existing regular file inside the project root after symlink resolution. `description` and `version` are passive metadata for operators and future provider integrations.

Prompt blocks do not execute anything and do not affect target fingerprints by themselves.

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
- `requires_confirmation`: set to `true` when operators should confirm intent before running. Real execution then requires `--yes`; dry-run is still allowed.
- `depends_on`: explicit target dependencies.
- `lock`: optional in-run named lock. Ready targets with the same lock do not run concurrently in one Bachkator invocation.
- `timeout`: optional Go-style duration such as `30s`, `5m`, or `1h`. The timeout bounds target operation execution and completion-contract checks.
- `retry`: optional operation retry policy. `attempts` is the total number of attempts; `backoff` is an optional duration to wait between failed attempts.
- `command`: argv array executed directly.
- `shell`: shell string executed via `/bin/sh -c`.
- `tools`: required host tools checked before execution. Each entry has `name` plus optional `command`, `version`, and `fix` fields. `command` is an exact probe command; Bachkator does not parse versions yet.
- `preflights`: credential or session probes checked after required tools and before target execution. Each entry has `command` plus `name` or `kind`, and optional `fix`. Bachkator treats these as generic host checks and does not hardcode provider-specific auth behavior.
- `quiet`: when `true`, write this target's progress and operation output only to `.bach/runs/.../*.log` unless `--verbose` is set.
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

`bach run --dry-run pipeline/deploy-staging` prints the pipeline and then each step in execution order. `bach run pipeline/deploy-staging` stops at the first failed step, so later steps do not run. Step targets remain runnable directly for debugging.

Pipeline targets inherit risk metadata from their steps. If any step has `requires_confirmation = true`, running the pipeline requires `--yes`; `--dry-run` still works without confirmation.

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
bach --var release_version=v0.1.0 run shell/github-release
```

`gh release create <tag>` creates the Git tag when it does not exist. `--target "$BACH_GIT_COMMIT"` pins the tag to the current commit.

## Policy Fan-Out

`policy` blocks define generated policy nodes that run required targets against a subject workspace.
The generated node address is `policy.<name>@<subject>`.

```hcl
policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = ".bach/agents/checkout_refactor"
  subject_commit    = "0123456789abcdef0123456789abcdef01234567"
  required_targets  = [group.gate]
}
```

Fields:

- `subject`: required subject identifier used in the generated address.
- `subject_workspace`: optional workspace where required targets run. Relative paths resolve from the project root.
- `subject_commit`: optional Git commit that the subject workspace must have checked out before policy evaluation starts.
- `required_targets`: required target references such as `group.gate` or `shell.test`.

Generated policy nodes are hidden from the default target list. Use:

```sh
bach list --generated
bach explain policy.merge@agent.checkout_refactor
bach run --dry-run policy.merge@agent.checkout_refactor
bach run policy.merge@agent.checkout_refactor
```

Policy nodes can be run directly. Implementer agents that reference a `policy` invoke a generated
`policy/<name>@agent.<subject>` target after implementation evidence passes, so required targets,
reviewers, policy logs, quality parsing, and applied-policy verdicts are recorded under a visible policy
target run instead of hidden inside the implementation agent. Required targets keep their normal target
identity in run records and logs, but execute with the subject workspace as project root. State and
artifacts for standalone subject-policy fan-out are written under the subject workspace's `.bach`
directory, so cached results from the main checkout cannot satisfy subject policy checks.

When `subject_commit` is set, required targets receive these environment variables:

- `BACH_POLICY_SUBJECT`
- `BACH_POLICY_SUBJECT_COMMIT`
- `BACH_POLICY_NODE`

Standalone subject-policy fan-out writes evaluation JSON to:

```text
<subject_workspace>/.bach/artifacts/<policy-node>.json
```

The JSON includes `policy`, `subject`, `subject_workspace`, `subject_commit`, `status`,
`required_targets`, `findings`, and `created_at`. Failed required targets add
`required_target_error`. If required targets mutate files outside `.bach` and declared target outputs,
the policy fails with `policy-required-target-mutated-workspace`. If `subject_commit` does not match
the subject workspace HEAD, the policy fails with `policy-subject-commit-mismatch`.

Quality-enabled policy runs, including generated policy targets invoked by implementer agents, also write
subject-keyed applied-policy verdict artifacts in the project root:

```text
.bach/artifacts/policies/<run-id>/<sanitized-target>.json
```

Sanitization replaces `/`, `:`, and spaces with `-`.

Merge agents consume the latest matching applied-policy artifact only when the artifact verdict passed, its
`subject_workspace` matches the merge subject workspace, its `subject_commit` matches the subject
workspace HEAD, its `policy_target` names the generated policy target for the subject's current
policy, and that exact policy target succeeded in the recorded run.

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

Use `bach runs list` to list the 10 most recent prior non-dry-run runs, statuses, timestamps, and log directories. Use `bach runs list --runs-limit 0` to list all recorded non-dry-run runs.

Use `bach runs inspect <run-id>` to inspect one completed run. The human output highlights failed targets, exit codes, log paths, parsed quality reports, failed quality gates, and declared preflight/tool fixes. Use `--json` for an agent-readable run export:

```sh
bach runs inspect --json 20260608T120000.000000000Z
```

The JSON object includes:

- `run_id`, `requested_target`, `status`, `started_at`, `finished_at`, and `log_dir`.
- `targets[]` with each target execution's name, status, operation, log path, artifact paths, quality report summaries, agent reports, applied-policy summaries, provider metadata, subject metadata, and merge evidence when present.
- `failed_targets[]` with `target`, `status`, optional `exit_code`, `operation`, `log_path`, `artifacts`, `quality`, preflight/tool diagnostics, and a bounded `log_excerpt`.
- `quality.reports[]` with report path, parser status, metric count, finding count, and parser message when present.
- `quality.failed_gates[]` with metric, operator, threshold, actual value, and message.
- `suggested_fixes[]` from declared tool or preflight `fix` fields.

The JSON export intentionally links to local artifact and log paths instead of embedding every raw finding
or full log line. Treat those artifacts as local evidence that may contain provider output or sensitive
project details.

Use `bach logs <run-id>` for concise log slices without opening large files manually:

```sh
bach logs 20260608T120000.000000000Z --failed --last 80
bach logs 20260608T120000.000000000Z --target shell/test --errors
```

`--failed` limits output to failed target logs. `--target` selects one target. `--last N` bounds output. `--errors` keeps lines containing likely error/failure terms.

Each run ends with a concise terminal summary, including the run ID, status, requested target or
targets, duration, log directory, and target status counts. A multi-target invocation stores the
requested target list as a space-joined value in run history:

```text
run 20260608T120000.000000000Z success target=shell/test duration=1.2s logs=.bach/runs/20260608T120000.000000000Z
targets: success=3 cached=2 failed=0 dry-run=0 running=0
```

```text
run 20260608T120500.000000000Z success target=shell/lint shell/test duration=2.4s logs=.bach/runs/20260608T120500.000000000Z
targets: success=4 cached=1 failed=0 dry-run=0 running=0
```

`--log-only` suppresses command stdout/stderr in the terminal while keeping Bach progress, quality progress, and the final summary visible. `quiet = true` targets suppress their command output and target progress unless `--verbose` is set. Full command target output is still written to the target log so agents can report the outcome and log location. Provider adapters may redact normal progress logs and keep complete provider telemetry in private run artifacts. Failed runs include the last 20 non-empty lines from the first failed target log; successful runs do not print log excerpts.

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

## Agent Targets

Agent Targets run implementation and merge providers through generated prompt and context artifacts.
Implementer agents can attach reviewer policies that run `mode = "review"` agents and publish
quality-gated policy evidence. Merge agents consume that passing policy evidence before invoking a
serialized provider.

```hcl
provider "opencode" {
  type = "opencode"
}

provider "generic_opencode" {
  type    = "agent"
  command = ["opencode", "run"]
}

prompt "implementer" {
  path        = "prompts/agents/implementer.md"
  description = "Default implementation instructions"
  version     = "v1"
}

prompt "architecture_review" {
  path = "prompts/agents/architecture-review.md"
}

prompt "merge" {
  path = "prompts/agents/merge.md"
}

agent_template "feature_implementer" {
  mode     = "implement"
  provider = provider.opencode
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    path = ".bach/agents/feature-implementer"
  }

  git {
    branch = "bach/agents/feature-implementer"
  }
}

agent "checkout_refactor" {
  template = agent_template.feature_implementer
  plan     = "plans/checkout-refactor.md"

  workspace {
    mode = "clone"
    path = ".bach/agents/checkout_refactor"
  }

  git {
    branch = "bach/agents/checkout_refactor"
    commit = "required"
  }
}

agent "architecture_review" {
  mode     = "review"
  provider = provider.generic_opencode
  role     = "architecture-reviewer"
  prompt   = prompt.architecture_review
}

agent "merge_checkout_refactor" {
  mode     = "merge"
  provider = provider.generic_opencode
  prompt   = prompt.merge
  subject  = agent.checkout_refactor
}

policy "merge_review" {
  reviewers = [agent.architecture_review]

  quality_gate {
    metric = "findings.error.open.count"
    max    = 0
  }
}
```

Provider blocks:

- `type`: `opencode` for the first-class OpenCode provider, or `agent` for a generic command provider.
- `command`: required only for generic `type = "agent"` providers. Bach expands environment variables and appends the generated prompt path as the final argument.

First-class OpenCode providers do not accept `command` and are supported for `mode = "implement"` Agent Targets. Bach owns the argv and invokes `opencode run --format json "Follow the attached Bach agent prompt." --file <generated-prompt>` so it can capture JSONL provider evidence and session IDs while ensuring OpenCode receives the generated Bach contract. On improvement attempts after attempt 1, Bach resumes the captured session with `opencode run --format json --session <sessionID> "Follow the attached Bach agent prompt." --file <generated-prompt>`. Review and merge agents that should run OpenCode directly can use a generic `type = "agent"` provider with `command = ["opencode", "run"]`.

OpenCode JSONL is provider evidence, not target success evidence. Malformed JSONL or a successful OpenCode process that emits no `sessionID` fails the provider attempt. Bach mirrors OpenCode assistant text events and tool names to the target log/stdout as readable progress while preserving the complete provider event stream, including tool inputs, outputs, titles, and descriptions, in raw JSONL. Normal progress output does not include raw tool arguments.
Raw OpenCode JSONL capture is capped at 10 MiB per attempt, individual JSONL events are capped at 1 MiB, and mirrored provider text/tool call summaries are capped at 1 MiB. Exceeding a cap fails the provider attempt after Bach drains the provider process output.

Prompt blocks:

- `path`: project-relative Markdown instructions file. Agent Targets that reference the prompt treat the file as an implicit input.
- `description`: optional passive metadata.
- `version`: optional passive metadata.

Prompt files provide reusable task guidance only. Bach always appends generated context and the required
structured report schema to the prompt passed to the provider. If an agent omits `prompt`, Bach uses a
baked-in default task prompt and still injects the same report schema.

Agent Template blocks:

- `agent_template "<name>"` declares reusable Agent Target defaults; it is not a Target, cannot run, and is hidden from `bach list`.
- `provider`: required typed reference such as `provider.opencode`.
- `mode`: optional `implement`, `review`, or `merge`; defaults to `implement`.
- `role`: optional report/log identity metadata.
- `prompt`: optional typed reference such as `prompt.implementer`.
- `policy`: optional typed reference supported only when `mode = "implement"`.
- `workspace` and `git` use the same block shape as Agent Targets.
- `plan` and `subject` are not valid on templates because concrete agents or future Factory Work Items provide concrete context.
- Template placeholders are valid only in `workspace.path` and `git.branch`: `${work_item.id}`, `${work_item.slug}`, `${plan.id}`, `${workstream.id}`, `${factory.name}`, and `${workflow.name}`.
- Concrete `agent` blocks can set `template = agent_template.<name>`; explicit fields and blocks on the agent override inherited template defaults.
- Runnable Agent Targets must be concrete. `bach validate` rejects an agent that still has unresolved template placeholders after inheritance.

Provider processes are untrusted with respect to Bach-owned policy evidence. During provider execution,
Bach fails the attempt if `.bach/artifacts/policies`, target cache state, policy-target run state, Plan ledger/evidence rows, or Factory approval records change outside Bach's own writes.
Implementer and reviewer providers require the main checkout to be clean before invocation and fail if
the provider changes main checkout HEAD, branch, Git metadata, ignored files, or non-`.bach` status.

Agent fields:

- `template`: optional typed reference such as `agent_template.feature_implementer`; inherited values are applied before validation.
- `mode`: `implement`, `review`, or `merge`. Managed workspaces and git evidence apply to `implement` agents.
- `provider`: required typed reference such as `provider.opencode`.
- `role`: optional report/log identity metadata.
- `prompt`: optional typed reference such as `prompt.implementer`.
- `plan`: required project-relative implementation plan path for `mode = "implement"`; not used by review or merge agents. The plan file is an implicit input.
- `subject`: required for `mode = "merge"`; must be an explicit `agent.<name>` reference to the implementer being merged.
- `policy`: optional typed reference such as `policy.merge_review`; supported on implementer agents.
- `workspace.mode`: defaults to `clone` and must be `clone` in v1.
- `workspace.path`: defaults to `.bach/agents/<agent-name>` and must stay under `.bach/agents`.
- `git.branch`: defaults to `bach/agents/<agent-name>`.
- `git.commit`: defaults to `required`; allowed values are `required` and `optional`.
- `improve.max_attempts`: optional number of policy-informed attempts for implementer agents.
- `improve.until`: currently supports `policy.passed`.

Agent Targets always execute when requested; prompt and plan inputs still participate in `bach affected`.
Bach creates or reuses the workspace clone for each run. New workspaces are cloned from the project root and
checked out on the configured branch. Existing workspaces must be clean git clones at run start, and the
project HEAD must already be an ancestor of the workspace HEAD; Bach fails the target instead of resetting or
cleaning a dirty or stale workspace.

Bach writes implementation attempt artifacts under `.bach/runs/<run-id>/<agent-target>/attempt-N/`:

- `agent-prompt.md`: provider prompt combining the optional prompt file, required plan, workspace path, context path, report path, and commit instructions.
- `agent-context.json`: structured metadata including target, mode, attempt, provider, prompt, plan, workspace, branch, report path, and context path.
- `agent-report.json`: provider completion report path.
- `provider-events.raw.jsonl`: raw OpenCode JSONL events when `type = "opencode"`.
- `provider-session.json`: OpenCode session evidence including session ID, workspace path, dirty status, raw event path, and executed argv when `type = "opencode"` and a session ID was observed.
- `provider-summary.json`: normalized OpenCode provider telemetry when `type = "opencode"`. `finish_reason`, `tokens`, and `cost` are present only when OpenCode emits them; `tokens` preserves provider-shaped token payloads.

Provider session and summary artifacts use the machine-readable contracts in `docs/schemas/provider-session.schema.json` and `docs/schemas/provider-summary.schema.json`.

Merge artifacts are written directly under `.bach/runs/<run-id>/<merge-agent-target>/` as
`merge-prompt.md`, `merge-context.json`, and `merge-report.json`.

Implementation and reviewer provider commands run in the managed workspace. Merge provider commands run in the main project checkout. Agent providers should report the provider base command, such as `["opencode", "run"]`; Bach-managed OpenCode flags such as `--format json`, `--session`, and the generated prompt path are captured in provider session evidence instead.

Bach also exposes convenience environment variables:

- `BACH_AGENT_PROMPT_PATH`
- `BACH_AGENT_CONTEXT_PATH`
- `BACH_AGENT_REPORT_PATH`
- `BACH_AGENT_WORKSPACE`
- `BACH_AGENT_TARGET`
- `BACH_AGENT_ATTEMPT`
- `BACH_AGENT_MAX_ATTEMPTS`
- `BACH_AGENT_FEEDBACK_BUNDLE`
- `BACH_AGENT_ATTEMPT_DIRECTORY`
- `BACH_AGENT_MODE`
- `BACH_AGENT_ROLE`
- `BACH_PROJECT_ROOT`

The prompt file remains the primary invocation contract; environment variables are convenience helpers.

Reviewer policies:

- `policy` blocks declare `reviewers`, a list of `agent.<name>` references whose targets must use `mode = "review"`.
- `quality_gate` blocks in a policy are enforced by the generated policy target for implementer agents that reference the policy.
- After implementation evidence passes, Bach invokes a generated policy target named `policy/<name>@agent.<subject>` so policy work has its own target run, log, quality report, and applied-policy artifact.
- Attached policy `required_targets` run in the subject workspace after implementation evidence passes and before reviewer fan-out.
- Required-target failures are converted into policy findings, and Bach validates the subject workspace commit, branch, and cleanliness before reviewers run.
- After an implementer creates valid git evidence, reviewers run in parallel inside the managed workspace.
- Reviewers receive `BACH_AGENT_MODE=review`, `BACH_AGENT_ROLE`, `BACH_AGENT_REPORT_PATH`, and `BACH_AGENT_SUBJECT_*` environment variables.
- Reviewer providers receive a generated reviewer prompt that combines the optional reviewer prompt file,
  subject metadata, and the required reviewer quality-evidence JSON schema.
- Bach aggregates reviewer findings into the generated policy target's `policy-report.json` using the `agent-report-json` quality format.
- Policy reports provide `findings.open.count` and `findings.error.open.count` metrics for quality gates.

Default reviewer prompt files are provided under `prompts/agents/architecture-review.md`,
`prompts/agents/docs-sweeper.md`, and `prompts/agents/security-review.md`. Docs-sweeper and security
reviewers should emit blocking findings when user-visible/agent-visible docs or security expectations are
not met.

Improvement loops:

- `improve { max_attempts = N, until = "policy.passed" }` starts another provider attempt after failed policy evidence.
- Failed attempts write `feedback-bundle.json` under the attempt directory with verdict, findings, failed gates, reviewer summaries, and evidence paths.
- OpenCode improvement attempts resume the previous captured session by default. Before resuming, Bach verifies the previous session evidence, target, workspace path, feedback bundle, and current workspace branch. Bach records whether the workspace is dirty before invoking OpenCode and never cleans or resets dirty workspace contents.
- `attempt-history.json` records each attempt, provider session ID, and provider artifact paths while the final policy verdict is based on the latest attempt.
- `retry` remains separate from `improve`: retry repeats the same failed target execution, while improve starts a new policy-informed agent attempt.

Merge agents:

- `mode = "merge"` targets default to `lock = "merge-lane"` unless a lock is set explicitly.
- Bach refuses to invoke the merge provider until the subject's latest matching applied policy artifact under `.bach/artifacts/policies/<run-id>/<sanitized-subject>.json` has a passing verdict, whose `subject_workspace` matches the merge subject workspace, whose `subject_commit` matches the subject workspace HEAD, and whose `policy_target` names the generated policy target that succeeded in the recorded run. For example, `agent/checkout_refactor` is written as `agent-checkout_refactor.json`.
- Before provider invocation, Bach verifies the subject workspace is on the configured subject branch and is clean.
- Merge providers run in the main project checkout, receive `BACH_AGENT_SUBJECT_TARGET`, `BACH_AGENT_SUBJECT_BRANCH`, `BACH_AGENT_SUBJECT_COMMIT`, `BACH_AGENT_SUBJECT_WORKSPACE`, and `BACH_AGENT_POLICY_EVIDENCE`, and get the generated merge prompt as the final argv.
- The generated merge context includes subject branch, commit, workspace, plan, provider metadata, and policy evidence.
- A successful merge report must include `pr_url`, `target_branch_commit`, or `merge_commit` evidence. PR URLs must be valid absolute URLs and must be paired with `target_branch_commit` or `merge_commit`; commit evidence must name a commit reachable from the main checkout and must have the reviewed subject commit as an ancestor.
- Merge providers receive a generated merge prompt that combines the optional merge prompt file, subject
  metadata, policy evidence, and the required merge completion report JSON schema.
- `bach runs inspect --json <run-id>` exports all target executions in `targets`, including log paths, artifact paths, quality report summaries, agent reports, applied policy summaries, provider metadata, subject metadata, and merge evidence for control-plane ingestion.

Providers must write an Agent Report JSON file to `BACH_AGENT_REPORT_PATH`. The minimal v1 envelope is:

```json
{
  "target": "agent/checkout_refactor",
  "provider_name": "opencode",
  "provider_type": "opencode",
  "provider_command": ["opencode", "run"],
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "/absolute/project/.bach/agents/checkout_refactor",
  "branch": "bach/agents/checkout_refactor",
  "commit": "abc123",
  "changed_files": ["src/checkout.go"],
  "summary": "Refactored checkout validation."
}
```

Valid statuses are `passed`, `failed`, `blocked`, and `partial`; only `passed` succeeds the Agent Target.
Bach validates the report target, provider evidence, mode, attempt, workspace, branch, commit, and summary.
Missing or malformed reports fail the Agent Target before later policy or reviewer phases. When
`git.commit = "required"`, the provider must create a new descendant commit in the configured branch during
the attempt or the target fails. The workspace must also be clean after provider execution.

Merge providers write a merge Agent Report JSON file to `BACH_AGENT_REPORT_PATH`:

```json
{
  "target": "agent/merge_checkout_refactor",
  "provider_name": "opencode",
  "provider_type": "agent",
  "provider_command": ["opencode", "run"],
  "mode": "merge",
  "status": "passed",
  "subject": {
    "target": "agent/checkout_refactor",
    "workspace": "/absolute/project/.bach/agents/checkout_refactor",
    "commit": "abc123"
  },
  "pr_url": "https://github.com/example/repo/pull/123",
  "summary": "Opened merge PR for checkout refactor."
}
```

## Factory Work Items

Factories declare durable queues for plan-first work. A Factory is not a Target and is hidden from
`bach list`.

```hcl
factory "delivery" {
  workflow "ship" {
    plan {
      agent_template = agent_template.planner
      path = "plans/factory/${work_item.id}.md"
    }

    implement {
      agent_template = agent_template.implementer
    }

    merge {
      target = "pipeline.merge_ready"
    }

    deploy "staging" {
      target = "shell.deploy_staging"
    }

    verify "staging" {
      target = "shell.verify_staging"
    }

    deploy "production" {
      target            = "pipeline.deploy_production"
      requires_approval = true
    }

    verify "production" {
      target = "group.production_gate"
    }
  }

  triggers {
    manual {}

    provider "github_issues" {
      command       = ["bach-trigger-fixture"]
      poll_interval = "5m"
      config = {
        items_path = ".bach/fixtures/trigger-items.json"
      }

      route {
        label    = "factory:ship"
        workflow = "ship"
      }
    }
  }
}
```

Factory fields:

- `factory "<name>"`: declares a queue namespace.
- `workflow "<name>"`: declares a route for submitted Work Items. A single workflow is selected by default; multiple workflows require `--workflow` on submit.
- `plan { agent_template = agent_template.<name>, path = "...", requires_approval = true }`: runs a planning Agent Target and copies exactly one Plan file from the agent workspace to the configured project-relative path. `${work_item.id}`, `${factory.name}`, and `${workflow.name}` are supported in `path`. `requires_approval` defaults to `true`; set it to `false` to allow unattended planning.
- `implement { agent_template = agent_template.<name> }`: runs `bach plan implement` internally with this implementation template override. The daemon does not mutate the planner-authored Plan file.
- `merge { target = <target> }`: runs one normal Bach target after implementation succeeds.
- `deploy "<name>" { target = <target>, requires_approval = true }` and `verify "<name>" { target = <target> }`: run named deployment and verification targets in declaration order. Deploy approvals default to `false`; set `requires_approval = true` to pause before the deploy target runs.
- `triggers { manual {} }`: enables `bach factory submit` for that Factory.
- `triggers { provider "<name>" { ... } }`: enables an external trigger provider process that the Factory daemon polls for new Work Items.

Provider trigger fields:

- `command`: required argv array for the provider process. The first element may be `bach` to resolve to the current Bach executable.
- `poll_interval`: optional duration string; defaults to `5m` and is clamped to at least `1s`.
- `config`: optional map of string keys to string values passed to the provider during handshake and poll.
- `route { label = "...", workflow = "..." }`: optional routing rule. Items with the matching label are routed to the named workflow. When a Factory has multiple workflows, at least one route is required; with a single workflow, omitted routes default to that workflow.

Validation rules:

- Factory and workflow names must start with an ASCII letter, digit, or `_`, and may then contain ASCII letters, digits, `_`, `.`, or `-`.
- Factory names must be unique within a Bachfile.
- A Factory must declare at least one workflow.
- Workflow names must be unique within a Factory.
- Daemon-executable workflows require exactly one `plan` block and exactly one `implement` block.
- `merge`, named `deploy`, and named `verify` phases are optional. Each phase block accepts singular `target`.
- `requires_approval` is accepted only on `plan` and `deploy` blocks. `implement`, `merge`, and `verify` reject the field.
- A Factory may declare at most one `triggers` block, at most one `manual` trigger block, and any number of named `provider` trigger blocks.
- Provider trigger names must be unique within a Factory and must be simple identifiers.
- Provider triggers require a non-empty `command` array.
- Provider triggers require at least one `route` block when the Factory declares multiple workflows.
- Unknown Factory fields are rejected by Bachfile validation.

Submit a Work Item:

```sh
bach factory submit delivery \
  --title "Ship billing webhook" \
  --body "Implement the webhook and tests." \
  --label billing \
  --dedupe-key billing-webhook
```

Submission creates a UUIDv7 Work Item with lifecycle `pending`, `current_phase = "plan"`, one pending
attempt, and an immutable intake snapshot at `.bach/artifacts/factory/<work-item-id>/intake.json`.
The Work Item is persisted through the configured Backend Provider using the `factory_queue` capability.
The submitted `--plan` value is stored as an opaque reference and is not parsed or used for file I/O in
this slice.

If `--dedupe-key` matches an existing pending item for the same factory and workflow, submit returns that
existing item. JSON output reports `"created": false` for this case.

Inspect and manage the queue:

```sh
bach factory list delivery
bach factory inspect delivery <work-item-id>
bach factory cancel delivery <work-item-id> --reason "no longer needed"
bach factory approve delivery <work-item-id> --phase plan
bach factory approve delivery <work-item-id> --phase deploy.production --reason "change approved"
bach factory status delivery
```

`bach factory approve` records durable approval evidence for a Work Item that is currently waiting at the
specified phase. The command accepts `--phase <phase>` and an optional `--reason <text>`. It returns the
existing approval idempotently when the same Work Item, attempt, and phase were already approved. Approval
phase strings use dot form such as `deploy.production`. The Backend Provider DTO schema is
`docs/schemas/backend-factory-approval-v1.schema.json`.

Start a long-running Factory daemon:

```sh
bach factory start delivery --yes
bach factory start delivery --yes --poll-interval 10s --renew-interval 1m --lease-ttl 2m
```

The daemon acquires a Backend lease, polls for pending Work Items, claims one item at a time, and executes
the workflow spine `plan -> implement -> merge -> deploy[*] -> verify[*]`. Empty queues do not stop the
daemon; use SIGINT or SIGTERM to stop it and release the lease. `--poll-interval` controls how often the
daemon checks the queue and defaults to `5s`. `--renew-interval` controls how often the daemon renews its
lease and defaults to `10s`. `--lease-ttl` controls how long a lease remains valid without renewal and
defaults to `30s`.

When a `plan` or `deploy` phase requires approval and no matching approval exists, the daemon sets the Work
Item lifecycle to `waiting_approval`, keeps `current_phase` at the gated phase, clears active daemon
ownership, and continues polling other eligible work. After an operator records approval with
`bach factory approve`, the daemon resumes the Work Item on a later poll or after a restart. Plan approval
evidence stores the Plan path and hash; if the Plan file changes after approval but before implementation
resumes, the Work Item fails with a stale-approval message instead of silently implementing different text.

When a Factory declares provider triggers, `bach factory start` also starts a long-running JSON-RPC session with each provider process. The daemon polls each provider on its configured interval, routes returned items to workflows using labels, and enqueues or updates pending Work Items. If any item in a polled batch fails intake validation, the entire batch is nacked so the provider can redeliver; successfully processed batches are acked and the trigger cursor is advanced. Provider trigger protocol messages conform to `docs/schemas/trigger-provider-v1.schema.json`. Provider intake failures do not fail Work Items that are already queued or active.

Use `--json` with any Factory command for machine-readable output. `factory submit` returns
`{"item": <work-item>, "created": true|false}`. `factory list` returns `{"items": [<work-item>, ...]}`.
`factory inspect` and `factory cancel` return a single Work Item object. `factory inspect` includes an
`approvals` array. `factory status` returns the active daemon lease, optional active Work Item ID, and
lifecycle counts. `factory start --json` returns the daemon ID and acquired lease record after the command
stops. CLI Work Item JSON omits raw body text; use the intake evidence URI for private submission details.
Failed Work Items include `failure_phase` and `failure_message`. The Backend Provider Work Item DTO schema is
`docs/schemas/backend-factory-work-item-v1.schema.json`.

`factory list` defaults to `pending` and `waiting_approval` items; pass `--status all` to include every
lifecycle. `--status` accepts `pending`, `claimed`, `running`, `waiting_approval`, `completed`, `failed`,
`cancelled`, or `all`. Pass `--workflow` to filter list output by workflow.

Current lifecycle values are:

- `pending`: queued for future planning/execution phases.
- `claimed`: claimed by a daemon lease before phase execution starts.
- `running`: executing the current workflow phase.
- `waiting_approval`: paused at a gated phase until an approval is recorded.
- `completed`: all configured workflow phases succeeded.
- `failed`: a workflow phase failed; the Work Item records the failed phase and message.
- `cancelled`: manually cancelled before execution.

Deferred Factory behavior includes retries, review queues, and replan loops.

## Plans

`bach plan status <plan-file> [plan-file ...]` loads Markdown Plans, validates the selected Plan graph, reads Backend Plan ledgers, and prints deterministic status and planned wave output. It does not execute agents, generate targets, mutate Factory Work Items, or write Plan evidence.

`bach plan implement <plan-file> [plan-file ...]` executes one or more Markdown Plans. When a single Plan file is supplied it behaves like the focused single-Plan executor. When multiple Plan files are supplied it runs them as a batch: it validates the selected Plan graph, computes ready waves, executes ready Plans with bounded parallelism, applies the configured stop mode, and writes lifecycle Plan ledger records to the Backend for every executed Plan. Generated targets are not written to a Bachfile and are not shown by `bach list`.

`bach plan review <plan-file> [plan-file ...]` groups the selected Plans by decision state without executing anything. It derives the review queue from Plan ledgers, status, and diagnostics.

Plans do not require frontmatter. Bach infers the Plan ID from the project-relative file path and the title from the first Markdown heading:

```md
# Checkout refactor
```

Optional YAML frontmatter supplies stable overrides and execution hints:

```md
---
id: phase-14-checkout
title: Checkout refactor
depends_on: [phase-13-runtime-model]
agent_template: feature_implementer
policy: standard_feature
required_targets: [shell/test]
labels: [factory]
metadata:
  owner: platform
---

# Checkout refactor
```

Supported frontmatter fields are `schema`, `id`, `title`, `description`, `depends_on`, `agent_template`, `policy`, `required_targets`, `labels`, and `metadata`. `schema` is optional; when present it must be `bach.plan.v1`. Unknown fields fail validation. `workstreams` is rejected in v1 because one Plan is one future implementer unit.

Statuses are:

- `ready`: no ledger exists and the Plan has no dependencies, or all selected dependencies are implemented.
- `planned`: no ledger exists and at least one selected dependency is not implemented yet.
- `blocked`: no ledger exists and at least one selected dependency is stale, invalid, or blocked.
- `pending`: latest Backend ledger says execution has been queued but has not started.
- `in_progress`: latest Backend ledger says execution is currently active.
- `implemented`: latest Backend ledger is implemented and matches the current Plan hash.
- `failed`: latest Backend ledger says execution failed.
- `stale`: latest Backend ledger is implemented but the Plan hash changed.
- `invalid_ledger`: latest Backend ledger fails validation.

Use `--json` for machine-readable output with `schema_version`, `plans`, `waves`, and `diagnostics`.

`bach plan implement` writes `pending`, then `in_progress`, then either `implemented` or `failed` ledger records for the current Plan hash. Generated Plan implementer targets are remote/destructive agent targets and require `--yes` to execute. If the latest Backend ledger is already `implemented` for the same Plan hash, the command skips execution and reports `skipped`; use `--force` to run the generated target anyway. Plan implementation validates `required_targets` references but does not run them as a separate Plan phase.

`bach plan status` requires every `depends_on` Plan to be included in the selected Plan set so it can render waves. `bach plan implement <plan-file>` executes exactly one Plan and treats `depends_on` as external preconditions: every dependency must have a latest Backend ledger with status `implemented` before execution starts.

Batch execution supports:

- `--parallelism <n>`: maximum Plans to execute concurrently within a ready wave. Default `1`.
- `--stop-on <mode>`: `failure` stops starting new Plans after any Plan fails; `never` continues independent ready Plans. Default `failure`.
- `--dry-run`, `--force`, `--yes`, `--env-file`, `--log-only`, `--verbose`, and `--jobs` are passed through to each generated target run.

A Plan is ready for a batch wave when all selected dependencies were implemented in earlier waves and all external dependencies already have an `implemented` Backend ledger. A Plan is `blocked` when a dependency failed, is blocked, or has no implemented ledger. A Plan is `skipped` when the batch stopped before it could start and it was not already blocked.

Human batch output summarizes selected Plan count, wave count, per-state counts, and a per-Plan table with state, run, target, and reason. JSON batch output uses `schema_version: "bach.plan_batch.v1"` with `plans`, `waves`, `started_at`, and `ended_at`.

`bach plan review` groups Plans into:

- `implemented`: completed cleanly.
- `needs_review`: implemented but has diagnostics that may need human inspection.
- `failed`: execution failed.
- `blocked`: dependency or precondition blocked execution.
- `skipped`: skipped due to the batch stop mode or already fresh evidence.

JSON review output uses `schema_version: "bach.plan_review.v1"` with `implemented`, `needs_review`, `failed`, `blocked`, `skipped`, and optional `diagnostics`.

Human output summarizes the Plan, generated target, template, run ID, result, and written ledgers. JSON output for single-Plan execution uses `schema_version: "bach.plan_implement.v1"` with `plan`, `result`, `target`, optional `template`, optional `run_id`, latest `ledger`, `written_ledgers`, and optional `diagnostics`.

