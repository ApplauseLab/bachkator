# Agent Instructions

## Purpose

`internal/app/` is the production composition root for Bachkator. It wires CLI adapters, subsystem registries, target-kind handlers, quality handlers, config loaders, runner dependencies, and state access.

## Ownership

- Application assembly and dependency construction.
- App-owned lookup from CLI project DTOs to loaded config projects.
- Built-in subsystem registration order.
- Production defaults used by `cmd/bach`.

## Local Contracts

- Keep `cmd/bach` thin by placing production wiring here.
- Do not hide dependencies in package-level mutable globals.
- Do not put CLI presentation, domain algorithms, or target execution logic here.
- Keep private config-project backing out of `internal/cli`; register loaded projects in app-owned wiring when adapters need config context.
- Preserve explicit registration so tests can assemble smaller compositions.

## Work Guidance

- Add new subsystem registrations here after the owning package exposes a focused constructor or adapter.
- Keep initialization deterministic and easy to inspect.
- When registration affects public behavior, update CLI/reference docs and tests.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/app/`.
