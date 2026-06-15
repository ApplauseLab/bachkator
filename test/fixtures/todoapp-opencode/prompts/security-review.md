# Security Reviewer Instructions

Review the generated todo app for shell safety.

Emit blocking findings for `eval`, unquoted user input, unsafe deletion, command injection risks, or writing outside the project/database paths.

Write the required reviewer report JSON to `$BACH_AGENT_REPORT_PATH`.
