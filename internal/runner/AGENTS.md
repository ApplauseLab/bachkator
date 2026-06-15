# Agent Instructions

## Purpose

`internal/runner/` owns run planning, scheduling, execution, target runtime behavior, cache/fingerprint checks, logs, artifacts, retries, locks, dry-run output, and agent target orchestration.

## Ownership

- Run Plan construction and dry-run evidence.
- Run Session mutable execution coordination.
- Target scheduling, dependency execution, pipeline ordering, and log streaming.
- Agent implementation, review, merge, policy, and improvement-loop execution semantics.

## Local Contracts

- A Run Plan describes what will run; it must not execute commands, probe tools, write logs, or mutate state.
- A Run Session coordinates mutable execution state; persisted results belong to the State Store.
- Preserve pipeline ordering and dependency graph parallelism semantics.
- Preserve quiet/log-only behavior: full command output must still be written to logs.
- Keep CLI formatting out of runner internals except structured summaries explicitly returned to CLI adapters.
- Agent providers supply execution mechanics; Bach owns workspace, git, policy, report, and evidence semantics.

## Work Guidance

- Read ADRs for DAG execution, target handlers, timeouts/retries, agent targets, and managed control-plane boundaries before changing orchestration behavior.
- Add tests for scheduling, planning, cache, logs, policy, or agent behavior changes.
- Be careful around recent conflict hotspots: `runner.go`, `executor.go`, `scheduler.go`, and `logs.go`.

## Verification

- Use `go run ./cmd/bach run shell/test` after runner changes.
- Use `go run ./cmd/bach run shell/e2e` when CLI-visible run behavior changes.
- Use `go run ./cmd/bach run --log-only --force group/gate` for broad runner changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/runner/`.
