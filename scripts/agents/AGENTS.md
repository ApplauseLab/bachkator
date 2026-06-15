# Agent Instructions

## Purpose

`scripts/agents/` owns helper scripts for running OpenCode-based agent and merge workflows from Bach targets or phase orchestration.

## Ownership

- Agent run wrappers.
- Merge-agent run wrappers.
- OpenCode session helpers.

## Local Contracts

- Scripts must preserve provider report paths and environment variables expected by Bach agent targets.
- Do not bypass Bach policy, workspace, or git evidence requirements.
- Keep scripts non-interactive for automated agent runs.

## Work Guidance

- Update prompt/report docs when script behavior changes provider invocation semantics.
- Prefer explicit arguments and environment variables over implicit shell state.

## Verification

- Use the Bach target or agent workflow that invokes the changed script when available.
- Use `go run ./cmd/bach run shell/test` when script changes affect generated artifacts or tests.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `scripts/agents/`.
