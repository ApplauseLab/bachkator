# Agent Instructions

## Purpose

`internal/state/migrations/` contains SQL migrations for the private local State Store schema.

## Ownership

- Ordered State Store schema changes.
- SQL files applied by the state package migration mechanism.

## Local Contracts

- Add new numbered migrations for schema changes.
- Do not edit historical migrations unless they are known to be unshipped and the user explicitly approves.
- Keep migrations deterministic and local-only; they must not depend on external tools or network state.

## Work Guidance

- Pair migration changes with Go persistence types, load/save behavior, and tests.
- Preserve migration numbering and naming conventions.

## Verification

- Use `go run ./cmd/bach run shell/test` after migration changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/state/migrations/`.
