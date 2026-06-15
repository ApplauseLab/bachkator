# Agent Instructions

## Purpose

`internal/state/` owns Bachkator's private local State Store for fingerprints, runs, target records, artifacts, quality reports, quality gates, policy evidence, normalized findings, Factory queue records, and Plan ledger records.

## Ownership

- SQLite store access and persistence types.
- Migration application and schema evolution.
- Persistence of run, artifact, fingerprint, quality-owned report records, policy records, normalized finding records, Factory Work Item queue records, and Plan ledger/evidence records.

## Local Contracts

- `Store` owns the `*sql.DB` connection. Construct it with `NewStore(path)` (read-write) or `OpenReadOnlyStore(path)` (read-only), defer `Close()`, and call methods.
- `OpenReadOnlyStore` returns a `Store` with `db == nil` when the DB file does not exist; its read-style methods return empty results without error.
- The State Store schema is private; expose durable evidence through CLI output, logs, artifacts, quality reports, and JSON exports.
- `internal/quality` owns normalized quality record semantics; State Store persists those shapes without redefining the quality domain.
- Persist run start before writing run-scoped quality evidence; SQLite foreign keys are enforced for State Store connections.
- Never require users or managed control planes to read private tables directly.
- Migrations must be append-only and deterministic once shipped.
- Preserve existing persisted data unless an explicit migration handles it.

## Work Guidance

- Add a new migration for schema changes; do not edit old migrations unless they have not shipped and the user explicitly approves.
- Keep database operations scoped and error messages actionable.
- Update state tests and any JSON export tests when persisted evidence changes.

## Verification

- Use `go run ./cmd/bach run shell/test` after state changes.
- Use `go run ./cmd/bach run shell/e2e` when persisted run/quality evidence affects CLI behavior.

## Child DOX Index

- `internal/state/migrations/AGENTS.md`: State Store schema migrations.
