# Agent Guide

Bachkator gives agents a stable repository operation contract. Agents should use the Bach CLI instead of guessing commands from package files, shell history, or prose.

## Standard Workflow

1. Discover supported operations:

```sh
bach list
```

2. Inspect unfamiliar or risky targets:

```sh
bach explain <target>
bach run --dry-run <target>
```

3. After edits, ask Bach for focused validation:

```sh
bach affected
```

4. Run the smallest useful target first:

```sh
bach run shell/test
bach run shell/lint
```

5. Before handoff or commit, run the forced gate so quality reports and gates execute instead of relying on cached status:

```sh
bach run --log-only --force group/gate
```

6. If something fails, inspect run history and logs:

```sh
bach runs list
bach runs inspect <run-id>
bach logs <run-id> --failed --last 80
```

## Repository Rules

- Use `go run ./cmd/bach ...` inside this repository when testing local CLI changes before installing a binary.
- Dry-run expensive, remote, or destructive targets before real execution.
- Use `--yes` only when the target metadata requires confirmation and the action is intentional.
- Prefer `bach reference` and `bach reference <topic>` before guessing Bachfile syntax.
- Keep generated artifacts under declared target outputs or `.bach/`; do not commit `.bach/` or `dist/`.

## Plan And Factory Workflow

Use Plans when implementation should be ledgered before an agent starts work:

```sh
bach plan status plans/example.md
bach plan implement --dry-run plans/example.md
bach plan implement --yes plans/example.md
bach plan review plans/example.md
```

Use Factories when work should enter an unattended delivery lane:

```sh
bach factory submit <factory> --title "Ship billing webhook" --body "Implement webhook and tests"
bach factory list <factory>
bach factory start <factory> --yes
bach factory approve <factory> <work-item-id> --phase plan
bach factory approve <factory> <work-item-id> --phase deploy.production
bach factory inspect <factory> <work-item-id>
```

Factory daemons pause at gated `plan` or `deploy.<name>` phases until approval evidence exists. Inspect the Work Item and run history before approving a phase you did not personally watch complete.

## Commit Workflow

This repository uses semantic commit messages. Install the tracked hook before committing:

```sh
go run ./cmd/bach run shell/install-git-hooks
```

Commit subjects must use `type(scope): subject` or `type: subject`, for example:

```text
feat(cli): add dry-run output
docs: update agent workflow
```

## When To Update Docs

Update docs in the same change when commands, flags, Bachfile syntax, examples, workflows, quality reports, schemas, or agent behavior change. Reference documentation is generated from `docs/reference/*.md`; edit fragments and then run:

```sh
go run ./cmd/bach run shell/docs-generate
```
