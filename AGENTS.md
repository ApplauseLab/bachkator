# Agent Instructions

## Purpose

Bachkator is a local-first dark factory and build-system control plane for repositories where humans and coding agents need the same explicit, inspectable project operations and unattended delivery lanes. This root `AGENTS.md` is the project-wide DOX rail: it defines global workflow rules, repository-wide contracts, and the child context index that agents must follow before editing.

## Core Contract

- `AGENTS.md` files are binding work contracts for their subtrees.
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable `AGENTS.md` plus every parent `AGENTS.md` above it.
- Do not rely on memory. Re-read the applicable AGENTS chain in the current session before editing.
- If docs conflict, the closer `AGENTS.md` controls local work details, but no child doc may weaken this root contract.

## Read Before Editing

1. Read this root `AGENTS.md`.
2. Identify every file or folder you expect to touch.
3. Walk from the repository root to each target path.
4. Read every `AGENTS.md` found along each route.
5. If a parent `AGENTS.md` lists a child `AGENTS.md` whose scope contains the path, read that child and continue from there.
6. Use the nearest `AGENTS.md` as the local contract and parent docs for repo-wide rules.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning `AGENTS.md` when a change affects:

- purpose, scope, ownership, or responsibilities.
- durable structure, contracts, workflows, or operating rules.
- required inputs, outputs, permissions, constraints, side effects, or artifacts.
- user preferences about behavior, communication, process, organization, or quality.
- `AGENTS.md` creation, deletion, move, rename, or index contents.

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Bachkator Operations

Use Bachkator for all project operations in this repository. The CLI command is `bach`; during local development you can run it with `go run ./cmd/bach`.

Do not run project tools directly for normal repo work. Use Bach targets instead of `gofmt`, `go test`, `go build`, `golangci-lint`, Bats, docs generators, release scripts, or ad hoc shell pipelines. If a repeated operation is missing, add or update a Bach target first, then use that target.

Start by discovering available operations:

```sh
go run ./cmd/bach list
```

Before running expensive or side-effecting work, inspect the plan:

```sh
go run ./cmd/bach run --dry-run <target>
```

Use Bach targets instead of ad hoc commands:

```sh
go run ./cmd/bach run shell/test
go run ./cmd/bach run shell/lint
go run ./cmd/bach run shell/fmt
go run ./cmd/bach run --log-only --force group/gate
go run ./cmd/bach run shell/build
go run ./cmd/bach run shell/build-release
go run ./cmd/bach --var release_version=v0.1.2 run shell/github-release
```

When you need docs, query the embedded reference before searching manually:

```sh
go run ./cmd/bach reference
go run ./cmd/bach reference shell-targets
go run ./cmd/bach reference plugins
```

When a run fails, inspect Bach run history and logs:

```sh
go run ./cmd/bach runs list
```

Logs are under:

```text
.bach/runs/<run-id>/<target>.log
```

## Current Targets

- `agent/init_command_provider_scaffold`: run the phase 11 init-command provider scaffold agent.
- `shell/commit-msg`: validate a commit message file against the semantic commit rule.
- `shell/install-git-hooks`: configure Git to use tracked hooks from `.githooks/`.
- `shell/test`: run the Go test suite.
- `shell/lint`: run golangci-lint, parse Checkstyle output, and enforce zero lint issues.
- `shell/file-lines`: enforce the Go file size budget with a 500-line default and a baseline for existing oversized files.
- `shell/fmt`: format Go source files.
- `group/gate`: run lint, unit tests, and e2e tests as one deduplicated graph.
- `shell/build`: build the local `dist/bach` binary.
- `shell/e2e`: build `dist/bach`, install local Bats if needed, and run CLI e2e tests.
- `shell/docs-generate`: regenerate `docs/reference.md` from `docs/reference/*.md`.
- `shell/build-release`: build macOS/Linux amd64/arm64 release archives.
- `shell/github-release`: create a GitHub release with the release archives.

## Release Rule

Never create a release without a dry-run first:

```sh
go run ./cmd/bach --var release_version=vX.Y.Z run --dry-run shell/github-release
go run ./cmd/bach --var release_version=vX.Y.Z run shell/github-release
```

The release target pins the GitHub tag to `$BACH_GIT_COMMIT` and uploads all multi-platform archives.

## Repository-Wide Notes

