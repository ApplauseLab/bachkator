# Agent Instructions

## Purpose

`internal/registry/` owns small, type-safe registry mechanics shared by subsystem-specific registries.

## Ownership

- Generic duplicate-key and missing-key registry mechanics.
- Zero-value-safe typed storage for subsystem registry wrappers.
- Type-independent registry tests.

## Local Contracts

- Keep domain-specific wrapper types in their owning packages.
- Keep domain validation and error strings in wrapper packages, not in the generic primitive.
- Do not add broad utility helpers here; this package is only for repeated registry invariants.
- Registries are setup-time composition primitives. Populate them before concurrent use; do not add runtime mutation without explicit synchronization.

## Work Guidance

- Add behavior here only when at least two subsystem registries share the same invariant.
- Keep ordering, validation, and domain-specific construction in wrapper packages.
- Prefer explicit wrapper tests when changing registry semantics.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/registry/`.
