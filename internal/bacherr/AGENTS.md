# Agent Instructions

## Purpose

`internal/bacherr/` owns shared domain sentinel errors used across Bachkator packages to distinguish error classes without leaking CLI presentation or backend protocol details.

## Ownership

- Domain-wide error sentinels such as `ErrNotFound`, `ErrAlreadyExists`, `ErrValidationFailed`, `ErrUnsupported`, `ErrCancelled`, and `ErrUsage`.
- Helper constructors that wrap these sentinels with formatted messages.
- Predicate helpers for callers that need to branch on error class.

## Local Contracts

- Keep error messages here presentation-free. Do not embed CLI usage text in `bacherr` errors.
- Domain packages return `bacherr` sentinel-wrapped errors for reusable error classes.
- CLI and composition layers decide how to render each class, including exit codes and usage formatting.
- Do not make this package depend on other internal packages; it is a leaf utility.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/bacherr/`.
