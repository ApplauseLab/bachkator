# Agent Instructions

## Purpose

`internal/quality/` owns quality report ingestion, normalized metrics/findings, quality handler registration, quality gates, and applied policy evidence parsing.

## Ownership

- Built-in quality report parsers and handler registry.
- Normalized report, metric, finding, and gate-result records plus gate evaluation.
- Agent report and applied policy quality evidence ingestion.

## Local Contracts

- Quality handlers parse successful target outputs; they do not run targets.
- Quality ingestion may receive persistence callbacks, but State Store owns durable writes.
- Keep normalized report structures stable when they are exposed through CLI JSON or schemas.
- Do not put target-kind-specific execution logic in quality handlers.

## Work Guidance

- Update schemas, reference docs, and tests when report formats or exposed fields change.
- Keep parser errors actionable and tied to report paths.

## Verification

- Use `go run ./cmd/bach run shell/test` after changes.
- Use `go run ./cmd/bach run shell/e2e` when quality CLI behavior changes.

## Child DOX Index

- `internal/quality/agentreport/AGENTS.md`: project-independent `bach.agent_report.v1` artifact DTOs, validation, and safe file mutation helpers.
