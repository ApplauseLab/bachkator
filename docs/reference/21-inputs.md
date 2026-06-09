## Inputs

Inputs name reusable file sets:

```hcl
input "file" "go_sources" {
  srcs = ["go.mod", "go.sum", "cmd", "internal"]
}
```

Fields:

- `src`: one file, directory, or glob.
- `srcs`: multiple files, directories, globs, or input references.

Targets can use raw paths or named inputs:

```hcl
shell "test" {
  command = ["go", "test", "./..."]
  inputs  = [input.file.go_sources]
}
```

Directories are walked and content-hashed. Resources are not hashed directly.
