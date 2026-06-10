# Typed Plugin Lifecycle

Bachkator plugins are typed external executables. The plugin `type` determines when Bachkator runs the executable and which stdout contract it must satisfy.

Existing plugin blocks default to `type = "graph"` and keep their current project-load behaviour: they run while loading a Project and may contribute input sets or target dependency/input patches. Quality parser plugins use `type = "quality"` and run only after a target command succeeds, during quality ingestion. They read the declared quality report file and emit normalized metrics/findings JSON on stdout. Bachkator validates that output, writes the State Store, and evaluates gates.

Plugin lifecycle is derived from type in this phase. Bachfiles do not expose arbitrary lifecycle hooks such as pre-run, post-run, init, or stop. This preserves the existing graph plugin contract while allowing target-completion quality parsers without turning the State Store into a public extension API.
