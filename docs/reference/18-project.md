## Project

Every Bachfile needs one `project` block:

```hcl
project "example" {
  root    = "."
  default = "shell.test"
}
```

Fields:

- `root`: project working directory. Defaults to the Bachfile directory.
- `default`: target to run when no target is provided.

`project.state` is no longer supported. Omit `backend` to use the default bundled SQLite Backend Provider at `.bach/state.db`.

The omitted `backend` is equivalent to:

```hcl
project "example" {
  root = "."

  backend {
    type    = "stdio"
    command = ["bach", "backend", "sqlite"]
    config = {
      path = ".bach/state.db"
    }
  }
}
```

Backend fields:

- `type`: Backend transport. Phase 1 supports only `stdio`.
- `command`: argv array for the Backend Provider. Phase 1 supports only `["bach", "backend", "sqlite"]`.
- `config`: provider-owned object. The SQLite provider supports `path`, resolved relative to the project root.
