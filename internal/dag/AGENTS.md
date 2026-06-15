# Agent Instructions

## Purpose

`internal/dag/` owns generic directed acyclic graph behavior used by Bachkator planning and dependency ordering.

## Ownership

- Graph data structure behavior.
- Cycle detection and topological ordering utilities.

## Local Contracts

- Keep this package domain-light and independent from CLI, config, runner, and state packages.
- Do not introduce Bachfile parsing, target execution, or persistence concerns here.
- Preserve deterministic ordering where callers rely on stable plans and tests.

## Work Guidance

- Keep APIs small and generic.
- Add tests for cycle, ordering, and edge-case behavior when changing graph logic.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/dag/`.
