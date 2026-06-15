# Agent Instructions

## Purpose

`test/` owns end-to-end tests and test fixtures for Bachkator CLI behavior.

## Ownership

- `e2e/`: Bats-based CLI end-to-end tests.
- `fixtures/`: sample projects and fixture files consumed by e2e tests.

## Local Contracts

- Keep fixtures small, explicit, and isolated from the developer's real environment.
- Do not rely on global machine state beyond tools declared by the invoking Bach target.
- E2E tests should exercise user-visible CLI behavior and evidence artifacts, not private implementation details.

## Work Guidance

- Add or update e2e coverage when public CLI behavior, generated files, run logs, quality evidence, or Bachfile syntax behavior changes.
- Keep test names descriptive enough that a failing Bats line explains the user-visible behavior.

## Verification

- Use `go run ./cmd/bach run shell/e2e` for e2e or fixture changes.
- Use `go run ./cmd/bach run --log-only --force group/gate` before release-facing changes.

## Child DOX Index

- `test/e2e/AGENTS.md`: Bats-based CLI end-to-end tests.
- `test/fixtures/AGENTS.md`: e2e fixture projects and files.
