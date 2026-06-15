## Bachfile Imports

Use top-level `import "path"` declarations to split a Bachfile into local reusable fragments.

```hcl
project "example" {
  default = "shell.test"
}

import "./bach/go.bach"
import "./bach/docs.bach"
```

Import paths are string literals resolved relative to the file containing the import. Imported files share the root project's scope, and their declarations behave as if pasted at the import site.

Imported fragments can contain variables, environment blocks, profiles, inputs, resources, plugins, aliases, policy blocks, quality blocks, and targets. They must not introduce a second effective `project` block.

Each canonical local file is loaded at most once. Re-importing the same file is allowed and deduped, while duplicate declarations from different files remain errors. Import cycles, missing files, and invalid imported files fail project loading with diagnostics that include the importing location or imported file path.

Remote imports, glob imports, environment-expanded paths, registries, and version pinning are not supported.
