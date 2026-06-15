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
