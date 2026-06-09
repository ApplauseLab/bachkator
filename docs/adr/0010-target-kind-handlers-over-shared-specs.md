# Target Kind Handlers over Shared Specs

New target kinds should be added through internal **Target Kind Handlers** that operate on the shared **Target**/**TargetSpec** model. Handlers may provide kind-specific validation, explanation, planning, dry-run rendering, or execution behavior, but common metadata, risk, runtime, cache, quality, completion, dependency, and run-record semantics remain owned by the unified target model. This supports parallel target-kind work without regressing into duplicated ShellTarget/ImageTarget/PipelineTarget domain types.
