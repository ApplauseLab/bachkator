# Agent Instructions

## Purpose

`prompts/` owns reusable prompt assets used by Bachkator agent targets and agent workflow examples.

## Ownership

- Prompt files provide task guidance for providers.
- Bachkator-generated prompts remain responsible for injecting required context, environment paths, and structured report contracts.

## Local Contracts

- Keep prompts provider-usable and explicit about expected behavior.
- Do not treat prompt prose as the source of truth for report schemas or policy semantics.
- Avoid embedding secrets, machine-specific paths, or assumptions that break managed clone workspaces.

## Work Guidance

- Update prompts when agent target roles, policy expectations, report fields, or merge/review behavior changes.
- Prefer concise instruction blocks over long conversational prompt text.

## Verification

- Use `go run ./cmd/bach run shell/test` when prompt changes affect generated agent artifacts or docs tests.

## Child DOX Index

- `prompts/agents/AGENTS.md`: reusable role prompts for agent targets.
