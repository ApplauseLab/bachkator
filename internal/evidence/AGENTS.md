# Agent Instructions

## Purpose

`internal/evidence/` owns trusted local evidence path handling for Bach-owned files, provider artifacts, Agent Target workspaces, and private State Store files.

## Ownership

- Runtime path resolution for project files and Agent Target workspaces.
- Private file and JSON artifact writes for evidence that may contain prompts, provider telemetry, policy results, or run records.
- Symlink rejection at trust boundaries between provider-writable workspaces and Bach-owned evidence.

## Local Contracts

- Keep this package internal; it is not a public plugin or file layout API.
- Do not import CLI, runner, target, config, quality, state, or provider packages from here.
- Preserve existing public Bachfile syntax while enforcing resolved runtime paths.
- Prefer small, boring helpers over generic filesystem abstractions.

## Work Guidance

- Add focused tests for symlink, path containment, and private permission changes.
- Keep errors actionable and tied to the user-provided path where possible.
- Do not broaden this package into unrelated file utilities.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go edits.
- Use `go run ./cmd/bach run shell/test` after evidence boundary changes.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/evidence/`.
