# Bach Report Supplemental Evidence Demo

## Goal

Demonstrate that an OpenCode provider can call `bach report` inside the managed workspace to produce a supplemental `bach.agent_report.v1` quality artifact.

## Required Output

- Run `bach report init`, `bach report finding`, `bach report metric`, and `bach report status success` against one explicit report path under `BACH_AGENT_ATTEMPT_DIRECTORY`.
- Create `agent-output/bach-report-demo.md` with the commands and generated report contents.
- Commit the summary Markdown file on the agent branch.
- Write the normal Agent Target completion report to `BACH_AGENT_REPORT_PATH`.

## Constraints

- Do not use `BACH_AGENT_REPORT_PATH` as the `bach report` destination.
- Do not edit files outside `agent-output/bach-report-demo.md` except for generated run artifacts.
- Keep the workspace clean after committing.
