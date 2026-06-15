# Agent Instructions

## Purpose

`cmd/` owns executable entry points for Bachkator binaries. Entry points should stay thin and delegate application assembly, command behavior, and domain logic to internal packages.

## Ownership

- `cmd/bach`: main Bach CLI executable.
- `cmd/bach-docs-gen`: reference documentation generator invoked by the `shell/docs-generate` Bach target.
- `cmd/bach-file-lines`: internal Go file length checker for architecture hygiene.
- `cmd/bach-lint-cap`: internal lint-report capping helper invoked by the `shell/lint` Bach target.
- Command wiring and production dependency assembly belong in `internal/app` and `internal/cli`, not in executable packages.

## Local Contracts

- Keep executable packages small and side-effect-light.
- Do not add domain behavior, config parsing rules, runner logic, quality parsing, or state-store behavior here.
- Public CLI behavior changes must be reflected in `docs/reference/*.md` and generated into `docs/reference.md`.

## Work Guidance

- Prefer adding or changing CLI command adapters under `internal/cli` before touching `cmd/bach`.
- Prefer adding or changing docs-generation behavior in the generator package only when the reference source format changes.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go edits.
- Use `go run ./cmd/bach run shell/test` for focused executable or generator changes.
- Use `go run ./cmd/bach run shell/docs-generate` when docs-generation output may change.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `cmd/`.
