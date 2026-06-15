# Agent Instructions

## Purpose

`scripts/architecture-migration/` owns helper scripts for architecture migration phase agents and merge agents.

## Ownership

- Migration agent orchestration scripts.
- Migration merge orchestration scripts.
- OpenCode session helpers for migration worktrees.

## Local Contracts

- Scripts should leave the main checkout untouched unless explicitly used for a merge step.
- Preserve phase branch/worktree expectations documented in the root `AGENTS.md`.
- Do not silently reset or discard worktree changes.

## Work Guidance

- Keep script behavior aligned with the root Parallel Phase Work rules.
- Prefer explicit phase/worktree inputs over deriving paths from fragile shell state.

## Verification

- Use relevant dry-runs or phase workflow targets when available.
- Use `go run ./cmd/bach run shell/test` when script behavior affects tested orchestration.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `scripts/architecture-migration/`.
