# Agent Instructions

## Purpose

`internal/planstatus/` owns Plan status use-case orchestration: safe project-relative Plan file reads, project reference validation, Backend Plan ledger reads, and assembly of pure `internal/plan` status output.

## Local Contracts

- Keep CLI formatting in `internal/cli`, not here.
- Keep production dependency construction in `internal/app`, not here.
- Do not write Plan ledger/evidence records from the status use-case.
- Use the Backend client facade for Plan ledger reads; do not query private state tables directly.
- Return typed domain errors for invalid inputs. `ErrNoPlanPaths` signals that no plan file paths were supplied; `internal/cli` wraps it into a `UsageError` for presentation.

## Verification

- Use `go run ./cmd/bach run shell/test` after Plan status changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/planstatus/`.
