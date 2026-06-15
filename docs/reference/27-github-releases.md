## GitHub Releases

Bachkator does not need a special release target type. Use a shell target with the GitHub CLI:

```hcl
var "github_repo" {
  default = "owner/repo"
}

var "release_version" {
  default = ""
}

shell "github-release" {
  depends_on = [shell.build]
  command = [
    "gh",
    "release",
    "create",
    var.release_version,
    "dist/app",
    "--repo",
    var.github_repo,
    "--target",
    "$BACH_GIT_COMMIT",
    "--title",
    "App ${var.release_version}",
    "--generate-notes",
    "--latest",
  ]
}
```

Run it with:

```sh
bach --var release_version=v0.1.0 run shell/github-release
```

`gh release create <tag>` creates the Git tag when it does not exist. `--target "$BACH_GIT_COMMIT"` pins the tag to the current commit.
