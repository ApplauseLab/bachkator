# Bachkator

Bachkator is a build-system control plane for repositories where coding agents need explicit, inspectable project operations. This repository is documented as a single product context; configuration, execution, state, quality, and agent guidance share one language.

## Language

**Project**:
A repository workspace described by one Bachfile and executed from a resolved project root.
_Avoid_: App, package, repo config

**Bachfile**:
The HCL configuration file that declares a project's supported operations and their operational model.
_Avoid_: Build script, task file

**CLI Contract**:
The supported user and agent interface made up of commands, flags, Bachfile syntax, reference docs, and command output semantics.
_Avoid_: Go API, internal package contract

**CLI Command Adapter**:
An internal CLI-layer adapter that exposes a domain capability as a Cobra command or subcommand.
_Avoid_: Domain command, public extension command

**Run Command**:
The CLI subcommand that executes a target by typed or unambiguous target address.
_Avoid_: Direct target invocation

**Target**:
A named operation that Bachkator can inspect, plan, run, cache, and record.
_Avoid_: Task, job, script

**Target Facet**:
A coherent group of target fields that describes one concern such as metadata, runtime, cache, command, image, pipeline, quality, or completion contract.
_Avoid_: Subtarget, target subtype

**Target Address**:
The typed canonical identity of a target, written as `shell.test`, `pipeline.release`, or `image.app`.
_Avoid_: Slash target name, untyped target name

**Target Kind Handler**:
An internal handler registered for a target kind to provide kind-specific validation, explanation, planning, or execution behavior over the shared target model.
_Avoid_: Target subclass, public target plugin

**Shell Target**:
A target that runs a command or shell string directly on the host.
_Avoid_: Command target, script target

**Image Target**:
A target that generates and runs an OCI image build command.
_Avoid_: Docker target, container target

**Pipeline Target**:
A target that runs existing targets in a declared sequence.
_Avoid_: Workflow, deploy script

**Dependency Graph**:
The prerequisite graph formed by target dependencies and used for parallel scheduling.
_Avoid_: Pipeline, workflow

**Input**:
A named set of files or paths used as cache evidence and affected-target evidence.
_Avoid_: Source, dependency

**Output**:
A concrete file path expected after a target runs and used as cache evidence or report input.
_Avoid_: Resource, artifact

**Resource**:
A logical capability or produced artifact identity used as dependency evidence without hashing large concrete directories.
_Avoid_: Virtual file, artifact

**Plugin**:
An external executable that extends the loaded project graph by contributing input sets or target dependency/input patches.
_Avoid_: Runner extension, target extension

**Internal Extension Point**:
A compile-time seam where Bachkator's own packages can register target kinds, config blocks, quality handlers, or CLI commands without exposing a public extension API.
_Avoid_: Public plugin API, runtime extension

**Subsystem Registry**:
An internal registry owned by one subsystem for one family of extension points.
_Avoid_: Global registry, service locator

**Composition Root**:
The internal application assembly point that wires built-in subsystem registries and production dependencies together.
_Avoid_: Package init wiring, CLI bootstrap logic

**Run**:
One invocation of Bachkator against a requested target, including status, target records, logs, and artifacts.
_Avoid_: Build, session

**Run Plan**:
A runner-owned description of the Targets involved in a Run, including deterministic order, dependency edges, pipeline edges, effective risk, required tool declarations, preflight declarations, and dependency fingerprint input names. A Run Plan describes what Bachkator is preparing to run; it does not execute commands, probe the host, write logs, or mutate the State Store.
_Avoid_: Scheduler graph, dry-run JSON, execution plan

**Run Session**:
A runner-owned in-memory execution coordinator for one Run, containing mutable execution state such as the Run record, Run Plan, target fingerprints, dirty target records, locks, and synchronized output/log access. A Run Session coordinates execution while the Run is in progress; the persisted result remains the Run in the State Store.
_Avoid_: Run context, execution bag, state manager

**State Store**:
The private local persistence mechanism that keeps target fingerprints, run records, artifacts, quality reports, and quality gate results.
_Avoid_: Public database, cache file

**Quality Report**:
A parsed report associated with a target run and normalized into metrics or findings.
_Avoid_: Test report, coverage file

**Quality Handler**:
An internal quality subsystem adapter that parses a quality report format or evaluates a quality gate.
_Avoid_: Target-kind quality logic, runner parser

## Relationships

- A **Project** has exactly one **Bachfile**.
- The **CLI Contract** is the public product boundary for Bachkator.
- A **Bachfile** declares zero or more **Inputs**, **Resources**, and **Targets**.
- A **Target** may depend on other **Targets**, forming the **Dependency Graph**.
- A **Target** has one canonical **Target Address**.
- A **Target** may consume **Inputs** or **Resources**.
- A **Target** may declare **Outputs** as concrete file evidence.
- A **Target** may produce **Resources** as logical dependency evidence.
- A **Plugin** may add **Inputs** and dependency/input patches before target validation, fingerprinting, scheduling, and affected-target matching.
- An **Internal Extension Point** enables parallel feature work inside Bachkator while preserving the **CLI Contract** as the public product boundary.
- A **Subsystem Registry** owns one extension family; target kinds, config blocks, quality handlers, and CLI commands should not share one global registry.
- The **Composition Root** wires built-in **Subsystem Registries**, handlers, and production dependencies without making them public extension APIs.
- A **CLI Command Adapter** belongs to `internal/cli`; domain packages should not own the Cobra command tree.
- The **Run Command** executes **Targets**; top-level CLI words are reserved for subcommands.
- A **Run Plan** is built from the loaded **Project** before scheduling, dry-run rendering, risk reporting, tool/preflight reporting, and dependency fingerprint resolution.
- A **Run Session** owns mutable per-Run execution state and coordinates Run/TargetRun lifecycle, logs, output synchronization, fingerprints, locks, summaries, and completion persistence.
- A **Target Kind Handler** may add kind-specific behavior without owning common target metadata, risk, cache, runtime, or run-record semantics.

