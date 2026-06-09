# Private State Store

Bachkator treats `.bach/state.db` as a private local **State Store**, not as a public SQL interface. The supported contract is the CLI-visible domain model: target fingerprints, run history, target run records, artifacts, quality reports, metrics, findings, and quality gate results. Keeping SQLite table shape private lets Bachkator migrate persistence internals while still giving agents stable commands for inspection, diagnosis, and reporting.
