# Agent Instructions

## Purpose

`internal/target/` owns built-in target-kind handlers and target-specific behavior for shell, image, pipeline, group, agent, and policy targets.

## Ownership

- Built-in target-kind registration and validation hooks.
- Target-kind-specific planning, explanation, and execution adapters where applicable.
- Policy target behavior derived from agent acceptance policies.

## Local Contracts

- Implement target kinds through handlers over the shared Target model.
- Keep target-specific fingerprint parts and composite child discovery in handlers; the runner consumes those handler results for planning and orchestration.
- Do not create a separate execution universe outside the runner.
- Keep Bachfile syntax, handler behavior, docs, and tests aligned for each target kind.
- Agent and policy targets must preserve structured report and policy evidence contracts.
- Agent `plan` mode is generated only by the Factory daemon; it writes a Plan file to `BACH_PLAN_OUTPUT_PATH`, does not require an Agent Report, and skips the workspace-clean checks that apply to implementation runs. Plan providers still must not mutate Bach-owned policy evidence.

## Work Guidance

- Update reference fragments and examples when target-kind syntax or behavior changes.
- Add tests for registration, validation, explanation, and runtime behavior touched by a target kind.

## Verification

- Use `go run ./cmd/bach run shell/test` after target handler changes.
- Use `go run ./cmd/bach run shell/e2e` for user-visible target behavior changes.
- Use `go run ./cmd/bach run shell/docs-generate` after reference updates.

## Child DOX Index

- No child `AGENTS.md` files currently exist under `internal/target/`.
