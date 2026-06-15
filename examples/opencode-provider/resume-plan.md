# OpenCode Resume Session Demo

## Goal

Verify that an OpenCode Agent Target resumes the same OpenCode session on a policy-informed improvement attempt.

## Attempt Behavior

- Attempt 1 must create and commit `agent-output/resume-attempt-1.md` only.
- Attempt 1 must not create `agent-output/resume-marker.txt`, so the required policy target fails.
- Attempt 2 must read the feedback bundle path from `BACH_AGENT_FEEDBACK_BUNDLE`.
- Attempt 2 must create and commit `agent-output/resume-marker.txt` and `agent-output/resume-attempt-2.md`.

## Success Criteria

- Bach records two attempt entries in `attempt-history.json`.
- Both entries use the same OpenCode provider session ID.
- Attempt 2 provider session evidence includes `--session <sessionID>` in `executed_command`.
- The final policy passes because `agent-output/resume-marker.txt` exists.
