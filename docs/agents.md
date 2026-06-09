# Agent Guide

Bachkator is designed to make agent work predictable and inspectable.

## First Move

Start with discovery:

```sh
bach list
```

This gives the agent the supported operations for the repo. Prefer these target names over inventing package-manager commands.

Use verbose listing when you need cost and risk metadata:

```sh
bach list --verbose
```

Inspect unfamiliar, high-cost, remote, or destructive targets before running them:

```sh
bach explain shell/deploy-staging
```

## Before Running Expensive Work

Use dry-run:

```sh
bach --dry-run run shell/test
```

Dry-run shows dependency order and command strings without executing them. Pipeline targets print their steps in execution order, which catches unsafe deploy sequences before they happen.

## After Editing Code

Ask Bachkator which configured targets are affected by your changes:

```sh
bach affected
bach affected packages/api/src/foo.ts
```

Then run the smallest named target that covers the edit:

```sh
bach run shell/test-api
bach run shell/lint
bach run shell/typecheck
bach run image/all
```

`bach affected` is read-only. It matches Git changed files, or explicit paths, against resolved target inputs including plugin-provided inputs.

If a target has fresh fingerprints, Bachkator skips it. Use `-force` only when cache state is suspect or when validating a side effect intentionally.

Quality-enabled targets can publish reports and gates. After running lint, tests, coverage, or similar gates, inspect stored results when needed:

```sh
bach quality summary
bach quality gates
bach quality findings
```

## When Something Fails

List runs:

```sh
bach runs
```

Open the relevant log file under:

```text
.bach/runs/<run-id>/<target>.log
```

The log survives terminal truncation and gives other agents the same evidence.

## Why This Saves Agents

Bachkator saves agents from common failure modes:

- guessing the wrong test command.
- missing setup dependencies like install, codegen, migrations, or base images.
- rerunning the full suite when only a small target is stale.
- losing failure output when terminal scrollback truncates.
- publishing releases against the wrong commit.
- running deploy steps concurrently when a `pipeline` should fail fast in order.
- hashing large produced directories like `node_modules` instead of modeling them as resources.
- mixing up project roots when working across multiple repositories.

## Safe Release Pattern

Use a named release target with variables:

```sh
bach --var release_version=v0.1.0 --dry-run run shell/github-release
bach --var release_version=v0.1.0 run shell/github-release
```

The target should use `$BACH_GIT_COMMIT` for `gh release create --target` so the tag points at the exact commit the agent built.
