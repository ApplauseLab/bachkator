# DAG Execution with Explicit Pipelines

Bachkator treats `depends_on` as a prerequisite **Dependency Graph** that may run in parallel, bounded by target readiness, configured parallelism, and target locks. Ordered execution is represented explicitly with **Pipeline Targets**, whose steps run in declaration order and stop at the first failure. This keeps normal build/test graphs fast while making deploy, release, and merge lanes inspectably sequential for agents.
