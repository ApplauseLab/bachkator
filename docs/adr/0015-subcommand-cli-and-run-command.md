# Subcommand CLI and Run Command

Bachkator should move to a subcommand-first CLI. Operations such as listing targets and runs should be invoked as `bach list` and `bach runs`, and target execution should use `bach run <target>` rather than direct target invocation or flag-style commands such as `bach -runs`. This reserves top-level CLI words for descriptive commands and help while making target execution explicit.
