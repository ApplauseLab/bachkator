# Agent Instructions

## Purpose

`internal/cli/` owns Bachkator's public CLI contract presentation through Cobra commands, flags, output formats, and command adapters.

## Ownership

- Root command construction and shared flag handling.
- Command adapters for list, run, explain, affected, validate, reference, provenance, graph, quality, report, runs, factory, plan, and init behavior.
- Human-readable output and JSON output contracts for CLI commands.

## Local Contracts

- Keep Cobra-specific code in this package.
- Keep `Project` as a presentation/adapter DTO; do not attach private config or State Store backing objects to it.
- Domain packages should expose functions and types, not Cobra commands.
- Public flags, command names, exit behavior, JSON fields, and user-visible output are product contracts.
- Do not read or write private state directly when a domain service or runner API owns the behavior.
- Return CLI usage mistakes as `bacherr.ErrUsage`-wrapped errors from command adapters so the entry point can render them consistently and exit with code `2`. Domain packages must not embed CLI usage text in their errors; adapters wrap domain sentinels in usage errors when necessary.
- Exit codes are a public contract: `0` success, `1` general failure, `2` invalid arguments or command usage.

## Work Guidance

- Update `docs/reference/*.md` when adding or changing public commands, flags, output, or errors.
- Add or update e2e tests when behavior is user-visible at the terminal.
- Prefer small command adapters that delegate to domain packages.

## Verification

- Use `go run ./cmd/bach run shell/test` for command adapter changes.
- Use `go run ./cmd/bach run shell/e2e` for user-visible CLI behavior changes.
- Use `go run ./cmd/bach run shell/docs-generate` after reference fragment changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/cli/`.
