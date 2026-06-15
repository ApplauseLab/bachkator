# Agent Instructions

## Purpose

`internal/query/` owns read-only query DTO assembly over private persistence and domain records for CLI adapters and future control-plane surfaces.

## Ownership

- Run, log, quality, artifact, and evidence query records assembled from State Store and domain packages.
- Read-only aggregation logic that should not live in CLI formatting code.

## Local Contracts

- Keep this package read-only; execution and writes belong to runner, target handlers, quality ingestion, or State Store.
- Do not import `internal/cli`; return presentation-neutral DTOs.
- Preserve deterministic ordering for CLI output and tests.

## Work Guidance

- Keep query DTOs close to CLI/public evidence needs without exposing private SQL table shapes.
- Add focused tests when aggregation behavior changes.

## Verification

- Use `go run ./cmd/bach run shell/test` after query changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/query/`.
