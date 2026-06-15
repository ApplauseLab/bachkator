# Agent Instructions

## Purpose

`test/e2e/` owns Bats tests that exercise Bachkator as a user-facing CLI.

## Ownership

- CLI read-only behavior, run/state behavior, cache/fingerprint behavior, imports, quality plugins, agent workflows, policy fanout, merge, factory approvals, factory daemon behavior, and failure diagnostics.
- Shared Bats helpers; factory helpers live in `factory_helpers.bash`.

## Local Contracts

- Test public behavior and evidence artifacts, not private Go implementation details.
- Keep tests hermetic and based on fixtures or temporary directories.
- Do not depend on a developer's global Bach state.
- Use Bats helpers for repeated setup and assertions.

## Work Guidance

- Add e2e coverage when changing CLI behavior, run artifacts, logs, policy evidence, generated prompts, quality reports, or Bachfile syntax.
- Keep failure output useful for agents reading Bach logs.

## Verification

- Use `go run ./cmd/bach run shell/e2e` after e2e changes.
- Use `go run ./cmd/bach run --log-only --force group/gate` for release-facing changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `test/e2e/`.
