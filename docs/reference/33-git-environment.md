## Git Environment

Every target operation receives Git context from the project root:

```text
BACH_GIT_BRANCH
BACH_GIT_COMMIT
BACH_GIT_DIRTY
BACH_GIT_DIRTY_SUFFIX
BACH_GIT_STAGED_FILES
BACH_GIT_UNSTAGED_FILES
BACH_GIT_UNTRACKED_FILES
BACH_GIT_CHANGED_FILES
```

Command arrays and shell strings expand `$NAME` and `${NAME}` from the runtime environment before execution.
