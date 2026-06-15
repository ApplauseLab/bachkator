# Agent Instructions

## Purpose

`internal/graph/` owns project graph analysis: affected-target evidence, explain data, provenance, risk aggregation, and graph-facing types.

## Ownership

- Affected target matching from files to configured inputs/resources.
- Target explain and provenance data structures.
- Risk metadata aggregation for planning and inspection.

## Local Contracts

- Keep graph analysis read-only; execution belongs in `internal/runner`.
- Consume target-handler fragments for kind-specific explain, provenance, and composite-child facts instead of duplicating target-body switches.
- Keep CLI formatting out of this package; return structured data to adapters.
- Preserve deterministic output for agent-readable planning and tests.

## Work Guidance

- Update reference docs when graph-derived CLI output or JSON changes.
- Add focused tests for changed matching/provenance behavior.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/e2e` when affected/explain/provenance CLI behavior changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/graph/`.
