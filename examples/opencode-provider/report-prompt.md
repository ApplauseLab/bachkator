# Bach Report Demo Agent

You are testing Bachkator's `bach report` helper from inside a first-class OpenCode Agent Target.

Do these actions inside the managed agent workspace:

1. Print the exact `go run ./cmd/bach report ...` commands before running them.
2. Set `REPORT_PATH="$BACH_AGENT_ATTEMPT_DIRECTORY/bach-report-demo-agent-report-v1.json"`.
3. Run these commands from the workspace root:
   - `go run ./cmd/bach report init --path "$REPORT_PATH" --role bach-report-demo --name opencode --summary "Bach report demo started"`
   - `go run ./cmd/bach report finding --path "$REPORT_PATH" --kind demo --severity info --rule bach-report-demo --message "OpenCode wrote supplemental Bach report evidence"`
   - `go run ./cmd/bach report metric --path "$REPORT_PATH" --name demo.summary.items.count --value 3 --unit count --scope agent`
   - `go run ./cmd/bach report status success --path "$REPORT_PATH" --summary "Bach report demo completed"`
4. Create `agent-output/bach-report-demo.md` summarizing the commands, the report path, and the generated JSON contents.
5. Commit only `agent-output/bach-report-demo.md` on the current branch.
6. Write the required Agent Target completion report JSON to `BACH_AGENT_REPORT_PATH` using the schema appended by Bach.

`bach report` creates supplemental quality evidence only. You must still write the Agent Target completion report to `BACH_AGENT_REPORT_PATH`.

Use `provider_name` as `opencode`, `provider_type` as `opencode`, and `provider_command` as `["opencode", "run"]` in the completion report.
