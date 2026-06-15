# Agent Instructions

## Purpose

`internal/planbatch/` owns multi-Plan batch execution and review-queue orchestration. It schedules ready waves of selected Plans, executes each Plan through `internal/planexecute`, applies stop modes, and assembles review-queue summaries.

## Ownership

- Batch planning: selected Plan graph validation, wave calculation, external dependency precondition checks, and per-Plan readiness classification.
- Batch execution: bounded parallelism across ready Plans, stop-mode application, and per-Plan result collection.
- Review queue: grouping Plans by decision state from ledgers, run evidence, and batch results.

## Local Contracts

- Keep Markdown parsing and pure graph logic in `internal/plan` and `internal/planstatus`.
- Keep single-Plan execution semantics in `internal/planexecute`.
- Keep CLI formatting in `internal/cli`.
- Keep production dependency construction in `internal/app`.
- Do not mutate Factory Work Items, attempts, approvals, or daemon state.
- Generated Agent Targets remain in-memory runtime targets; do not write Bachfile blocks.

## Verification

- Use `go run ./cmd/bach run shell/test` after Plan batch changes.
- Use `go run ./cmd/bach run shell/e2e` when CLI-visible behavior changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/planbatch/`.
