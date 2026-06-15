# Todo App Plan

Build a small but real todo application in this repository.

## Required Product

- Implement `todo.sh`, a POSIX shell CLI.
- Supported commands:
- `./todo.sh add "task text"` appends an open todo.
- `./todo.sh list` prints todos in insertion order.
- `./todo.sh complete N` marks the Nth todo as complete.
- Store data under `.todo/todos.txt` by default.
- Respect `TODO_DB` when set so tests can use an isolated database.

## Required Tests

- Implement `test/todo.bats` with Bats tests for add, list, complete, invalid commands, and isolated `TODO_DB` use.
- Implement or update `scripts/run-bats.sh` so `bach run shell.test` validates the actual app with Bats.
- The test target must write `.bach/artifacts/todo-bats.passed` only after the Bats tests pass.

## Required Docs

- Update `README.md` with usage examples for all supported commands.
- Mention `TODO_DB` and the Bats validation flow.

## Quality Bar

- Keep the app dependency-free and runnable on macOS with `/bin/sh`.
- Do not use `eval`, unsafe shell expansion, or unchecked destructive paths.
- Commit the implementation on the configured agent branch.
- Write the required Bach Agent Report to `$BACH_AGENT_REPORT_PATH` after committing.
