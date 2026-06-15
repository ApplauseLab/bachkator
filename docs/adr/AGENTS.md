# Agent Instructions

## Purpose

`docs/adr/` records durable architecture decisions for Bachkator's product boundary, package boundaries, extension points, runner semantics, and agent/control-plane design.

## Ownership

- Each ADR owns one accepted decision or boundary.
- ADRs explain why a stable direction exists; reference docs explain how users operate the current product.

## Local Contracts

- Keep ADRs concise and decision-focused.
- Do not use ADRs as phase logs or task status trackers.
- When a new decision supersedes an old one, update or add ADR text so the active rule is clear.

## Work Guidance

- Read relevant ADRs before changing internal package boundaries, CLI/public API boundaries, plugin behavior, target model semantics, state persistence, runner orchestration, or agent target policy behavior.
- Prefer amending nearby ADRs for small clarifications; add a new ADR for a new durable architectural decision.

## Verification

- Use `go run ./cmd/bach run shell/test` when ADR changes affect embedded docs tests or generated documentation expectations.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `docs/adr/`.
