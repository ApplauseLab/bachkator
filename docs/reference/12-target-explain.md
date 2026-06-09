## Target Explain

`bach explain <target>` prints target guidance without running anything. It includes description, when to use the target, cost, risk flags, dependencies, pipeline steps, inputs, outputs, and produced resources. If `<target>` is an alias, explain also prints the alias name, canonical target, and optional deprecation message.

```sh
bach explain shell/test-api
```

Use explain before high-cost, remote, destructive, or unfamiliar targets.
