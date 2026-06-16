# Agent Instructions

## Purpose

`internal/` owns Bachkator's Go implementation: application composition, CLI adapters, Bachfile config, Factory queue services, Markdown Plan status and execution services, graph loading, target modeling, runner behavior, state persistence, quality reports, and target-kind handlers.

## Ownership

- `app`: composition root for production dependencies and built-in registries.
- `agentprovider`: private Agent Target provider adapters and provider evidence capture.
- `backend`: internal Backend client facade, provider lifecycle, protocol DTO mapping, and built-in Backend providers.
- `bacherr`: shared domain sentinel errors and error-class predicates used across subsystems.
- `clock`: shared UTC clock defaults and injectable `NowFunc` helpers for deterministic timestamping.
- `cli`: Cobra command adapters and public CLI contract presentation.
- `config`: Bachfile loading, decoding, validation, and config-time references.
- `dag`, `graph`, `model`, and `target`: target model, dependency graph, graph plugins, and target-kind registration behavior.
- `githubissuetrigger`: GitHub Issues Trigger Provider implementation.
- `evidence`: trusted local evidence path handling, private artifact writes, and Agent Target workspace path resolution.
- `factory`: Factory Work Item service logic, manual queue validation, intake evidence creation, and queue-facing DTOs.
- `factorydaemon`: Factory daemon leases, queue polling, Work Item phase orchestration, and workflow execution.
- `plan`: side-effect-free Markdown Plan parsing, validation, hashing, graph construction, and status derivation.
- `planbatch`: multi-Plan batch execution and review-queue orchestration.
- `planexecute`: Plan implementation orchestration through generated Agent Targets and lifecycle ledger writes.
- `planstatus`: Plan status use-case orchestration over safe file reads and Backend ledger reads.
- `runner`: run planning, scheduling, execution, caching, logs, retries, locks, and agent target orchestration.
- `quality`: quality report parsing, findings, metrics, and gate evaluation.
- `query`: read-only query DTO assembly over private persistence and domain records.
- `registry`: shared typed registry mechanics for subsystem-specific registries.
- `runenv`: runtime command environment expansion and deterministic env list conversion.
- `state`: legacy private local SQLite persistence package being absorbed behind the Backend provider boundary.
- `git`: git environment and repository evidence helpers.

## Local Contracts

- Preserve the CLI Contract as the public product boundary; do not create a public Go API by accident.
- Keep `internal/app` as the composition root; avoid package-level production wiring and global service locators.
- Keep Cobra-specific code in `internal/cli` or CLI adapter layers, not in domain packages.
- Preserve the unified Target model. Prefer target-kind handlers and subsystem registries over parallel target universes.
- Avoid import cycles by keeping domain packages independent from CLI presentation and executable packages.
- State store schema and files are private implementation details; expose evidence through CLI output, logs, reports, and JSON exports.
- Prefer `internal/bacherr` sentinel errors for cross-cutting error classes (`ErrNotFound`, `ErrValidationFailed`, etc.) instead of ad-hoc string errors. CLI and composition layers own presentation and exit-code mapping.

## Work Guidance

- Read relevant ADRs under `docs/adr/` before changing package boundaries, extension points, target model shape, state semantics, or runner orchestration.
- Keep changes minimal and local to the owning subsystem unless a cross-cutting contract is actually changing.
- When adding Bachfile syntax, update config types, validation, docs reference fragments, examples if useful, and tests in the same change.
- When changing public CLI output or flags, update reference docs and e2e coverage where behavior is user-visible.

## Verification

- Use `go run ./cmd/bach run shell/fmt` after Go edits.
- Use `go run ./cmd/bach run shell/test` for normal internal package changes.
- Use `go run ./cmd/bach run --dry-run shell/lint` before expensive lint runs.
- Use `go run ./cmd/bach run --log-only --force group/gate` for broad or release-facing changes.

## Child DOX Index

- `internal/app/AGENTS.md`: production composition root and subsystem wiring.
- `internal/agentprovider/AGENTS.md`: private Agent Target provider adapters and provider evidence capture.
- `internal/backend/AGENTS.md`: Backend client facade, provider lifecycle, protocol mapping, and built-in providers.
- `internal/bacherr/AGENTS.md`: shared domain sentinel errors and error-class predicates.
- `internal/cli/AGENTS.md`: CLI command adapters, flags, and output contracts.
- `internal/config/AGENTS.md`: Bachfile loading, syntax, validation, and runtime project conversion.
- `internal/dag/AGENTS.md`: generic DAG behavior and deterministic ordering.
- `internal/evidence/AGENTS.md`: trusted local evidence paths, private artifact writes, and runtime workspace safety.
- `internal/factory/AGENTS.md`: Factory Work Item service logic and manual queue contracts.
- `internal/factorydaemon/AGENTS.md`: Factory daemon leases, polling, claims, and workflow phase orchestration.
- `internal/git/AGENTS.md`: git evidence helpers.
- `internal/graph/AGENTS.md`: affected-target, explain, provenance, and risk graph analysis.
- `internal/githubissuetrigger/AGENTS.md`: GitHub Issues Trigger Provider implementation.
- `internal/model/AGENTS.md`: shared domain model and target address semantics.
- `internal/plan/AGENTS.md`: Markdown Plan parsing, validation, hashing, graph construction, and pure status derivation.
- `internal/planbatch/AGENTS.md`: multi-Plan batch execution and review-queue orchestration.
- `internal/planexecute/AGENTS.md`: Plan implementation orchestration through generated Agent Targets and lifecycle ledger writes.
- `internal/planstatus/AGENTS.md`: Plan status file loading, reference validation, and Backend ledger read orchestration.
- `internal/quality/AGENTS.md`: quality reports, findings, metrics, gates, and policy evidence.
- `internal/query/AGENTS.md`: read-only query DTO assembly for CLI adapters and future evidence surfaces.
- `internal/registry/AGENTS.md`: shared typed registry mechanics for subsystem-specific registries.
- `internal/runner/AGENTS.md`: run planning, scheduling, execution, logs, cache, and agent orchestration.
- `internal/runenv/AGENTS.md`: runtime command env expansion and deterministic env list conversion.
- `internal/state/AGENTS.md`: private local State Store and persistence contracts.
- `internal/target/AGENTS.md`: built-in target-kind handlers and target-specific semantics.
