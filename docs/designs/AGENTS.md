# Agent Instructions

## Purpose

`docs/designs/` owns design notes for proposed or evolving behavior that is more detailed than reference docs and less final than ADRs.

## Ownership

- Design sketches and product workflow proposals.
- Cross-feature rationale that is not yet an accepted architecture decision.

## Local Contracts

- Label current design intent clearly; do not present proposals as shipped reference behavior.
- Promote accepted architectural decisions to ADRs.
- Promote shipped user-facing behavior to reference fragments.

## Work Guidance

- Keep designs concise enough for agents to use as task context.
- Remove or rewrite stale designs when implementation diverges.

## Verification

- Use `go run ./cmd/bach run shell/test` when docs tests or embedded docs behavior may be affected.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `docs/designs/`.
