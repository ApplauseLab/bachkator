# Agent Instructions

## Purpose

`scripts/` owns checked-in helper scripts invoked by Bach targets, release workflows, e2e tests, or phase orchestration.

## Ownership

- Scripts should support Bach-declared operations rather than replacing Bach targets.
- Complex shell needed by a repeated operation may live here when keeping it inside a Bachfile would be brittle.

## Local Contracts

- Do not add standalone project workflows here without a corresponding Bach target when the operation is repeated.
- Keep scripts non-interactive unless the caller target explicitly models confirmation or risk.
- Avoid shell-local variables inside Bach `shell` strings; move complex logic into scripts when needed.
- Scripts must avoid committing or depending on `.bach/`, `dist/`, or local-only artifacts unless those paths are declared target outputs or tools.

## Work Guidance

- Prefer POSIX `sh` for scripts invoked by existing Bach targets unless a stronger shell requirement is documented.
- Keep release scripts deterministic and tied to `$BACH_GIT_COMMIT` when publishing artifacts.

## Verification

- Use the Bach target that invokes the changed script.
- Use `go run ./cmd/bach run --dry-run <target>` before release or side-effecting script targets.

## Child DOX Index

- `scripts/agents/AGENTS.md`: agent and merge workflow helper scripts.
- `scripts/architecture-migration/AGENTS.md`: architecture migration phase helper scripts.
- `scripts/overnight/AGENTS.md`: unattended overnight agent workflow scripts.
