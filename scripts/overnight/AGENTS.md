# Agent Instructions

## Purpose

`scripts/overnight/` owns helper scripts for long-running overnight agent and merge workflows.

## Ownership

- Overnight implementation agent wrappers.
- Overnight merge agent wrappers.

## Local Contracts

- Scripts must be safe for unattended execution and produce inspectable logs.
- Do not hide destructive git or release behavior behind overnight helpers.
- Preserve branch, worktree, and report evidence expectations.

## Work Guidance

- Prefer explicit logging and failure exits over best-effort continuation.
- Keep long-running workflow behavior represented in Bach targets where possible.

## Verification

- Use relevant dry-runs or workflow targets when available.
- Use `go run ./cmd/bach run shell/test` when script behavior affects tested orchestration.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `scripts/overnight/`.
