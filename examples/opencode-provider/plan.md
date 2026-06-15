# Print OpenCode Provider Environment

## Goal

Verify the first-class OpenCode provider receives Bach's generated prompt, context, report path, workspace, attempt metadata, and project root through environment variables.

## Required Output

- Print each requested `BACH_*` variable to stdout in `KEY=value` format.
- Create `agent-output/opencode-provider-environment.md` in the managed workspace.
- Include a Markdown table with environment variable names and values.
- Commit the new file on the agent branch.
- Write a passing Agent Report JSON to `BACH_AGENT_REPORT_PATH`.

## Constraints

- Do not edit files outside `agent-output/opencode-provider-environment.md` except for the required report file.
- Do not merge back to the main checkout.
- Keep the workspace clean after committing.
