# Agent Instructions

## Purpose

`docs/reference/` contains the source fragments for Bachkator's generated reference documentation.

## Ownership

- Each Markdown file owns one reference topic or closely related topic group.
- `docs/reference.md` is generated from these fragments.

## Local Contracts

- Edit source fragments here instead of editing `docs/reference.md` directly.
- Keep fragment names topic-based and stable.
- Keep command examples aligned with the actual CLI and Bachfile syntax.
- Avoid documenting unsupported syntax or future behavior as current behavior.

## Work Guidance

- When adding public behavior, add or update the smallest relevant fragment.
- When removing behavior, delete or rewrite stale reference text in the same change.
- Prefer short examples that agents can reuse without project-specific assumptions.

## Verification

- Use `go run ./cmd/bach run shell/docs-generate` after any fragment edit.
- Use `go run ./cmd/bach run shell/test` if generated docs or docs tests may change.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `docs/reference/`.
