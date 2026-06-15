# Agent Instructions

## Purpose

`internal/git/` owns local git evidence helpers used by runtime environment, release targeting, changed-file detection, and agent workspace evidence.

## Ownership

- Repository metadata collection.
- Commit, branch, dirty state, and changed-file helpers.
- Git-derived environment values surfaced through Bach contracts.

## Local Contracts

- Keep git operations read-only unless a caller explicitly owns mutation semantics.
- Do not embed provider-specific agent behavior here.
- Return evidence in forms that callers can include in logs, JSON, and generated prompts.

## Work Guidance

- Handle detached HEAD, missing git metadata, and dirty worktrees deliberately.
- Keep shell-outs bounded and predictable.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/git/`.
