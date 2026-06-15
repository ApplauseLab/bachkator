## Affected Targets

`bach affected [path ...]` suggests the smallest useful configured targets for changed files. It is read-only and never runs targets.

With explicit paths, Bachkator matches those paths against each target's resolved `inputs`, including named inputs and plugin-provided inputs:

```sh
bach affected packages/api/src/foo.ts
```

With no paths, Bachkator uses the current Git staged, unstaged, and untracked files:

```sh
bach affected
```

Output is sorted by target name. Each line includes the target name, the number of matching inputs, and up to the first three matching inputs.
