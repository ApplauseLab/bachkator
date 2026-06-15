# Agent Instructions

## Purpose

`internal/model/` owns shared domain model types for projects, targets, inputs, resources, target addresses, and normalized shared contracts.

## Ownership

- Canonical target addresses and identity semantics.
- Shared target/input/resource structures consumed across config, graph, runner, and CLI packages.
- Typed status constants (`RunStatus`, `Lifecycle`, `Priority`) used by runner, state, factory, backend protocol, and CLI packages.

## Local Contracts

- Preserve the unified Target model; do not fork target-kind-specific universes here.
- Keep model types free of CLI, HCL decoding, execution, and persistence details unless the type is intentionally shared.
- Target address parsing and formatting are public-facing semantics and must remain stable.

## Work Guidance

- Keep shared types minimal; add behavior to owning subsystems when it is not truly cross-cutting.
- Update tests and reference docs when address syntax or shared terminology changes.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/fmt` after Go edits.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/model/`.
