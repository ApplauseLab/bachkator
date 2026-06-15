# Agent Instructions

## Purpose

`internal/factory/` owns Factory Work Item service behavior for durable manual queues.

## Ownership

- Factory Work Item submit, list, inspect, cancel, and approval service operations.
- Manual queue validation, default lifecycle values, and lifecycle constants.
- Intake evidence snapshot creation under Bach-owned private artifacts.
- Queue-facing DTOs used by app and CLI adapters.
- Approval logic lives in `service_approvals.go`; core service helpers stay in `service.go`.

## Local Contracts

- Keep Factories separate from Targets; do not add runner execution or target graph behavior here.
- Do not import Cobra or CLI presentation packages.
- Use `internal/evidence` for Bach-owned artifact writes and project file resolution.
- Treat submitted bodies and intake snapshots as private local evidence; do not expose raw body text by default in CLI views.
- Delegate persistence through an explicit queue dependency instead of opening the State Store directly.

## Work Guidance

- Keep lifecycle additions explicit and covered by service tests.
- Validate user-provided submit and cancel fields before queue writes.
- Update reference docs and e2e tests when Factory CLI behavior changes.

## Verification

- Use `go run ./cmd/bach run shell/test` after service changes.
- Use `go run ./cmd/bach run shell/e2e` when behavior is visible through `bach factory`.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/factory/`.
