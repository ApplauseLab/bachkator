# Agent Instructions

## Purpose

`docs/` owns durable product documentation: generated and source reference docs, ADRs, schemas, design notes, and agent-facing guides.

## Ownership

- `index.html`: human-readable landing page for the documentation tree.
- `reference/`: source fragments for the generated Bach reference.
- `reference.md`: generated output; do not edit directly.
- `adr/`: architectural decisions and durable design rationale.
- `schemas/`: JSON schemas for documented machine-readable contracts.
- `agents.md`: generated or maintained agent-facing guide content mirrored from reference behavior.
- `architecture/` and `designs/`: durable design notes that are less formal than ADRs.

## Local Contracts

- Keep documentation operational, concise, and current.
- Document stable contracts, not implementation diary entries.
- Do not edit `docs/reference.md` directly for reference changes; edit `docs/reference/*.md` and regenerate.
- Public CLI, Bachfile syntax, quality report, agent report, policy, and JSON export changes require documentation updates.
- Remove stale or contradictory docs instead of adding historical explanations.

## Work Guidance

- Use feature/topic names for reference fragments, not implementation phase numbers.
- Keep examples valid and small enough for agents to copy safely.
- Prefer ADR updates when a decision or boundary changes; prefer reference updates when user-facing syntax or behavior changes.

## Verification

- Use `go run ./cmd/bach run shell/docs-generate` after reference fragment changes.
- Use `go run ./cmd/bach run shell/test` when docs tests or embedded docs behavior may be affected.

## Child DOX Index

- `docs/adr/AGENTS.md`: architectural decision records.
- `docs/architecture/AGENTS.md`: architecture planning materials and migration design context.
- `docs/designs/AGENTS.md`: design proposals and evolving workflow notes.
- `docs/reference/AGENTS.md`: source fragments for generated reference docs.
- `docs/schemas/AGENTS.md`: JSON schemas for machine-readable contracts.
