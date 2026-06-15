# Plan-First Agent Workflows

Status: in progress

See `docs/designs/dark-software-factory-master-plan.md` for the broader Factory, Backend, trigger-provider, and Atelier integration roadmap that builds on this plan-first foundation.

Accepted direction: `docs/adr/0020-plan-execution-unit-and-backend-evidence.md` locks one accepted Plan as one implementer unit and stores Bach-owned Plan ledger/evidence records in the Backend database.

## Current Intent

Bach should support repeated agent implementation workflows where a feature Plan is durable, validated, implemented by one agent, verified by Bach, and safe to rerun. If the Plan has not changed and the recorded Backend evidence is still valid, rerunning should do nothing. If the Plan changed, Bach should identify that Plan as stale and require retry or replan.

The v1 unit is deliberately simple:

```text
one Plan file -> one accepted Plan -> one implementer Agent Target
```

Do not split one Plan into workstream agents in v1. If work is too large for one implementer, split it into multiple Plan files and express dependencies between those Plans.

## Core Model

```text
Plan markdown = human intent
Optional Plan frontmatter = machine-readable overrides and execution hints
Plan ledger = Backend-owned implementation/evidence status
Agent Template = reusable execution machinery
Generated target = one temporary concrete implementer target per Plan
Run/evidence records = proof stored through the Project Backend
Plan batch = selected Plans plus cross-Plan DAG for one execution window
Review queue = morning handoff across executed Plans
```

## Factory Roadmap

Plan-first workflows are the foundation for the dark software factory. The factory should not be built first, because triggers need a stable execution contract to target.

```text
1. Plan-first agent workflows
   - infer Plan defaults and parse optional frontmatter
   - expose reusable Agent Templates
   - store Plan ledger/evidence records in the Backend database
   - materialize one temporary implementer target per Plan
   - run batch dry-runs, evidence writes, and review queues

2. First-class Factory syntax
   - add a `factory` Bachfile block
   - attach triggers to Plan-first execution
   - start with manual intake before external triggers

3. Atelier Backend and evidence integration
   - let a Factory use a supported Backend Provider
   - export durable evidence, ledgers, and review queues through supported Backend contracts
   - keep managed products away from private SQLite tables
```

The first Factory trigger slice can be intentionally small: accept manually submitted work items, normalize them into a Work Item and one Plan, then hand off to Plan execution. Browser automation, GitHub Issues, Discord, and Atelier intake can follow once the queue, evidence, and credential rules are proven.

## Batch Graph

Large work happens across many Plan files, not by manually invoking one agent target per feature. Bach should load multiple Plans, combine them into one Plan-level DAG, and execute the ready frontier with bounded parallelism.

```sh
bach plan status plans/day-2026-06-13/*.md
bach plan implement --yes plans/phase-14-checkout.md
```

Batch `plan implement` over a ready frontier is future Phase 7 behavior; Phase 5 executes exactly one Plan per invocation.

Graph node identity is the Plan ID:

```text
phase-14-checkout
phase-15-admin
phase-16-billing-webhooks
```

Plan IDs default from project-relative file paths and may be pinned with optional frontmatter. Dependencies are selected Plan IDs:

```yaml
id: phase-16-billing-webhooks
title: Billing webhooks
depends_on:
  - phase-14-checkout
  - phase-15-admin
```

Cross-Plan dependencies are part of the reviewed Plan contract. `bach plan status` fails graph loading when a dependency is missing, duplicated, cyclic, or outside the selected execution set. Single-Plan execution may treat dependencies as external preconditions when the latest Backend ledger proves each dependency is implemented.

## Plan Metadata

Plans do not require frontmatter. Bach infers `id` from the project-relative path and `title` from the first Markdown heading. Plan dependencies and execution hints live in optional frontmatter because they are part of the reviewed implementation contract when present.

```md
---
id: phase-14-checkout
title: Checkout refactor
depends_on: [phase-13-runtime-model]
agent_template: feature_implementer
policy: standard_feature
required_targets: [shell/test]
labels: [factory]
---

# Phase 14 - Checkout refactor
```

V1 frontmatter is Plan-level only. `schema: bach.plan.v1` is optional; when present it must match. `workstreams` is not a valid v1 field.

## Plan Ledger And Evidence

Bach, not agents, records implementation status. Plan ledgers and Plan evidence are Backend records, not sidecar JSON files beside the Markdown Plan.

The default local Backend is bundled SQLite, but the contract is the Backend Provider protocol. Plan ledger records should be stored through a `plan_ledger` capability with domain methods such as:

```text
plans.recordLedger
plans.getLedger
```

Ledger writes are append-only. Bach supplies UUIDv7 ledger and evidence record IDs. Repeating the same ledger ID with identical payload is idempotent; repeating it with different payload is a conflict. `plans.getLedger` returns the latest ledger for a Plan ID or `not_found` when no ledger exists.

Example ledger DTO:

```json
{
  "schema_version": "bach.plan_ledger.v1",
  "ledger_id": "019ec...",
  "plan_id": "phase-14-checkout",
  "status": "implemented",
  "hash": "sha256:...",
  "run_id": "019ec...",
  "commit": "abc123",
  "recorded_at": "2026-06-13T10:15:00Z",
  "evidence": [
    {
      "id": "019ec...",
      "kind": "plan.implemented",
      "hash": "sha256:...",
      "content": {
        "summary": "Implemented checkout refactor"
      },
      "metadata": {
        "generated_target": "agent/plan.phase-14-checkout",
        "plan_path": "plans/phase-14-checkout.md",
        "run_id": "019ec..."
      }
    }
  ],
  "implemented_at": "2026-06-13T10:15:00Z"
}
```

