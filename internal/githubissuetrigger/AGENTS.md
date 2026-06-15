# Agent Instructions

## Purpose

`internal/githubissuetrigger/` owns the GitHub Issues Trigger Provider implementation used by the
`bach-github-issue-trigger` executable.

## Local Contracts

- Speak only the public `bach.trigger.v1` protocol through `pkg/triggerprotocol`.
- Keep GitHub API access read-only; do not mutate issues, labels, comments, or repository state.
- Read GitHub tokens from environment variables named by provider config. Never accept token values in
  Bachfile config and never log token values.
- Keep Work Item mapping deterministic: stable source IDs, issue labels preserved for Factory routing, and
  cursor advancement based on GitHub `updated_at` values.

## Verification

- Use `go run ./cmd/bach run shell/test` after provider changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/githubissuetrigger/`.
