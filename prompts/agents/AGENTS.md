# Agent Instructions

## Purpose

`prompts/agents/` owns reusable role prompts for implementer, reviewer, docs, security, and merge agent targets.

## Ownership

- `implementer.md`: implementation agent guidance.
- `architecture-review.md`: architecture reviewer guidance.
- `docs-sweeper.md`: documentation reviewer guidance.
- `security-review.md`: security reviewer guidance.
- `merge.md`: merge agent guidance.

## Local Contracts

- Prompt files are guidance, not schemas; generated Bach prompts inject required report contracts.
- Keep prompts compatible with non-interactive provider execution.
- Do not include secrets, absolute local paths, or assumptions about the caller's checkout.
- Preserve clear severity and evidence expectations for reviewers.

## Work Guidance

- Update prompts when agent roles, reports, policies, or merge evidence requirements change.
- Prefer concise, testable instructions over broad motivational prose.

## Verification

- Use `go run ./cmd/bach run shell/test` when prompt changes affect generated agent artifacts or docs tests.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `prompts/agents/`.
