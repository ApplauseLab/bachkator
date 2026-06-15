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
