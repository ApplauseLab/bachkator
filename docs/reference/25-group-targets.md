## Group Targets

Group targets name an unordered collection of existing targets. Use a group when several
targets should run as one logical step but may execute in parallel subject to dependencies,
locks, cache state, and preflights:

```hcl
group "ci" {
  targets = [shell.lint, shell.test, shell.build]
}
```

Running `bach run group/ci` walks one deduplicated execution graph. Shared dependencies and
shared group members run at most once per run.

Groups can be used as pipeline steps:

```hcl
group "checks" {
  targets = [shell.lint, shell.test]
}

pipeline "release" {
  steps = [group.checks, shell.build-release, shell.github-release]
}
```

In that example, `shell.build-release` cannot start until all targets in `group.checks` and
their dependency closures complete. Inside the group, normal dependency and lock rules decide
what can run concurrently.

Fields:

- `description`: shown by `bach list`.
- `targets`: existing target references to include in the unordered group.
- `timeout`: optional Go-style duration that bounds the whole group invocation, including all
  members.
- `lock`: optional lock held for the whole group invocation.

Groups are scoped completion targets. They do not run their own command and do not have
independent cache inputs or outputs; cache and fingerprints remain attached to the member
targets. Group-level `timeout` and `lock` wrap member execution, while member-level runtime
fields still apply to each individual target.
