# Agent Instructions

## Purpose

`internal/plan/` owns side-effect-free Markdown Plan parsing, metadata inference, validation, hashing, graph construction, wave calculation, and status derivation from supplied ledger records.

## Local Contracts

- Do not import CLI, app, backend, config, runner, state, or filesystem-writing packages.
- Keep Plan documents separate from `internal/runner.Plan`, which remains target execution planning.
- Treat frontmatter as optional overrides; infer Plan ID from project-relative path and title from the first Markdown heading when absent.
- Reject `workstreams` metadata in v1 so one Plan remains one implementation unit.
- Keep Backend reads and provider lifecycle outside this package.

## Verification

- Use `go run ./cmd/bach run shell/test` after Plan package changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/plan/`.
