# Agent Instructions

## Purpose

`internal/backend/` owns Bachkator's internal Backend client facade, provider process lifecycle, mapping between internal domain models and public provider protocol DTOs, and built-in Backend provider implementations.

## Ownership

- Internal Backend client facade and domain subclients, including the Factory queue and Plan ledger subclients.
- Provider process startup, initialization, shutdown, capability checks, and protocol error handling.
- Mapping between internal runner/query/state/Factory queue/Plan ledger models and `pkg/backendprotocol` DTOs; Factory-specific mapping helpers live in `factory_mapping.go`.
- Built-in SQLite Backend provider under `internal/backend/sqlite`.

## Local Contracts

- Keep public provider DTOs and helpers in `pkg/backendprotocol` and `pkg/jsonrpcstdio`; do not expose internal runner, config, or storage types as provider SDK API.
- Backend Provider protocol methods are domain operations, not SQL/table/key-value operations.
- Backend subclient methods accept caller-owned `context.Context` and provider calls must derive their timeout with `context.WithTimeout(ctx, providerCallTimeout)`.
- SQLite schema and migrations belong to the SQLite provider implementation, not Bach core callers.
- Preserve the Evidence Store as the local path safety/redaction boundary before Backend writes.

## Work Guidance

- Keep provider process behavior deterministic and testable with fixture providers.
- Keep internal domain models separate from public protocol DTOs and map explicitly at this boundary.
- Add conformance tests when provider protocol behavior changes.
- Serialize SQLite schema migrations across provider processes with a filesystem lock so concurrent `backend.initialize` calls do not dirty the migration table.

## Verification

- Use `go run ./cmd/bach run shell/test` after Backend changes.
- Use `go run ./cmd/bach run shell/e2e` when Backend behavior affects CLI-visible runs, logs, or evidence.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/backend/`.
