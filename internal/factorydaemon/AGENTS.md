# Agent Instructions

## Purpose

`internal/factorydaemon/` owns Factory daemon orchestration: Backend daemon leases, queue polling, Work Item claiming, workflow phase execution, phase status writes, daemon status assembly, provider trigger polling, and external approval provider polling.

## Local Contracts

- Keep CLI formatting in `internal/cli`.
- Keep production dependency construction in `internal/app`.
- Use Backend client methods for Factory state; do not query private State Store tables directly.
- Run executable work through existing Agent Target, Plan execution, and runner paths instead of creating a parallel execution universe.
- Trigger provider polling runs inside `bach factory start`; there is no standalone trigger poll command.
- Approval provider polling also runs inside `bach factory start`; providers submit approval evidence over `bach.approval.v1`, and Bach validates records against waiting Work Items, attempts, phases, and workflow gates before storing approvals through the same path as `bach factory approve`.
- External approval matching uses intake source identity (`source_type`/`source_id`) or a direct `work_item_id`; recordings are idempotent and redelivery safe.
- Trigger and approval provider sessions share one process/session helper; protocol clients stay in `pkg/*protocol` packages.
- Provider trigger failures are logged and nacked; they do not fail queued or active Work Items.
- Release the daemon lease on shutdown using a fresh timeout context so SIGINT/SIGTERM teardown is not blocked by the canceled signal context.
- Expose tunable queue poll, lease renewal, and lease TTL intervals through the CLI adapter; defaults are 5s poll, 10s renew, 30s TTL.

## Verification

- Use `go run ./cmd/bach run shell/test` after daemon orchestration changes.
- Use `go run ./cmd/bach run shell/e2e` when CLI-visible Factory behavior changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/factorydaemon/`.
