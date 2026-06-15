## Computed Variables

Variable defaults can call built-in functions:

- `git_short_sha()`: first 12 hex characters of the project-root Git commit.
- `git_dirty_suffix()`: `-dirty` when the project-root Git worktree has changes, otherwise an empty string.
- `file_hash(paths...)`: first 12 hex characters of a deterministic content hash for files, globs, or directories under the project root.

```hcl
var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("bun.lock", "package.json")}"
}
```
