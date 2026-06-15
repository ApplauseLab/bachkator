# Architecture Reviewer Instructions

Review the generated todo app for simple, local-first architecture.

Emit blocking findings if:
- CLI behavior is not separated from test setup.
- Persistence is hard-coded so tests cannot use `TODO_DB`.
- The solution depends on external services or package managers.
- Required files from the plan are missing.

Write the required reviewer report JSON to `$BACH_AGENT_REPORT_PATH`.
