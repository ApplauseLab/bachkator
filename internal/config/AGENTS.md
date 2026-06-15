# Agent Instructions

## Purpose

`internal/config/` owns Bachfile loading, decoding, validation, imports, variables, computed values, environment/profile layering, plugin declarations, and conversion into runtime project structures.

## Ownership

- HCL schema and decode behavior.
- Bachfile reference validation and diagnostic codes.
- Config registries for target kinds and related blocks.
- Runtime project construction from loaded config.

## Local Contracts

- Bachfile syntax is public product surface; keep docs, examples, tests, and validation in sync.
- Prefer typed references and explicit validation errors over stringly implicit behavior.
- Do not perform target execution, host probing, or state mutation while loading config.
- Preserve import/profile/env ordering semantics when adding fields.
- Keep config-time target-kind facts in local adapters such as `target_kind.go`; do not duplicate target prefix checks or body assertions across validation loops.

## Work Guidance

- When adding a field, update config types, decode/eval/validation, reference docs, and tests together.
- Keep diagnostics stable and machine-readable when they are exposed by `bach validate --json`.
- Avoid backward-compatibility shims unless persisted or shipped behavior requires them.

## Verification

- Use `go run ./cmd/bach run shell/test` after config changes.
- Use `go run ./cmd/bach run shell/e2e` when validation or user-facing Bachfile behavior changes.
- Use `go run ./cmd/bach run shell/docs-generate` after reference updates.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/config/`.
