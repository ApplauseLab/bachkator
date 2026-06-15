# Agent Instructions

## Purpose

`internal/runenv/` owns runtime command environment expansion and deterministic environment list conversion shared by command-executing packages.

## Ownership

- `$(RUN_DIRECTORY)` expansion through `BACH_RUN_DIRECTORY`.
- Shell-style `$NAME` and `${NAME}` expansion from an explicit runtime env map.
- Deterministic `KEY=value` list conversion for process execution.

## Local Contracts

- Preserve exact command environment semantics; missing variables expand to the empty string.
- Keep output ordering deterministic by sorting keys lexicographically.
- Do not move project, profile, dotenv, git, or run-directory env layering into this package.
- Do not import runner, target, agentprovider, config, state, app, or CLI packages.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go edits.
- Use `go run ./cmd/bach run shell/test` after behavior changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/runenv/`.
