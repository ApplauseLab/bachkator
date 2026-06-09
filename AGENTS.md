# Agent Instructions

Use Bachkator for all project operations in this repository. The CLI command is `bach`; during local development you can run it with `go run ./cmd/bach`.

## Required Workflow

Start by discovering available operations:

```sh
go run ./cmd/bach list
```

Before running expensive or side-effecting work, inspect the plan:

```sh
go run ./cmd/bach --dry-run run <target>
```

Use Bach targets instead of ad hoc commands:

```sh
go run ./cmd/bach run shell/test
go run ./cmd/bach run shell/lint
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
go run ./cmd/bach runs
```

Logs are under:

```text
.bach/runs/<run-id>/<target>.log
```

## Current Targets

- `shell/test`: run the Go test suite.
- `shell/lint`: run golangci-lint, parse Checkstyle output, and enforce zero lint issues.
- `shell/build`: build the local `dist/bach` binary.
- `shell/build-release`: build macOS/Linux amd64/arm64 release archives.
- `shell/github-release`: create a GitHub release with the release archives.

## Release Rule

Never create a release without a dry-run first:

```sh
go run ./cmd/bach --var release_version=vX.Y.Z --dry-run run shell/github-release
go run ./cmd/bach --var release_version=vX.Y.Z run shell/github-release
```

The release target pins the GitHub tag to `$BACH_GIT_COMMIT` and uploads all multi-platform archives.

## Notes

- Do not commit `.bach/`, `dist/`, or `.opencode-snitch-off`.
- `shell/lint` requires a golangci-lint v2-compatible binary on `PATH`; `.golangci.yml`
  also enables `golines` at 100 columns and `dupl` duplication checks.
- Prefer `bach reference <topic>` over reading source when asking about supported Bachfile syntax.
- If a Bach target is missing for a repeated operation, add or update the Bachfile instead of documenting another standalone command.

## Reference Docs

`docs/reference.md` is generated. Do not edit it directly for reference changes.

Edit the split source files under:

```text
docs/reference/*.md
```

Then regenerate and test:

```sh
go run ./cmd/bach run shell/docs-generate
go run ./cmd/bach run shell/lint
go run ./cmd/bach run shell/test
```

Use feature/topic names for reference fragments, not implementation phase numbers. For example, prefer `computed-variables.md`, `target-locks.md`, or `completion-contracts.md` over `phase-1.md`.

## Parallel Phase Work

When orchestrating implementation phases in parallel, prefer a dedicated Bachfile such as `Bachfile.batch` over ad hoc shell scripts. Use a `pipeline` target for the sequential merge/test phase, and use normal `depends_on` fan-out for parallel implementation targets.

For non-interactive OpenCode workers, use `opencode run <prompt>`. If the user explicitly approves broad worktree access, pass `--dangerously-skip-permissions`; otherwise agents may be blocked from reading or testing external worktrees.

Phase plans under `plans/` are intentionally ignored. Prompts should pass explicit absolute plan paths instead of relying on glob search from a new worktree.

Each phase worker should:

- Create or reuse a separate worktree outside this checkout.
- Use branch names like `bach/phase-N`.
- Leave the main checkout untouched.
- Start with `go run ./cmd/bach list` and dry-run the likely gates, usually
  `go run ./cmd/bach --dry-run run shell/lint` and
  `go run ./cmd/bach --dry-run run shell/test`.
- After edits, run `go run ./cmd/bach affected` to choose focused tests.
- Run focused tests repeatedly, then `go run ./cmd/bach run shell/lint` and `go run ./cmd/bach run shell/test` before committing.
- Commit only intended phase files on the phase branch and do not merge back.

Merge phases back sequentially. After every merge or conflict resolution, run:

```sh
go run ./cmd/bach affected
```

If conflicts occur, preserve all existing target fields and runner behavior unless the phase explicitly replaces them.
Recent conflict hotspots are:

- `internal/config/config_types.go`: keep metadata fields, profiles, `quiet`, `lock`, `steps`, env blocks,
  and image fields together.
- `internal/runner/runner.go`, `executor.go`, `scheduler.go`, and `logs.go`: preserve pipeline execution,
  profile/env layering, quiet/log-only streaming, target labels, and lock manager plumbing.
- `internal/cli/flags.go`: avoid duplicate option fields or duplicate flag bindings when phases add CLI flags.
- `docs/reference.md` and `docs/agents.md`: merge documentation sections rather than choosing one side.

Avoid shell-local variables like `$out` inside Bach `shell` strings because Bach expands `$NAME` before `/bin/sh` runs. Prefer command pipelines that do not depend on shell-local variable expansion, or move complex logic into a checked-in script/Go test.
