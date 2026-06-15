## Target Explain

`bach explain <target>` prints target guidance without running anything. It includes description, when to use the target, cost, risk flags, dependencies, pipeline steps, inputs, outputs, and produced resources. If `<target>` is an alias, explain also prints the alias name, canonical target, and optional deprecation message. If `<target>` is a generated policy node, explain prints `generated: true`, the policy subject, subject workspace/commit scope, and required targets.

```sh
bach explain shell/test-api
bach explain policy.merge@agent.checkout_refactor
```

Use explain before high-cost, remote, destructive, or unfamiliar targets.
