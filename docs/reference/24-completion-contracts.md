## Completion Contracts

Completion contracts are generic execution evidence. They do not replace `outputs`, which remain cache evidence. Contracts are evaluated only after the target operation exits successfully. When both `success_when` and `fail_when` are omitted for a shell target, Bach relies on the command exit code exactly as before.

```hcl
shell "deploy" {
  command = ["./deploy.sh"]

  fail_when {
    output_contains = "ROLLBACK"
  }

  success_when {
    output_contains = "Deployment complete"
  }

  success_when {
    command = ["./scripts/smoke.sh"]
  }
}
```

Supported completion checks are:

- `output_contains`: match text in the target log after operation execution.
- `file_exists`: require a file path under the project root to exist.
- `command`: run a verification command from the target workdir with the target environment.

Each `success_when` or `fail_when` block must set exactly one check field.
