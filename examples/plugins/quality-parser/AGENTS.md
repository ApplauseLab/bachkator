# Agent Instructions

## Purpose

`examples/plugins/quality-parser/` demonstrates a typed quality plugin that emits normalized quality report JSON.

## Ownership

- Minimal plugin implementation files.
- README guidance for using the example.

## Local Contracts

- Keep emitted JSON aligned with `docs/schemas/quality-plugin-report.schema.json`.
- Keep README commands and Bachfile snippets aligned with current plugin syntax.

## Work Guidance

- Prefer a tiny fixture-driven example over production-grade plugin framework code.
- Update docs when changing the example's demonstrated contract.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `examples/plugins/quality-parser/`.
