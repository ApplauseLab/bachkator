## Target Metadata

Shell, image, and pipeline targets support optional guidance metadata:

```hcl
shell "deploy-staging" {
  description           = "Deploy staging API"
  when                  = "after image publish"
  cost                  = "high"
  remote                = true
  destructive           = true
  requires_confirmation = true
}
```

Fields:

- `description`: shown by `bach list` and `bach explain`.
- `when`: guidance for when humans or agents should run the target.
- `cost`: expected cost. Valid values are `low`, `medium`, or `high`.
- `remote`: set to `true` when the target talks to external services.
- `destructive`: set to `true` when the target can delete, overwrite, or irreversibly change state.
- `requires_confirmation`: set to `true` when operators should confirm intent before running.

Risk metadata is inherited through `depends_on` and pipeline `steps`, so aggregate targets show and enforce the risk of the targets they run. Dry-runs are always allowed. Real execution of a target whose effective risk includes `requires_confirmation` must use `-yes`.