- Do not commit `.bach/`, `dist/`, or `.opencode-snitch-off`.
- Use semantic commit messages in the form `type(scope): subject` or `type: subject`; allowed types are `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, and `revert`.
- The semantic commit hook is `.githooks/commit-msg`; install it with `go run ./cmd/bach run shell/install-git-hooks` and validate message files with `go run ./cmd/bach --var commit_msg_file=/path/to/message run shell/commit-msg`. The install target refuses to replace an existing custom local hook path.
- `shell/fmt` and `shell/lint` require a golangci-lint v2-compatible binary on `PATH`; `.golangci.yml` enables `gofmt`, `golines` at 100 columns, and `dupl` duplication checks.
- Keep lint output bounded; the `shell/lint` target caps golangci-lint text and Checkstyle output to the first 10 total findings so run logs and artifacts stay readable.
- Keep Go files at or below 500 lines. Existing larger files are listed in `docs/architecture/go-file-size-baseline.txt`; do not let baseline files grow, and split them during architecture work.
- Never run `gofmt`, `go test`, `go build`, `golangci-lint`, Bats, or docs generators directly for routine work; use Bach targets so cache, logs, quality gates, and affected-target logic stay authoritative.
- Prefer `bach reference <topic>` over reading source when asking about supported Bachfile syntax.
- If a Bach target is missing for a repeated operation, add or update the Bachfile instead of documenting another standalone command.
- Keep docs concise, current, and operational. Document stable contracts, not diary entries.
- Put broad rules in parent docs and concrete details in child docs.
- Prefer direct bullets with explicit names.
- Delete stale notes instead of explaining history.
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist.

## Parallel Phase Work

When orchestrating implementation phases in parallel, prefer a dedicated Bachfile such as `Bachfile.batch` over ad hoc shell scripts. Use a `pipeline` target for the sequential merge/test phase, and use normal `depends_on` fan-out for parallel implementation targets.

For non-interactive OpenCode workers, use `opencode run <prompt>`. If the user explicitly approves broad worktree access, pass `--dangerously-skip-permissions`; otherwise agents may be blocked from reading or testing external worktrees.

Phase plans under `plans/` are intentionally ignored. Prompts should pass explicit absolute plan paths instead of relying on glob search from a new worktree.

Each phase worker should:

- Create or reuse a separate worktree outside this checkout.
- Use branch names like `bach/phase-N`.
- Leave the main checkout untouched.
- Start with `go run ./cmd/bach list` and dry-run the likely gates, usually `go run ./cmd/bach run --dry-run shell/lint` and `go run ./cmd/bach run --dry-run shell/test`.
- After edits, run `go run ./cmd/bach affected` to choose focused tests.
- Run focused Bach targets repeatedly, then `go run ./cmd/bach run --log-only --force group/gate` before committing so quality reports and gates execute instead of relying on cached status.
- Commit only intended phase files on the phase branch and do not merge back.

Merge phases back sequentially. After every merge or conflict resolution, run:

```sh
go run ./cmd/bach affected
```

If conflicts occur, preserve all existing target fields and runner behavior unless the phase explicitly replaces them. Recent conflict hotspots are:

- `internal/config/config_types.go`: keep metadata fields, profiles, `quiet`, `lock`, `steps`, env blocks, and image fields together.
- `internal/runner/runner.go`, `executor.go`, `scheduler.go`, and `logs.go`: preserve pipeline execution, profile/env layering, quiet/log-only streaming, target labels, and lock manager plumbing.
- `internal/cli/flags.go`: avoid duplicate option fields or duplicate flag bindings when phases add CLI flags.
- `docs/reference.md` and `docs/agent-guide.md`: merge documentation sections rather than choosing one side.

Avoid shell-local variables like `$out` inside Bach `shell` strings because Bach expands `$NAME` before `/bin/sh` runs. Prefer command pipelines that do not depend on shell-local variable expansion, or move complex logic into a checked-in script/Go test.

## Closeout

1. Re-check changed paths against the AGENTS chain.
2. Update nearest owning docs and any affected parents or children.
3. Refresh every affected Child DOX Index.
4. Remove stale or contradictory text.
5. Run existing Bach verification when relevant.
6. Report any docs intentionally left unchanged and why.

## User Preferences

- Integrate the DOX AGENTS.md hierarchy in Bachkator by keeping scoped `AGENTS.md` files current as durable work contracts.

## Child DOX Index

- `cmd/AGENTS.md`: executable entry points and command-specific generation binaries.
- `internal/AGENTS.md`: internal Go packages, domain boundaries, and import-direction rules.
- `docs/AGENTS.md`: documentation, reference fragments, ADRs, schemas, and agent-facing guides.
- `examples/AGENTS.md`: example Bachfiles, plugins, and sample projects.
- `prompts/AGENTS.md`: reusable agent prompt assets.
- `scripts/AGENTS.md`: checked-in helper scripts used by Bach targets or phase workflows.
- `test/AGENTS.md`: e2e tests and test fixtures.

# Go Coding Guidelines

## Core Principles
- Follow idiomatic Go and Effective Go conventions.
- Prefer clarity over cleverness.
- Optimize for readability and maintainability.
- Write code that is easy to understand for developers unfamiliar with the codebase.
- Choose the simplest solution that meets the requirements.

## Code Structure
- Keep functions small and focused on a single responsibility.
- Use descriptive names; avoid unnecessary abbreviations.
- Prefer composition over complex inheritance-like patterns.
- Avoid premature abstraction.
- Define interfaces where they are consumed, not where they are implemented.
- Prefer concrete types until an interface is clearly justified.

## Control Flow
- Prefer early returns to reduce nesting.
- Keep control flow straightforward and easy to follow.
- Minimize cognitive complexity.
- Avoid unnecessary indirection.

## Error Handling
- Handle errors explicitly.
- Return errors instead of using panic for expected failures.
- Add useful context when wrapping errors.
- Keep error paths clear and readable.

## Formatting & Tooling
- Use standard Go import organization.
- Ensure code passes `bach run shell/lint` and configured linters.
- Do not introduce non-standard style conventions without strong justification.

## Concurrency
- Use goroutines only when they provide clear value.
- Keep concurrent code simple and safe.
- Avoid unnecessary synchronization complexity.
- Favor correctness and readability over clever concurrency patterns.

## Testing
- Write clear, deterministic tests.
- Prefer table-driven tests when appropriate.
- Test behavior, not implementation details.
- Keep test code readable and maintainable.

## Performance
- Prioritize correctness and readability before optimization.
- Optimize only when supported by measurement or clear evidence.
- Document non-obvious performance tradeoffs.

## Review Checklist
Before submitting code, verify:
- Is it idiomatic Go?
- Can it be simplified?
- Is the intent obvious?
- Are errors handled clearly?
- Is every abstraction justified?
- Would a new team member understand it quickly?
