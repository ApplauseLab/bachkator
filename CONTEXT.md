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
A typed external executable declared in a Bachfile. The plugin type determines its lifecycle and stdout contract.
_Avoid_: Go plugin, state-store extension, arbitrary lifecycle hook

**Graph Plugin**:
A `type = "graph"` Plugin that runs while loading a Project and may contribute input sets or target dependency/input patches.
_Avoid_: Runtime hook, target execution plugin

**Quality Plugin**:
A `type = "quality"` Plugin that runs during quality parsing after a target command succeeds, reads a report file, and emits normalized quality metrics/findings JSON.
_Avoid_: Quality gate, State Store writer

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

**Backend**:
A Project-level state and evidence provider selected by Bachfile configuration. The Backend owns durable persistence for factory queues, approvals, run/evidence indexes, normalized findings, and failure patterns while preserving the public CLI/JSON contracts. The first Backend is the bundled SQLite stdio provider backed by `.bach/state.db`; it implements the same Backend Provider protocol that future external Backends must satisfy.
_Avoid_: Managed control plane, public database API, target runner, artifact directory

**Evidence Store**:
The trusted local boundary for Bach-owned artifact paths, safe project/workspace path resolution, private evidence writes, and evidence redaction rules.
_Avoid_: Generic filesystem helper, provider workspace cache

**Query Module**:
An internal read-only module that assembles CLI-ready DTOs from private persistence and domain records without exposing State Store schema to command adapters.
_Avoid_: CLI formatter, public State Store API

**Quality Report**:
A parsed report associated with a target run and normalized into metrics or findings.
_Avoid_: Test report, coverage file

**Quality Handler**:
An internal quality subsystem adapter that parses a quality report format or evaluates a quality gate.
_Avoid_: Target-kind quality logic, runner parser

**Agent Target**:
A Target that runs an external coding agent provider against a declared prompt, workspace, git policy, and optional acceptance policy.
_Avoid_: Agent job, agent task, external workflow

**Provider**:
A Bachfile declaration for the executable or service used by an Agent Target, such as OpenCode.
_Avoid_: Agent provider, model config, runtime hook

**Prompt**:
The user-facing instruction or prompt file supplied to an Agent Target or reusable Agent Pack.
_Avoid_: Script, command, task body

**Policy**:
A named acceptance contract that can require targets, reviewers, quality gates, and findings thresholds before work is ready to ship.
_Avoid_: Quality parser, merge script, checklist

**Factory**:
A top-level Bachfile daemon configuration that defines how a Project ships software unattended. A Factory owns trigger intake, work-item queueing, and the workflow policy for planning, implementation, merge, staging deploy, production deploy, and related unattended delivery steps. A Factory is started explicitly by a CLI command such as `bach factory start`; it coordinates Targets and plan-first workflows rather than replacing them.
_Avoid_: Target, pipeline, background shell script, managed control plane

**Factory Workflow**:
A named unattended delivery lane inside a Factory, such as `ship`, `hotfix`, `dependency_update`, or `nightly_maintenance`. A Factory Workflow declares the ordered delivery phases and the policies for when a queued work item may move from planning to implementation, merge, deploy, verification, and completion. Review remains part of implementer Policy evaluation rather than a separate Factory Workflow phase. Factory Workflows may reference Agent Templates, Policies, Plans, and Targets, but they are not themselves Targets.
_Avoid_: Pipeline, target group, CI workflow, OpenWorkflow workflow

**Factory Work Item**:
A queued, normalized unit of unattended delivery intake created by a Factory trigger, such as a manual submission or GitHub Issue. Factory Work Items are persisted before planning starts. The planning phase consumes Factory Work Items and creates or updates durable Plans and workstreams.
_Avoid_: Plan, Target, Issue, Task, prompt

**Trigger Provider**:
An executable provider that supplies normalized Factory Work Items to a Factory through a narrow public protocol. Bach owns the trigger-provider protocol, routing, dedupe, queueing, and daemon behavior; concrete network integrations such as GitHub Issues, Discord, browser intake, or Atelier-owned intake may be private products.
_Avoid_: Built-in trigger, webhook handler, source plugin, factory workflow

**Integration Provider**:
An external provider that connects Bach or a Factory to another system without expanding Bach core. Integration Providers are the umbrella family for trigger intake, notifications, bidirectional commands, external approvals, deployment coordination, and managed backend sync. Concrete providers are external integrations while Bach owns the supported provider protocols and local execution semantics.
_Avoid_: Generic plugin, in-process extension, public Bach internals

**Provider SDK**:
A public Go package that helps external provider implementations speak a supported Bach provider protocol. Provider SDKs may expose protocol DTOs, JSON-RPC framing helpers, error types, provider harnesses, and conformance helpers, but they do not expose Bach's internal execution, configuration, runner, or storage implementation as a public API. The provider protocol and schemas remain the authoritative cross-language contract.
_Avoid_: Public Bach internals, plugin API, runner API

**Improvement Loop**:
The bounded Agent Target behavior that reruns implementation with findings and policy evidence until the policy passes or attempts are exhausted.
_Avoid_: Retry, rerun, loop script

**Agent Report**:
A lightweight structured output from an Agent Target that records execution evidence such as status, changed files, git evidence, and handoff notes. Policies own report parsing, finding aggregation, Quality Plugins, quality gates, and final acceptance verdicts.
_Avoid_: Log, transcript, chat summary

