# Agent Instructions

## Purpose

`internal/factorydaemon/` owns Factory daemon orchestration: Backend daemon leases, queue polling, Work Item claiming, workflow phase execution, phase status writes, daemon status assembly, and provider trigger polling.

## Local Contracts

- Keep CLI formatting in `internal/cli`.
- Keep production dependency construction in `internal/app`.
- Use Backend client methods for Factory state; do not query private State Store tables directly.
- Run executable work through existing Agent Target, Plan execution, and runner paths instead of creating a parallel execution universe.
- Trigger provider polling runs inside `bach factory start`; there is no standalone trigger poll command.
- Trigger provider subprocesses receive only `PATH`, temp directory variables, and the env variable named by `config.token_env` when present.
- Provider trigger failures are logged and nacked; they do not fail queued or active Work Items.
- Release the daemon lease on shutdown using a fresh timeout context so SIGINT/SIGTERM teardown is not blocked by the canceled signal context.
- Expose tunable queue poll, lease renewal, and lease TTL intervals through the CLI adapter; defaults are 5s poll, 10s renew, 30s TTL.

## Verification

- Use `go run ./cmd/bach run shell/test` after daemon orchestration changes.
- Use `go run ./cmd/bach run shell/e2e` when CLI-visible Factory behavior changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/factorydaemon/`.
