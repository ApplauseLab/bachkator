# Agent Instructions

## Purpose

`examples/plan-agents/` demonstrates plan-driven agent workflows using Bach agent targets and verification Bachfiles.

## Ownership

- Example Bachfiles for agent implementation and verification flows.
- README explanation of how to inspect and run the example.
- Helper scripts local to the example.

## Local Contracts

- Keep examples safe to dry-run and inspect before execution.
- Do not require private provider credentials or machine-specific paths.
- Keep agent workflow examples aligned with current agent target syntax and report contracts.

## Work Guidance

- Update this example when agent target, policy, prompt, or provider syntax changes.
- Prefer explicit plan paths and target names over glob-dependent behavior.

## Verification

- Use `go run ./cmd/bach run shell/test` after docs or example changes that affect embedded tests.
- Use relevant example Bach dry-runs manually only when needed and safe.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `examples/plan-agents/`.