**Managed Control Plane**:
An external product layer that governs Bach runs across projects, organizations, provider credentials, prompt packs, policies, workspaces, audit trails, and usage data. Bachkator exposes CLI contracts and JSON evidence for this layer; it does not own organization state.
_Avoid_: Bach cloud, public Go API, central State Store

## Relationships

- A **Project** has exactly one **Bachfile**.
- The **CLI Contract** is the public product boundary for Bachkator.
- A **Bachfile** declares zero or more **Inputs**, **Resources**, and **Targets**.
- A **Target** may depend on other **Targets**, forming the **Dependency Graph**.
- A **Target** has one canonical **Target Address**.
- A **Target** may consume **Inputs** or **Resources**.
- A **Target** may declare **Outputs** as concrete file evidence.
- A **Target** may produce **Resources** as logical dependency evidence.
- A **Graph Plugin** may add **Inputs** and dependency/input patches before target validation, fingerprinting, scheduling, and affected-target matching.
- A **Quality Plugin** may parse a target-produced report file into **Quality Report** metrics/findings during quality ingestion.
- An **Agent Target** is still a **Target** and participates in planning, dependencies, run records, logs, quality reports, and quality gates.
- A **Provider** supplies execution mechanics for an **Agent Target**, while Bach owns semantics such as workspace safety, git policy, improvement loop, report evidence, required-target fan-out, and policy fan-out.
- A **Policy** names acceptance criteria above individual **Quality Reports** and may aggregate required targets, reviewer agents, and quality gates.
- An **Improvement Loop** uses **Policy** evidence and findings to decide whether an **Agent Target** should keep working or stop.
- An **Agent Report** provides stable agent execution evidence; custom finding types, severities, metrics, parsing, and gate semantics belong to **Policy**, **Quality Plugins**, and **Quality Handlers** rather than **Agent Target** behavior.
- The **Evidence Store** protects Bach-owned local evidence from provider-writable workspaces and keeps raw provider details out of normal progress output.
- A **Query Module** may read private persistence through internal APIs and return DTOs; CLI command adapters own presentation and output contracts.
- A **Managed Control Plane** may ingest Bach CLI JSON, **Agent Reports**, **Quality Reports**, and **Run** evidence, but the local **State Store** remains private to Bachkator.
- An **Integration Provider** connects Bach to external systems; concrete provider implementations remain outside Bach core, but supported protocols and local execution semantics belong to Bach.
- A **Provider SDK** supports external provider implementations without making Bach internals public.
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
- `internal/backend` should own the **Backend** client facade, provider lifecycle, protocol mapping, and built-in provider implementations.
- `internal/state` is legacy local SQLite persistence being absorbed behind the **Backend** provider boundary.
- `internal/evidence` should own trusted local evidence paths, safe writes, and provider/workspace evidence boundaries.
- `internal/quality` should own report parsing, metric/finding normalization, quality gate evaluation, and quality handler registration.
- `internal/query` should own read-only DTO assembly for CLI adapters and future evidence surfaces.
- `internal/graph` should own graph-load plugin integration, structured explain/provenance data, risk aggregation, and affected-target graph evidence.
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
- Agent orchestration should be expressed through first-class **Agent Targets** and runner-owned orchestration helpers rather than macro-generated shell targets when Bach needs to understand workspaces, provider sessions, git evidence, policies, reviewer agents, and improvement loops.
- Private or enterprise governance should live in a **Managed Control Plane** that consumes Bach's CLI Contract and JSON evidence instead of depending on Bach's internal Go packages or State Store schema.

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
- "plugin" should resolve to a typed external executable. Check the plugin type before assuming lifecycle: graph plugins run at Project load; quality plugins run during quality ingestion.
- "private plugin" should resolve to **Integration Provider** when referring to enterprise triggers, notifications, bidirectional commands, approvals, deployments, or managed backend sync.
- "artifact" is overloaded; prefer **Output** for configured file evidence, **Resource** for logical dependency evidence, and run artifact when referring to recorded files from a **Run**.
- "API" should usually mean the **CLI Contract**. Use **Provider SDK** only for public Go packages that support external provider protocols, not for Bach internals.
- "registry" should mean an **Internal Extension Point** unless a future ADR explicitly accepts an external/runtime extension API.
- "central registry" is not the intended architecture; prefer **Subsystem Registry** plus composition wiring.
- "target kind" should not imply a separate target domain type; use **Target Kind Handler** when talking about kind-specific behavior.
- "target name" should resolve to **Target Address** when referring to canonical identity.
- "run target" should resolve to **Run Command**, written as `bach run <target>`.
- "quality parser" or "quality gate" should resolve to **Quality Handler**, not runner behavior or target-kind behavior.
- "command registry" should mean internal **CLI Command Adapter** composition, not a public CLI plugin system.
- "app wiring" should resolve to **Composition Root** work in `internal/app`, not package `init` side effects.
- "agent workflow" should resolve to **Agent Target** plus **Policy** when Bach must inspect, run, cache, gate, or record it.
- "agent provider" should usually resolve to **Provider** in user-facing Bachfile language.
- "retry an agent" should resolve to **Improvement Loop** only when new findings or policy feedback are supplied; plain `retry` means rerun the same Target attempt.
- "Bach cloud" or "enterprise Bach" should resolve to **Managed Control Plane** unless a future decision creates hosted Bach State Store semantics.