Plan ID is the inferred or frontmatter-pinned slug for dependencies and CLI display. Backend ledger/evidence record IDs are Bach-generated UUIDv7 IDs.

## Write Ownership

```text
Planning agent or human writes:
  Plan Markdown files
  Optional Plan frontmatter dependencies

Implementer agent writes:
  source files
  docs
  tests
  Agent Target completion report

Bach writes:
  Backend Plan ledger records
  Backend Plan evidence records
  Run records and quality/finding records
```

Agents should not mark Plans implemented. Bach records implementation only after commit, workspace, required targets, reviewers, quality gates, policy verdict, and evidence are valid.

## Agent Template Relationship

Committed HCL should define reusable machinery, not one-off Plan targets.

```hcl
provider "opencode" {
  type = "opencode"
}

prompt "implementer" {
  path = "prompts/agents/implementer.md"
}

agent_template "feature_implementer" {
  provider = provider.opencode
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    mode = "clone"
    path = ".bach/agents/${plan.id}"
  }

  git {
    branch = "bach/${plan.id}"
    commit = "required"
  }
}
```

Bach materializes one temporary concrete implementer Target from the Plan plus template. Generated targets are execution artifacts, not source of truth.

## CLI Shape

```sh
bach plan status plans/phase-14-checkout.md
bach plan status plans/day-2026-06-13/*.md --json
bach plan implement --yes plans/phase-14-checkout.md
```

`bach plan status` is the first slice. It parses Plans, builds a selected Plan DAG, computes Plan hashes, reads Backend ledgers, and reports `ready`, `planned`, `blocked`, `implemented`, `stale`, or `invalid_ledger`.

`bach plan implement` materializes one implementer Agent Target for one selected Plan and writes Plan ledger/evidence records to the Backend after validation passes. Batch selection, `--ready`, and `--stop-on` are future Phase 7 behavior.

## Execution Loop

```text
load selected Plans
build combined Plan DAG
load Backend ledgers and evidence
skip already-valid Plans
while ready Plans exist:
  start up to -j Plan implementer agents
  for each completed Plan:
    verify implementation commit
    run required targets
    run reviewers/policy gates
    write Backend Plan ledger/evidence records
  unlock downstream Plans whose dependencies now pass
stop when all possible work completed or a stop condition fires
write batch review queue through Backend evidence records
```

Stop conditions should be explicit because overnight execution has different risk tolerance from an interactive run:

- `--stop-on blocker`: continue through ordinary failures, but stop if a policy marks a blocker or dependency root fails.
- `--stop-on failure`: stop starting new work after any failed Plan.
- `--best-effort`: keep executing independent Plans even when some branches fail.

## Morning Review Queue

The output of overnight execution should be optimized for waking up and making decisions quickly. A human should not have to open dozens of run logs to understand what happened.

```sh
bach plan status plans/day-2026-06-13/*.md
bach plan status plans/day-2026-06-13/*.md --json
```

Review states:

- `ready_to_review`: implementation, required targets, reviewers, and policy passed; human should inspect the commit or PR.
- `ready_to_merge`: merge policy passed and a merge artifact or PR URL exists.
- `needs_plan_update`: implementation could not proceed because the Plan changed, dependency contract was invalid, or the agent discovered missing requirements.
- `failed_verification`: code was produced but tests, quality gates, or policy failed.
- `blocked`: dependency or required external condition prevents progress.
- `skipped_valid`: previous evidence remains valid; no new work done.

## Idempotency

Before running a Plan, Bach checks:

```text
current Plan hash == recorded Backend ledger hash
implementation commit exists
workspace is clean
policy evidence passed
Backend evidence records exist and validate
```

If all checks pass, Bach no-ops:

```text
Plan phase-14-checkout already implemented at abc123. No changes needed.
```

If the hash changed, Bach marks the Plan stale. A later implementation phase can decide whether to rerun from scratch, generate a delta prompt, or require an explicit retry/replan action.

## Risk And Selection

Before launching an overnight batch, Bach should provide an inspectable plan:

```sh
bach plan implement --dry-run plans/phase-14-checkout.md
```

The dry-run should show:

- selected Plans.
- skipped-valid Plans.
- ready wave order.
- maximum parallelism.
- required targets and reviewers per Plan.
- estimated risk tier when available.
- missing or external dependency errors.
- expected Backend ledger/evidence records.

## Open Questions

1. Which optional Plan frontmatter fields remain necessary after path and heading inference?
2. Should `bach plan implement --ready` run all dependency waves in one invocation, or only the currently ready wave?
3. How much evidence validation is required for no-op: commit existence only, latest policy verdict, or all linked Backend evidence records?
4. How should Bach cap overnight spend: max agents, max model tier, max estimated cost, max wall-clock, or all of them?
5. Should cross-Plan dependencies require selected Plans by default, or may they refer to any implemented ledger in the Backend?

## Next Phase Candidate

The first implementation phase should ship:

- Plan default inference and optional frontmatter decoding.
- Plan-level hashing.
- Backend Plan ledger/evidence storage.
- `bach plan status`.
- selected multi-Plan DAG validation.
- planned wave output.
- no agent execution yet.