## Architecture Direction

- Architecture migration should happen before broad parallel feature work, in behavior-preserving subsystem slices.
- `internal/model` should own shared domain types such as **Project**, **Target**, **Input**, **Resource**, target specs, and risk metadata.
- `internal/config` should own Bachfile loading, decoding, validation, variables, profiles, env overlays, and config block registration.
- `internal/target` should own **Target Kind Handler** registration and kind-specific behavior over shared target specs.
- `internal/runner` should own planning, scheduling, locks, target execution, dry-run plans, logs, completion contracts, cache decisions, and run summaries.
- `internal/state` should remain the **State Store** implementation boundary.
- `internal/quality` should own report parsing, metric/finding normalization, quality gate evaluation, and quality handler registration.
- `internal/plugin` or `internal/graph` should own graph-load plugin integration and affected-target graph evidence.
- `internal/cli` should own the **CLI Contract** and CLI subcommand registration.
- `internal/app` should be the **Composition Root** for production wiring.
- CLI extensibility should use **CLI Command Adapters** composed by `internal/cli` rather than Cobra dependencies in core domain packages.
- The dependency direction should flow from CLI/composition into subsystem packages, with subsystem packages depending on `internal/model` rather than each other where possible.
- Target-kind extensibility should use **Target Kind Handlers** in `internal/target` over the shared model rather than separate target subclasses.
- A **Target** is the canonical operation abstraction; target kinds are expressed through **Target Facets**, not separate domain concepts.
- The **Dependency Graph** expresses prerequisites and allows parallel execution when safe.
- A **Pipeline Target** orders existing **Targets** when sequence is part of the domain requirement.
- The **Run Plan** must keep **Dependency Graph** edges distinct from **Pipeline Target** step edges because dependency edges may run in parallel while pipeline edges preserve declared sequence.
- The **Run Session** consumes a **Run Plan** but does not change planning semantics.
- A **Run** records one requested **Target** and the **Targets** executed to satisfy it.
- A **Quality Report** belongs to one **Target** within one **Run**.
- A **Quality Handler** belongs to the quality subsystem, not to a target kind.
- A report-format **Quality Handler** should expose a parser interface instead of requiring callers to edit a central parsing switch.
- The **State Store** persists **Runs**, target fingerprints, artifacts, and **Quality Reports**, but its table schema is not part of Bachkator's public contract.

## Improvement Sequence

- First extract `internal/model` so subsystem packages share domain types without importing runner or config code.
- Then extract independent subsystems that already have clear seams, especially `internal/quality` and the `internal/state` wrapper boundary.
- Then extract `internal/runner` around planning, scheduling, execution, cache decisions, logs, and completion contracts.
- Then extract `internal/config` around Bachfile decoding, validation, environment layering, plugins, and config registries.
- Then introduce `internal/app` composition and `internal/cli` command adapter composition.
- Only after these slices are stable should broad target-kind, config-family, quality, or CLI-command feature work proceed in parallel.

## Example Dialogue

> **Dev:** "Should the agent run the test command from the README?"
> **Domain expert:** "No. Ask Bachkator for the supported **Targets**, inspect the plan, then run the named **Target**. The **Run** will keep logs and cache evidence for later agents."

## Flagged Ambiguities

- "task", "job", and "script" should resolve to **Target** when referring to an operation Bachkator can list, plan, run, cache, or record.
- "workflow" should resolve to **Pipeline Target** only when target execution order is the point; otherwise prefer **Target** or **Run**.
- "dependency order" should not imply execution sequence; use **Pipeline Target** when ordering is required.
- ".bach/state.db" should resolve to **State Store**; avoid describing its SQLite schema as a supported user or plugin interface.
- "plugin" should not imply execution hooks, state access, or new target kinds; those would be separate future extension points.
- "artifact" is overloaded; prefer **Output** for configured file evidence, **Resource** for logical dependency evidence, and run artifact when referring to recorded files from a **Run**.
- "API" should mean the **CLI Contract** unless a future decision explicitly creates a public Go API.
- "registry" should mean an **Internal Extension Point** unless a future ADR explicitly accepts an external/runtime extension API.
- "central registry" is not the intended architecture; prefer **Subsystem Registry** plus composition wiring.
- "target kind" should not imply a separate target domain type; use **Target Kind Handler** when talking about kind-specific behavior.
- "target name" should resolve to **Target Address** when referring to canonical identity.
- "run target" should resolve to **Run Command**, written as `bach run <target>`.
- "quality parser" or "quality gate" should resolve to **Quality Handler**, not runner behavior or target-kind behavior.
- "command registry" should mean internal **CLI Command Adapter** composition, not a public CLI plugin system.
- "app wiring" should resolve to **Composition Root** work in `internal/app`, not package `init` side effects.
