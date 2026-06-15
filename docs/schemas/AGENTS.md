# Agent Instructions

## Purpose

`docs/schemas/` owns JSON schemas for machine-readable contracts exposed by Bachkator.

## Ownership

- Quality plugin report schemas and public CLI result schemas.
- Schema files referenced by docs, plugins, tests, or external integrations.

## Local Contracts

- Treat schema changes as public contract changes.
- Keep schemas aligned with parser behavior, docs, and examples.
- Do not remove or rename fields without considering existing plugin or control-plane consumers.

## Work Guidance

- Update reference docs and tests in the same change as schema changes.
- Prefer additive schema evolution unless a breaking change is explicitly intended.

## Verification

- Use `go run ./cmd/bach run shell/test` after schema changes.
- Use `go run ./cmd/bach run shell/e2e` when schema changes affect plugin or quality workflows.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `docs/schemas/`.
