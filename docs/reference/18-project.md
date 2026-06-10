## Project

Every Bachfile needs one `project` block:

```hcl
project "example" {
  root    = "."
  default = "shell.test"
  state   = ".bach/state.db"
}
```

Fields:

- `root`: project working directory. Defaults to the Bachfile directory.
- `default`: target to run when no target is provided.
- `state`: SQLite state path. Defaults to `.bach/state.db` under `root`.
