# Agent Instructions

## Purpose

`internal/agentprovider/` owns private provider adapters used by Agent Targets to run external agent tools and capture provider evidence.

## Ownership

- Provider selection for built-in provider types.
- Generic command provider execution.
- First-class OpenCode command construction, JSONL capture, session evidence, summaries, and resume validation.

## Local Contracts

- Keep this package private to Bachkator; do not expose a public plugin API from it.
- Preserve Agent Target evidence semantics owned by `internal/target`: provider artifacts must remain structured and usable in attempt history, reports, and policy feedback.
- Provider telemetry is supplemental evidence only; target success still depends on Agent Target orchestration, git evidence, required targets, reports, and policies.
- Provider adapters may execute external commands, but they must not mutate the main project checkout directly.

## Work Guidance

- Add provider-specific tests in this package when changing command construction, event capture, resume validation, or artifact schemas.
- Coordinate Bachfile syntax and user-visible behavior changes with `internal/config`, `internal/target`, and documentation.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go edits.
- Use `go run ./cmd/bach run shell/test` after provider adapter changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/agentprovider/`.
