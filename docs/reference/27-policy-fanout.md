## Policy Fan-Out

`policy` blocks define generated policy nodes that run required targets against a subject workspace.
The generated node address is `policy.<name>@<subject>`.

```hcl
policy "merge" {
  subject           = "agent.checkout_refactor"
  subject_workspace = ".bach/agents/checkout_refactor"
  subject_commit    = "0123456789abcdef0123456789abcdef01234567"
  required_targets  = [group.gate]
}
```

Fields:

- `subject`: required subject identifier used in the generated address.
- `subject_workspace`: optional workspace where required targets run. Relative paths resolve from the project root.
- `subject_commit`: optional Git commit that the subject workspace must have checked out before policy evaluation starts.
- `required_targets`: required target references such as `group.gate` or `shell.test`.

Generated policy nodes are hidden from the default target list. Use:

```sh
bach list --generated
bach explain policy.merge@agent.checkout_refactor
bach run --dry-run policy.merge@agent.checkout_refactor
bach run policy.merge@agent.checkout_refactor
```

Policy nodes can be run directly. Implementer agents that reference a `policy` invoke a generated
`policy/<name>@agent.<subject>` target after implementation evidence passes, so required targets,
reviewers, policy logs, quality parsing, and applied-policy verdicts are recorded under a visible policy
target run instead of hidden inside the implementation agent. Required targets keep their normal target
identity in run records and logs, but execute with the subject workspace as project root. State and
artifacts for standalone subject-policy fan-out are written under the subject workspace's `.bach`
directory, so cached results from the main checkout cannot satisfy subject policy checks.

When `subject_commit` is set, required targets receive these environment variables:

- `BACH_POLICY_SUBJECT`
- `BACH_POLICY_SUBJECT_COMMIT`
- `BACH_POLICY_NODE`

Standalone subject-policy fan-out writes evaluation JSON to:

```text
<subject_workspace>/.bach/artifacts/<policy-node>.json
```

The JSON includes `policy`, `subject`, `subject_workspace`, `subject_commit`, `status`,
`required_targets`, `findings`, and `created_at`. Failed required targets add
`required_target_error`. If required targets mutate files outside `.bach` and declared target outputs,
the policy fails with `policy-required-target-mutated-workspace`. If `subject_commit` does not match
the subject workspace HEAD, the policy fails with `policy-subject-commit-mismatch`.

Quality-enabled policy runs, including generated policy targets invoked by implementer agents, also write
subject-keyed applied-policy verdict artifacts in the project root:

```text
.bach/artifacts/policies/<run-id>/<sanitized-target>.json
```

Sanitization replaces `/`, `:`, and spaces with `-`.

Merge agents consume the latest matching applied-policy artifact only when the artifact verdict passed, its
`subject_workspace` matches the merge subject workspace, its `subject_commit` matches the subject
workspace HEAD, its `policy_target` names the generated policy target for the subject's current
policy, and that exact policy target succeeded in the recorded run.
