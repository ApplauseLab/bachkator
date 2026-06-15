# Agent Instructions

## Purpose

`internal/planexecute/` owns Plan implementation orchestration: single-Plan validation, generated Agent Target materialization from Agent Templates, runner invocation, lifecycle Plan ledger writes, and execution result assembly.

## Local Contracts

- Keep Markdown parsing and pure graph/status logic in `internal/plan` and `internal/planstatus`.
- Keep CLI formatting in `internal/cli`.
- Keep production dependency construction in `internal/app`.
- Do not mutate Factory Work Items, attempts, approvals, daemon leases, or phase state.
- Generated Agent Targets are in-memory runtime targets and must not be written to Bachfiles.

## Verification

- Use `go run ./cmd/bach run shell/test` after Plan execution changes.
- Use `go run ./cmd/bach run shell/e2e` when CLI-visible behavior changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/planexecute/`.
