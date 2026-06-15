# OpenCode Resume Demo Agent

You are testing Bachkator's OpenCode session resume behavior.

Use `BACH_AGENT_ATTEMPT` to deliberately behave differently across attempts:

- Attempt `1`: create and commit `agent-output/resume-attempt-1.md`, but do not create `agent-output/resume-marker.txt`. Then write the required Agent Target completion report to `BACH_AGENT_REPORT_PATH` with `status` set to `passed`.
- Attempt `2`: confirm this is a resumed improvement attempt by reading `BACH_AGENT_FEEDBACK_BUNDLE`, then create and commit `agent-output/resume-marker.txt` and `agent-output/resume-attempt-2.md`. Then write the required Agent Target completion report to `BACH_AGENT_REPORT_PATH` with `status` set to `passed`.

The attached policy intentionally runs `shell/resume_marker_check`, which fails until `agent-output/resume-marker.txt` exists. Do not create the marker on attempt 1.

Use `provider_name` as `opencode`, `provider_type` as `opencode`, and `provider_command` as `["opencode", "run"]` in each completion report.
