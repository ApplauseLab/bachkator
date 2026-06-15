# Contributing

Thanks for helping improve Bachkator. The project is built around explicit,
inspectable repository operations, so contributions should use the Bach targets
checked into this repo instead of ad hoc commands.

## Development workflow

Start by discovering the available Targets:

```sh
go run ./cmd/bach list
```

Before running expensive or side-effecting work, inspect the Run Plan:

```sh
go run ./cmd/bach run --dry-run <target>
```

For normal changes, run the relevant focused checks first, then the standard
gates before opening a pull request:

```sh
go run ./cmd/bach affected
go run ./cmd/bach run shell/lint
go run ./cmd/bach run shell/test
```

If a run fails, inspect the recorded run history and logs:

```sh
go run ./cmd/bach runs
```

Logs live under `.bach/runs/<run-id>/`.

## Commit messages

Use semantic commit subject lines:

```text
type(scope): subject
type: subject
```

Allowed types are `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, and `revert`. Scopes are optional and should identify
the affected subsystem when useful.

Install the tracked Git hook before committing:

```sh
go run ./cmd/bach run --dry-run shell/install-git-hooks
go run ./cmd/bach run shell/install-git-hooks
```

The hook is a `commit-msg` hook because Git does not expose the final commit
message to `pre-commit`. To validate a message file manually:

```sh
go run ./cmd/bach --var commit_msg_file=/path/to/message run shell/commit-msg
```

## Documentation

`docs/reference.md` is generated. Do not edit it directly. Update the split
source files under `docs/reference/*.md`, then regenerate and verify:

```sh
go run ./cmd/bach run shell/docs-generate
go run ./cmd/bach run shell/lint
go run ./cmd/bach run shell/test
```

Use feature or topic names for reference fragments rather than implementation
phase numbers.

## Pull request checklist

- Keep changes focused and explain the user-visible behavior they affect.
- Add or update tests when behavior changes.
- Update documentation when commands, configuration, examples, or workflows
  change.
- Do not commit `.bach/`, `dist/`, or local agent/session files.
- Run `go run ./cmd/bach run shell/lint` and
  `go run ./cmd/bach run shell/test` before requesting review.

## Releases

Only maintainers should create releases. Never create a release without a
dry-run first:

```sh
go run ./cmd/bach --var release_version=vX.Y.Z run --dry-run shell/github-release
go run ./cmd/bach --var release_version=vX.Y.Z run shell/github-release
```
