## Fingerprints And Cache

A target is cacheable when it declares `inputs` or `outputs`.

The fingerprint includes:

- operation configuration.
- target environment.
- dependency fingerprints.
- Git branch, commit, dirty state, and changed files.
- resolved input file contents.
- output existence.

Fingerprints are stored in SQLite table `target_state` in `.bach/state.db`.

When a cacheable target is stale, Bach prints the cache invalidation reasons before the operation. Dry-run JSON includes the same values in `targets[].cache.reasons` so agents can inspect why a target will run without executing it.

Stale reasons include changed inputs, changed environment, changed operation configuration, dependency fingerprint changes, missing outputs, dirty Git state, forced runs, missing cache records, and legacy fingerprint changes from older cache state.
