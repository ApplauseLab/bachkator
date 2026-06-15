# Agent Instructions

## Purpose

`examples/` owns sample Bachfiles, plugins, and demonstration projects that show supported Bachkator workflows.

## Ownership

- Examples should demonstrate stable user-facing behavior.
- Example plugins should match documented plugin lifecycle and stdout contracts.
- Example agent workflows should be safe to inspect and adapt.

## Local Contracts

- Keep examples minimal but complete enough to run or reason about.
- Do not introduce examples for speculative or unsupported syntax.
- Keep example commands aligned with current reference docs.

## Work Guidance

- Update examples when changing syntax, plugin contracts, agent target behavior, or recommended project layout.
- Prefer a focused new example over expanding one example into a mixed showcase.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go example edits.
- Use `go run ./cmd/bach run shell/test` when example Go packages or docs tests are affected.

## Child DOX Index

- `examples/opencode-provider/AGENTS.md`: first-class OpenCode provider, supplemental report, policy, and resume-session examples.
- `examples/plan-agents/AGENTS.md`: plan-driven agent workflow examples.
- `examples/plugins/AGENTS.md`: graph and quality plugin examples.
