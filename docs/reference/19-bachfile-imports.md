## Bachfile Imports

Root Bachfiles can import local Bachfile fragments to keep reusable target packs in separate files:

```hcl
import "./bach/go.bach"
import "./bach/docs.bach"

project "example" {}
```

Import paths must be string literals and are resolved relative to the file that contains the import declaration. Imported files are local files only; HTTP, Git, registry, glob, environment-expanded, and computed import paths are not supported.

Imported files share the same project scope as the root Bachfile. Targets, aliases, policies, variables, inputs, resources, plugins, profiles, and quality blocks from imported files participate in the same validation rules as root declarations, so duplicate declarations are errors. The root Bachfile must own the `project` block; imported files must not declare one.

The same canonical file can be imported more than once and is loaded only once. Import cycles fail during project loading with a diagnostic that includes the cycle path.
