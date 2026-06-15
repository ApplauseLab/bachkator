# Internal App Composition Root

Bachkator should use `internal/app` as the **Composition Root** that assembles built-in subsystem registries, target-kind handlers, quality handlers, config loaders, runner dependencies, state access, and CLI command adapters. `cmd/bach` should remain a tiny executable entry point, and `internal/cli` should focus on command presentation and the public CLI contract. Centralizing production wiring in `internal/app` keeps initialization explicit and lets tests assemble smaller subsystem compositions without package-level side effects.
