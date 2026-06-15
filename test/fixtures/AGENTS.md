# Agent Instructions

## Purpose

`test/fixtures/` owns fixture projects and files used by Bachkator e2e tests.

## Ownership

- Minimal test repositories and files copied or referenced by e2e tests.

## Local Contracts

- Keep fixtures small, deterministic, and purpose-specific.
- Do not include real credentials, private paths, or machine-specific assumptions.
- Fixture behavior should be understandable from the consuming test.

## Work Guidance

- Update consuming e2e tests when fixture layout or expected output changes.
- Prefer adding a focused fixture over overloading an existing one with unrelated behavior.

## Verification

- Use `go run ./cmd/bach run shell/e2e` after fixture changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `test/fixtures/`.
