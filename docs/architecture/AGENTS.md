# Agent Instructions

## Purpose

`docs/architecture/` owns architecture planning materials that support larger design or migration work before it becomes an ADR or reference contract.

## Ownership

- Architecture plans and supporting design materials.
- Transitional architecture notes that may inform ADRs.
- Go file-size hygiene baseline in `go-file-size-baseline.txt`.

## Local Contracts

- Keep durable architecture conclusions reflected in ADRs when they become accepted decisions.
- Do not let planning notes contradict current reference docs or ADRs.
- Prefer clear current-state guidance over historical phase narrative.

## Work Guidance

- Move stable decisions into `docs/adr/` when they become binding.
- Keep plans scoped and tied to concrete package or product boundaries.

## Verification

- Use `go run ./cmd/bach run shell/test` when docs tests or embedded docs behavior may be affected.
- Use `go run ./cmd/bach run shell/file-lines` after editing `go-file-size-baseline.txt`.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `docs/architecture/`.
