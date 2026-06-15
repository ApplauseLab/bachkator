## Variables

Variables are declared with `var` blocks and referenced as `var.name`:

```hcl
var "release_version" {
  default = ""
}

shell "release" {
  command = ["gh", "release", "create", var.release_version]
}
```

Variable defaults can derive values with built-in computed functions:

```hcl
var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("bun.lock", "package.json")}"
}
```

Computed functions:

- `git_short_sha()`: first 12 hex characters of the current project-root Git commit.
- `git_dirty_suffix()`: `-dirty` when the project-root Git worktree has changes, otherwise an empty string.
- `file_hash(paths...)`: first 12 hex characters of a deterministic content hash for files, globs, or directories under the project root.

Value precedence:

- `--var release_version=v0.1.0`
- `BACH_VAR_release_version=v0.1.0`
- `BACH_VAR_RELEASE_VERSION=v0.1.0`
- `default` in the `var` block
- empty string when no default is set
