# OpenCode Provider Environment Printer

You are testing Bachkator's first-class OpenCode provider.

Do these actions inside the managed agent workspace:

1. Print the important Bach environment variables to stdout so they appear in the provider log.
2. Create `agent-output/opencode-provider-environment.md` with the same environment snapshot and a short note describing the workspace, target, attempt, and report path.
3. Commit only `agent-output/opencode-provider-environment.md` on the current branch.
4. Write the required Agent Report JSON to `BACH_AGENT_REPORT_PATH` using the schema appended by Bach.

Environment variables to print and record:

- `BACH_AGENT_PROMPT_PATH`
- `BACH_AGENT_CONTEXT_PATH`
- `BACH_AGENT_REPORT_PATH`
- `BACH_AGENT_WORKSPACE`
- `BACH_AGENT_TARGET`
- `BACH_AGENT_ATTEMPT`
- `BACH_AGENT_MAX_ATTEMPTS`
- `BACH_AGENT_FEEDBACK_BUNDLE`
- `BACH_AGENT_ATTEMPT_DIRECTORY`
- `BACH_AGENT_MODE`
- `BACH_AGENT_ROLE`
- `BACH_PROJECT_ROOT`

Use `provider_name` as `opencode`, `provider_type` as `opencode`, and `provider_command` as `["opencode", "run"]` in the report.
