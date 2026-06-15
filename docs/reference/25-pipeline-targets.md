## Pipeline Targets

Pipeline targets run existing targets in a declared sequence. Use them for deploy flows where dependency concurrency is unsafe:

```hcl
pipeline "deploy-staging" {
  timeout = "15m"
  steps = [
    shell.render-staging,
    shell.apply-staging,
    shell.rollout-staging,
    shell.smoke-staging,
  ]
}
```

Pipeline steps may reference shell, image, or pipeline targets. Nested pipelines let you name reusable ordered lanes and compose them into a higher-level program:

```hcl
pipeline "build-lane" {
  steps = [shell.build-a, shell.build-b]
}

pipeline "merge-lane" {
  steps = [shell.merge-a, shell.merge-b]
}

pipeline "delivery-program" {
  steps = [pipeline.build-lane, pipeline.merge-lane, shell.regression]
}
```

Pipeline cycles are rejected during config loading.

Fields:

- `description`: shown by `bach list`.
- `steps`: existing target names to run in order.
- `timeout`: optional Go-style duration that bounds the whole pipeline invocation, including all steps.

`bach run --dry-run pipeline/deploy-staging` prints the pipeline and then each step in execution order. `bach run pipeline/deploy-staging` stops at the first failed step, so later steps do not run. Step targets remain runnable directly for debugging.

Pipeline targets inherit risk metadata from their steps. If any step has `requires_confirmation = true`, running the pipeline requires `--yes`; `--dry-run` still works without confirmation.

Pipeline retry is not enabled by default; use retry on individual shell steps that are safe to repeat.
