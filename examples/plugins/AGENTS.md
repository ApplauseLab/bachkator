# Agent Instructions

## Purpose

`examples/plugins/` owns example graph and quality plugins plus fixtures that demonstrate supported plugin contracts.

## Ownership

- TypeScript graph plugin examples.
- Quality parser plugin example.
- Fixtures and tests that prove example plugin behavior.

## Local Contracts

- Keep plugin stdout and input behavior aligned with documented plugin contracts.
- Do not add examples for private or unsupported plugin lifecycles.
- Keep fixtures small and deterministic.

## Work Guidance

- Update `docs/reference/28-plugins.md` or related reference fragments when plugin contracts change.
- Pair example changes with tests where possible.

## Verification

- Use `go run ./cmd/bach run shell/test` after example plugin changes.
- Use `go run ./cmd/bach run shell/fmt` after Go example test edits.

## Child DOX Index

- `examples/plugins/quality-parser/AGENTS.md`: standalone quality parser plugin example.
