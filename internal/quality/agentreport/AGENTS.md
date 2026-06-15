# Agent Instructions

## Purpose

`internal/quality/agentreport/` owns the public `bach.agent_report.v1` artifact DTO and project-independent helpers used by `bach report` to create or mutate quality report files.

## Local Contracts

- Keep this package free of Cobra, Project loading, State Store writes, runner behavior, and target execution logic.
- Preserve compatibility with the `agent-report-v1` quality parser in `internal/quality`.
- Validate and write artifacts only; normal quality ingestion remains responsible for State Store persistence and gate evaluation.
- Keep report writes local-first and safe: sidecar lock, atomic same-directory rename, explicit external-path opt-in, and strict JSON decoding where user input is accepted.

## Verification

- Use `go run ./cmd/bach run shell/test` after helper changes.
- Use `go run ./cmd/bach run shell/lint` after validation, path handling, or JSON changes.
