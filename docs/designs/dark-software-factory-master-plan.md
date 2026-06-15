# Dark Software Factory Master Plan

Status: planning

## Purpose

Bach should become a local-first dark software factory: a daemon-configured delivery system that can accept work, plan it, implement it with agents, merge it, deploy it, verify it, pause for approval, and export durable evidence for managed control planes such as Atelier.

The factory is not a replacement for Targets, Policies, Runs, or Plans. It is the unattended delivery layer above them.

## Core Decisions

### Factory

A `factory` block is a top-level Bachfile daemon configuration.

It defines how a Project ships software unattended:

- trigger intake.
- work-item queueing.
- workflow routing.
- plan creation.
- implementation through plan-first workflows.
- merge.
- deploy and verify phases.
- approval gates.
- evidence and finding export through the Project Backend.

The daemon is started explicitly:

```sh
bach factory start sldc
```

`factory` is not a Target. Factory Workflows coordinate Targets and plan-first workflows rather than replacing them.

### Backend

`project.backend` replaces `project.state` as the public project persistence concept.

```hcl
project "my-repo" {
  root = "."

  backend {
    type = "stdio"
    command = ["bach", "backend", "sqlite"]
    config = {
      path = ".bach/state.db"
    }
  }
}
```

Defaults:

```hcl
project "my-repo" {
  root = "."
}
```

is equivalent to:

```hcl
project "my-repo" {
  root = "."

  backend {
    type = "stdio"
    command = ["bach", "backend", "sqlite"]
    config = {
      path = ".bach/state.db"
    }
  }
}
```

The default Backend is implicit. Most Bachfiles do not need a `backend` block; omitting it resolves to the bundled SQLite stdio Backend Provider with `path = ".bach/state.db"`.

`bach backend sqlite` is a supported low-level provider entrypoint, not a Target. It should be documented in help/reference and visible under `bach backend --help` with wording that it is normally launched from Project Backend config, but it is not shown by `bach list`. Users may run it manually for debugging, but it is protocol-only: it starts the JSON-RPC stdio provider, waits for protocol messages on stdin, and does not provide an interactive shell.

`project.state` is removed with no compatibility shim.

Phase 1 config rules:

- `backend {}` is allowed only inside `project`.
- A Project has exactly one Backend after default resolution.
- Multiple explicit `backend` blocks in one Project are a hard validation error with source locations.
- Omitted Backend config resolves to the bundled SQLite stdio Backend and should be visible in dry-run/config inspection output as resolved config.
- Any legacy `state =` or `state {}` syntax is a hard validation error with a direct migration message to `backend`.
- Phase 1 accepts `type = "stdio"` only with `command = ["bach", "backend", "sqlite"]`; arbitrary external Backend commands wait until conformance tests exist.
- Explicit `backend {}` blocks must include `type = "stdio"` and `command = [...]`; only omitted Backend config gets the implicit default.
- Provider config uses assignment syntax only: `config = { ... }`. Phase 1 does not support a nested `config {}` block form.
- Resolved Backend config output shows non-secret values such as SQLite `path` and redacts keys matching sensitive patterns.

The existing `.bach/state.db` remains the first Backend implementation. SQLite is exposed through the same stdio JSON-RPC Backend Provider protocol as future Backends, but the provider is bundled as `bach backend sqlite` and configured with the database path. The internal implementation should move from `internal/state` language toward `internal/backend`, with a clean extension interface.

Only `backend.type = "stdio"` is accepted initially. Concrete Backends are selected by `command`; the bundled default command is `["bach", "backend", "sqlite"]`. Other Backend Provider commands should fail until the provider protocol and command invocation are implemented.

Backends use JSON-RPC over stdio for the Bach-to-provider boundary. An external Backend provider may speak HTTP, gRPC, or another service protocol upstream, but Bach core should not embed those managed-service assumptions. Managed products must not read private SQLite tables directly.

The Backend Provider protocol is the stable Backend boundary from day one, not an experimental internal seam. The bundled SQLite provider proves the same public provider contract that external Backend providers must implement.

Phase 1 creates public provider SDK packages because external and private provider integrations need to implement the stable protocol from separate repositories. `pkg/backendprotocol` exposes protocol DTOs, error types, provider server/client helpers, and conformance helpers for `bach.backend.v1`; it does not expose Bach internals or the full internal Backend interface. `pkg/jsonrpcstdio` exposes reusable stdio JSON-RPC framing with `Content-Length` messages for Backend now and future provider protocols later. The JSON-RPC spec and schemas remain authoritative, while the Go SDK is a supported convenience for Go providers.

The public SDK import paths are `github.com/applauselab/bachkator/pkg/backendprotocol` and `github.com/applauselab/bachkator/pkg/jsonrpcstdio`. `pkg/backendprotocol` exposes Go structs/constants and links to schema docs; schema JSON lives under `docs/schemas`. It should include a Go conformance test harness/helpers that external Backend Providers can use.

Phase 1 may use short-lived Backend Provider sessions for one-shot CLI writes. Later daemon and steady-state runtimes should keep the Backend Provider process long-lived for each Bach process or daemon session; `bach factory start` keeps the provider alive for the daemon lifetime so queue, phase, run, approval, and finding calls share one provider session.

For the bundled SQLite provider, runtime should spawn the current Bach executable with `backend sqlite` rather than resolving `bach` from `PATH`, even though resolved config displays `command = ["bach", "backend", "sqlite"]`. This keeps `go run ./cmd/bach` and local binaries from accidentally launching a different Bach version.

Backend Provider processes receive a sanitized inherited environment plus Bach-managed additions. Provider startup has a default initialization timeout; command preflight fails if `backend.initialize` does not complete in time. Bach shuts providers down with `backend.shutdown`, then closes stdin and kills the process only if graceful shutdown times out.

The Backend Provider protocol version is `bach.backend.v1`. Bach starts the provider process and sends `backend.initialize` with the provider config before any normal domain RPCs. The provider validates config, initializes storage, runs required migrations, and returns provider identity, protocol version, and capabilities. Normal Backend RPCs are rejected until initialization succeeds.

`backend.initialize` params include the requested protocol version, Project name, Project root, and opaque provider `config`. Bach core does not send the full decoded Project model to the provider.

The SQLite provider owns SQLite schema and migrations. `bach backend sqlite` receives `config.path` during `backend.initialize`, validates it, and migrates `.bach/state.db` before serving normal domain RPCs. Bach core owns Backend protocol version and capability checks, but it must not know or migrate SQLite tables directly.

SQLite `config.path` is resolved relative to the Project root, and absolute paths are rejected in Phase 1. The default path is `.bach/state.db`, but Phase 1 may allow another safe project-root-relative path contained within the Project root. The SQLite provider creates the database parent directory after safe path validation. SQLite uses WAL mode and a reasonable busy timeout by default for daemon/read concurrency.

The Backend v1 SQLite migration may destructively clear legacy local state. It can clear or recreate old run, cache, fingerprint, and related state tables without backup; `.bach` is treated as disposable local state for this migration. New runs rebuild cache/fingerprint data under the Backend v1 schema.

Backend Providers must expose initialization/capability data before normal domain RPCs. The initialization response should identify provider name/version, Backend protocol version, supported capabilities such as runs, quality reports, findings, factory queue, approvals, and evidence references, plus any provider limits needed for safe execution. Bach fails early when required capabilities or protocol versions are missing.

Backend capability checks happen during command preflight, not plain static config loading. Config load validates Bachfile syntax and required fields without starting providers. Commands that need persistence start the Backend Provider, run `backend.initialize`, and verify the capabilities required by that command.

Commands that require Backend persistence fail fast when the Backend Provider is unavailable, lacks required capabilities, or returns a required write error. `bach list` and other static inspection commands should not need the Backend, but `bach run`, `bach factory submit`, `bach factory start`, and `bach factory approve` require authoritative persistence and must not continue in degraded mode by default.

Backend provider `config` is mostly opaque to Bach core. The config loader validates generic shape such as `type`, `command`, and that `config` is an object. Concrete providers validate their own config during startup or preflight; for example, the SQLite provider validates its `path`.

Provider config should not contain raw secrets. Bachfile provider config may contain non-secret settings; credentials should come from environment variables, provider-owned credential stores, or future explicit secret-reference mechanisms with redaction rules.

Provider commands are argv arrays only, not shell strings. For example, use `command = ["bach", "backend", "sqlite"]` so argument boundaries are explicit and provider execution avoids shell interpolation.

Provider stdio streams are strict: stdin and stdout are reserved for JSON-RPC protocol messages only, and stderr is used for provider diagnostics/logs. Providers must not print banners, progress, or human-readable output to stdout.

JSON-RPC stdio messages use LSP-style `Content-Length` framing so multi-line JSON messages are robust. Phase 1 Bach runtime sends Backend RPCs sequentially and may open short-lived provider sessions for write operations. Providers may be internally concurrent, but they do not need to support multiple in-flight Bach requests for the first protocol.

Provider domain errors use standard JSON-RPC error responses with Bach domain error codes and structured details in `error.data`. Stderr is reserved for structured JSON-lines diagnostics with fields such as level, message, and metadata; Bach captures and redacts these logs separately from protocol responses.

Backend Provider methods should be coarse-grained Bach domain operations, not generic key-value, SQL, or table-row operations. Method names use domain namespaces: `backend.initialize` for setup, then methods such as `runs.create`, `runs.startTarget`, `runs.finishTarget`, `evidence.recordRef`, `quality.recordReport`, `findings.recordObservation`, `factory.enqueueWorkItem`, `factory.claimWorkItem`, `factory.recordApproval`, and `factory.transitionPhase`. The protocol must not expose SQLite schema as the contract.

Current Backend capabilities are runs, target runs, evidence references, quality reports sufficient for current behavior, normalized findings storage, and the Factory queue. Approval, daemon lease, and phase-transition capabilities remain deferred until later Factory phases implement them.

The internal Backend client facade uses domain subclients for readability and future growth: Runs, Evidence, Quality, Findings, and later Factory-related subclients. Internal domain models remain separate from public provider protocol DTOs; Phase 1 maps between internal models and `pkg/backendprotocol` DTOs at the Backend boundary.

Phase 1 moves current run, target-run, artifact/evidence-reference, and quality-report persistence writes toward the internal Backend client facade while adding stdio provider protocol coverage for the bundled SQLite Backend. Findings storage is implemented with DTOs, SQLite storage, Backend RPCs, schemas, and tests even if no current producer emits normalized findings yet.

All normalized findings go through Backend storage. There should be no parallel ephemeral finding path for CLI, export, dashboard, policy, quality, reviewer, or future analysis findings; producers record observations through Backend storage before those findings are surfaced elsewhere.

Phase 1 method names use lifecycle/domain verbs: `backend.initialize`, `backend.shutdown`, `runs.create`, `runs.startTarget`, `runs.finishTarget`, `runs.finish`, `runs.get`, `runs.list`, `evidence.recordRef`, `evidence.listRefs`, `quality.recordReport`, `quality.recordReports`, `findings.recordObservation`, `findings.listCurrent`, `findings.listEvents`, and `findings.get`. Target-run lifecycle methods stay under the `runs.*` namespace because target runs belong to a Run.

If current quality parsing already produces finding-like records with source, severity, message, and location data, Phase 1 should persist them as normalized findings. If a current quality report only produces aggregate metrics, Phase 1 should leave producer behavior unchanged and rely on storage/RPC tests for findings until a later producer slice.

`runs.create` requires the Bach-generated UUIDv7 Run ID, requested Target Address, start timestamp, Project/root context, command arguments/options relevant to replaying the CLI request, and metadata. `runs.finish` records the final Run record together with target-run records, changed target fingerprints, and evidence references. Later provider hardening can add compare-and-set transition fields where needed. Backend write failure during a command that requires persistence fails the run immediately, marks failure when possible, and returns a Backend persistence error rather than continuing best-effort.

Phase 1 keeps existing run log file content and format behavior-compatible except for the run directory path switching to the UUIDv7 Run ID. Logs remain files/evidence references, not Backend blobs.

Finding observations require source reference, severity, category or rule, message, optional project-relative location, optional suggested fingerprint, and metadata. Finding severities are `info`, `warning`, `error`, and `critical`. Locations use project-relative paths with optional start/end line and column; finding DTOs must not expose absolute local paths.

Evidence records require Bach-generated UUIDv7 evidence ID, kind, optional content hash, source Run/Target/Plan context, and metadata. Bach-owned evidence records are Backend records. Large run logs and provider event streams may remain artifact files referenced by Backend evidence records until a later blob-storage contract exists.

Phase 1 Backend domain error codes are `invalid_request`, `not_initialized`, `unsupported_capability`, `not_found`, `conflict`, `validation_failed`, and `internal`. These codes are carried in JSON-RPC `error.data` with structured details.

Backend Provider reads should return Bach domain DTOs, not raw storage records. Read operations should be shaped around supported concepts such as runs, target runs, factory work items, findings, quality reports, approvals, and evidence references. Provider-specific metadata may be carried only in explicit metadata fields and must not become the primary output shape.

Backend Provider methods should be synchronous request/response domain RPCs first. Small evidence records and summaries belong in the Backend database. Large logs, artifacts, and provider raw event streams may remain Evidence Store files or external evidence objects referenced by Backend records until an explicit blob-storage contract exists.

Bach core and the Evidence Store handle path safety, workspace containment, and redaction before recording evidence references through the Backend. Backend Providers store accepted references and metadata; external providers may validate defensively, but they are not the primary trust boundary for local file safety.

Bach core computes public IDs, dedupe keys, canonical fingerprints, and intended state transitions. Backend Providers enforce available uniqueness and idempotency constraints atomically. Later daemon/factory transition methods should add compare-and-set style transition constraints so repeated trigger polls, daemon parallelism, and finding observations cannot corrupt authoritative state.

Normalized findings are first-class Backend records, separate from their evidence sources. Quality reports, agent reports, policy gates, test/lint parsers, reviewer agents, and future analysis can all produce findings. Findings need stable identity, source references, severity, category, message, location, fingerprint, lifecycle status, timestamps, and metadata so dashboards and improvement loops can reason across evidence sources.

Bach owns canonical finding fingerprinting. Finding producers may provide a suggested fingerprint, but Bach normalizes and selects the canonical fingerprint used for dedupe, lifecycle tracking, dashboards, and improvement loops.

Findings should preserve event history and expose current state. Phase 1 records observations and supports current-state queries for `open` and `resolved` findings. Later lifecycle work can add suppressions and reopen events. The protocol exposes domain operations such as recording observations, resolving findings, listing current findings, and listing finding events without exposing storage tables.

Finding records and events have UUIDv7 IDs, while canonical fingerprints key dedupe and current-state lifecycle. The ID identifies a specific persisted record or event; the fingerprint identifies the recurring issue across observations.

There is one authoritative Backend for a Project. The bundled SQLite provider is the first authoritative Backend and stores records in the current `.bach/state.db`, including runs, factory queues, approvals, evidence indexes, normalized findings, and failure patterns through the same Backend Provider protocol that future external Backends must satisfy. External Backends are not modeled as secondary mirrors in the first interface design; swapping Backend providers should preserve the same supported contracts.

Atelier can later provide an enterprise Backend implementation that stores or mirrors Bach results, evidence, approvals, findings, factory queue state, and timelines. That Backend is persistence and sync infrastructure. The Atelier managed dashboard and governance product is the Managed Control Plane built on top of supported Bach Backend and evidence contracts.

### Factory Workflows

A Factory can contain multiple named Factory Workflows.

```hcl
factory "sldc" {
  workflow "ship" {}
  workflow "hotfix" {}
}
```

If a Factory has more than one workflow, triggers must route each work item to exactly one workflow. If a Factory has one workflow, routing may default to that workflow.

Workflow routing happens before final enqueue. Bach computes the canonical dedupe key from the factory, selected workflow, and source identity when the same source can legitimately feed different workflows. If a source routes differently later, Bach must not silently move the existing Work Item to another workflow; it requires an explicit new Work Item, replan, or operator decision.

Factory Workflow phases for v1:

- `plan`
- `implement`
- `merge`
- `deploy "<environment>"`
- `verify "<environment>"`

Review is not a Factory phase. Review remains part of implementer Policy evaluation.

Completion is derived from the required phases passing; there is no `complete` block.

### Plan-First Foundation

Plan-first workflows must ship before the Factory daemon is useful.

ADR `docs/adr/0020-plan-execution-unit-and-backend-evidence.md` locks the v1 direction: one accepted Plan materializes one implementer Agent Target, and Bach-owned Plan ledger/evidence records are stored through the Project Backend database.

The plan-first layer provides:

- Plan metadata inference and optional frontmatter parsing.
- Plan-level dependency graph loading.
- agent templates.
- generated targets.
- Backend Plan ledger/evidence records.
- batch dry-runs.
- implementation execution.
- review queues.
- idempotency and stale-work detection.

Factory Workflows call into this layer rather than inventing separate implementation semantics.

Each accepted Plan materializes one implementer Agent Target by default. Plan dependencies control execution order and allow independent Plans to run concurrently within configured limits. If the work is too large for one implementer, split it into multiple Plan files and express dependencies between Plans.

The merge phase uses a merge Agent Template and merge Policy to combine Plan implementation branches, resolve conflicts, preserve work, and produce merge evidence. Review remains inside implementation and merge Policy evaluation; `review` is not a separate Factory Workflow phase.

### Agent Templates

`agent_template` blocks define reusable execution machinery. They are not runnable Targets.

```hcl
agent_template "planner" {
  provider = provider.opencode
  role     = "planner"
  prompt   = prompt.factory_planner

  workspace {
    mode = "readonly"
  }
}

agent_template "implementer" {
  provider = provider.opencode
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    mode = "clone"
    path = ".bach/agents/${factory.workflow}/${work_item.id}/${plan.id}"
  }

  git {
    branch = "bach/${factory.workflow}/${work_item.id}/${plan.id}"
    commit = "required"
  }
}
```

Plans and factories reference templates. Bach materializes temporary concrete Agent Targets from templates plus plan/work-item context. Materialized template Targets participate in normal Run graph, logging, policy, and evidence machinery rather than using a separate Factory-specific agent executor. Materialized target addresses should be deterministic and inspectable, including factory, work item, attempt, phase, and Plan context.

Template materialization is visible in dry-run. Dry-run output should show generated target addresses, workspaces, policies, and dependent Targets so humans and agents can inspect the concrete execution plan before work starts.

The Factory `plan` phase uses a planner template and requires approval or policy acceptance before implementation begins. Planner templates default to read-only repository access while writing Plan output only to controlled Plan/evidence paths. Implementer templates default to isolated clone/worktree execution with explicit git branch and commit policy.

When `plan.requires_approval = true`, human approval happens after built-in Plan validation and any configured plan Policy passes. The planner Agent Template produces a Plan, Bach validates the Plan structure, optional `policy.plan_ready` validates additional readiness, the Work Item enters `waiting_approval` for the `plan` phase, a human approves, Bach snapshots the accepted Plan into evidence, and implementation begins.

`plan.requires_approval` defaults to `true` for Factory Workflows. Plan approval is the default control point before unattended implementation begins; workflows that support fully unattended planning must opt out explicitly if that option is allowed.

Workflows may explicitly set `plan.requires_approval = false` to allow fully unattended planning, but built-in Plan validation still runs and any configured plan Policy still runs. The opt-out must be visible in the Factory Workflow config rather than implied by omission.

`plan.policy` is optional. Basic Plan validation is mandatory even when no Policy is configured; it should check default metadata inference, optional frontmatter parsing, unique selected Plan IDs, Plan dependency graph validity, syntactically valid references where possible, and safe plan metadata paths.

`plan.template` is required for generated planning. A Work Item submitted with an explicit Plan may skip planner execution, but it still uses the workflow's plan phase for built-in validation, optional Policy, approval, and evidence snapshotting. If no Plan is supplied and no planner template is configured, the Work Item cannot enter implementation.

Generated Plans are editable before approval. The planner writes a working Plan, Bach validates it, any configured Policy runs, and the Work Item waits for plan approval. A human may edit the Plan before running `bach factory approve <factory> <work-item-id> --phase plan`; Bach reruns built-in validation and any configured plan Policy, snapshots the approved version into evidence, and only then starts implementation.

Plan approval is recorded against the immutable Plan evidence snapshot and content hash. The approval record includes the work item id, attempt id, phase `plan`, approved Plan evidence reference, Plan hash, approver identity when available, approval time, and optional reason. Implementation uses the approved Plan snapshot, not the mutable working Plan file.

Implementation agents receive the approved Plan snapshot as authoritative context. The mutable working Plan path may be included as supplemental context when useful, but implementation scope is determined by the approved snapshot.

Editing a Plan after approval requires a new approval on the same attempt only if implementation has not started. Once implementation starts, material Plan changes require retrying the Work Item with a new attempt or creating a new Work Item, while prior approval and implementation evidence remains intact.

### Triggers

The first trigger is built-in manual intake.

```hcl
factory "sldc" {
  triggers {
    manual {}
  }
}
```

Manual submissions enqueue directly and do not require the daemon to be running:

```sh
bach factory submit sldc --workflow ship --title "Add billing webhook" --body-file request.md
```

Manual submit does not dedupe implicitly. Every manual submit creates a new Work Item unless the user supplies an explicit dedupe key. If an explicit dedupe key already exists, Bach returns the existing Work Item ID and status rather than creating a duplicate. `bach factory submit` prints a human-readable summary by default, including Work Item ID, workflow, lifecycle, and next action.

Factory commands should support stable `--json` output from day one for agents and automation. Human-readable output remains the default, but commands such as submit, list, inspect, approve, cancel, retry, and replan should expose machine-readable JSON where useful.

Factory CLI JSON outputs include a top-level `schema_version` and are redacted by default using the same safety rules as human-readable output. Raw evidence access, if ever needed, should be a separate explicit surface rather than the default `--json` behavior.

The primary Work Item inspection command is `bach factory list <factory>`, with filters for status, workflow, priority, and other queue metadata.

`bach factory list <factory>` shows active operational work by default: pending, planning, active, waiting approval, blocked, and failed Work Items. Completed and cancelled Work Items are hidden unless requested with filters or flags.

Factory daemon inspection uses `bach factory status <factory>` for daemon lease, heartbeat, active work, paused or blocked counts, and high-level queue health. Work Item detail uses `bach factory inspect <factory> <work-item-id>` for full Work Item state, attempts, phases, approvals, evidence references, findings, and related Runs.

`bach factory inspect` shows log and evidence references plus summaries by default, not full large logs inline. `bach factory status <factory>` works even when no daemon is running; it reads Backend lease and queue health and reports the daemon as inactive.

Manual trigger intake, direct submit, the local queue, and daemon processing are OSS core requirements. Bach must be able to accept, queue, plan, implement, and complete manual work locally without Atelier, GitHub, Discord, ServiceNow, Slack, or any other enterprise Integration Provider.

Network triggers should be pluggable so Bach core is not polluted with GitHub, Discord, browser, or Atelier-specific logic.

Trigger providers use one public minimal JSON-RPC-over-stdio protocol rather than per-provider CLI contracts. Provider implementations can remain private moat.

Trigger Providers supply source identity such as source type, source id, and optional source revision. Bach computes the canonical dedupe key and owns dedupe semantics. Providers may suggest a dedupe key, but routing and queue idempotency use Bach's canonical key.

The Trigger Provider protocol should be stable when it ships. Manual trigger intake does not require the provider protocol, so the trigger-provider contract can land after Backend and local Factory foundations, but it should not be documented as experimental once introduced.

Only Trigger Provider and Backend protocols are part of the first factory roadmap. Notification Providers, Command Providers, Approval Providers, and Deployment Providers are planned Integration Provider families, but their protocols should not be designed until the local Factory semantics are proven.

Notification Providers and Command Providers remain separate planned families. Notification Providers are one-way event sinks for Bach events such as work item creation, waiting approval, run failure, and deploy completion. Command Providers translate external commands such as approve, retry, cancel, or status into Bach-owned actions after validation. Slack, Discord, or similar systems may support both roles later, but the provider roles should not be merged.

Core Bach owns:

- trigger-provider protocol.
- plugin process invocation.
- config passing and redaction.
- normalized item validation.
- routing.
- dedupe.
- enqueue.
- ack/nack.

Providers own:

- network APIs.
- auth flows.
- rate limits.
- cursors.
- provider payload parsing.

Integration Providers adapt external systems into Bach-owned semantics. They must not secretly mutate Factory queue state, skip policies, write private Backend tables, or own workflow phase transitions. Bach owns validation, routing, dedupe, queue transitions, phase transitions, policy gates, approvals, run records, and evidence.

Initial private providers can include:

- GitHub Issues.
- Discord.
- browser or Atelier intake.

### Factory Queue

All trigger intake creates Factory Work Items first.

```text
trigger/manual submit -> Factory Work Item -> plan phase -> accepted Plan -> one implementer -> merge -> deploy/verify
```

The Factory queue lives in the Backend from day one. Since the first Backend is SQLite and only the Bach factory daemon writes queue transitions, queue tables should be added to the existing `.bach/state.db`.

Manual submit inserts a pending item and exits. The daemon later claims it.

Factory Work Item intake is snapshotted into evidence. Bach stores the normalized title, body, labels, metadata, source reference, source revision or cursor when available, content hash, and evidence snapshot so planning starts from a stable request even if the external source changes later.

External source revisions update the same pending Work Item before planning starts. If the source changes while the Work Item is waiting for plan approval, Bach marks the item stale or needing replan rather than silently changing the plan context. If the source changes after implementation starts, the change requires a retry/new attempt or a new Work Item depending on whether the request changed meaningfully.

Stale-source handling depends on Work Item lifecycle. Pending items may update automatically. Planning or waiting-approval items become stale and approval is blocked until replan or explicit accept-stale behavior exists. Active items stop at the next safe phase boundary and require retry or replan decision. Completed items do not mutate; later source changes create a new Work Item or are ignored according to dedupe/source policy.

V1 should not include an `accept-stale` command. Stale Work Items should use replan or retry paths so the approved Plan and execution evidence match the current intake snapshot.

Bach core generates Factory Work Item IDs because they are public CLI identities used by commands such as `bach factory approve sldc <work-item-id>`. Backend Providers store work item IDs but do not choose them, so public identity remains stable across Backend implementations.

Bach core should generate public IDs broadly, including Run IDs, Factory Work Item IDs, exposed Plan IDs, exposed Approval IDs, and finding identities or canonical fingerprints. Backend Providers are authoritative storage for these IDs, not the identity policy owner.

All Bach-generated public IDs use UUIDv7, including Run IDs, Factory Work Item IDs, Work Item attempt IDs, exposed Plan IDs, Approval IDs, finding record/event IDs, and other exposed records. IDs remain opaque to users and providers beyond being stable public identifiers, but UUIDv7 provides time-sortable identity across Bach evidence and CLI surfaces.

The Backend migration does not preserve legacy non-UUIDv7 run history. Existing non-UUIDv7 Run records in local `.bach/state.db` may be dropped during the Backend/SQLite provider migration, and legacy IDs are not exposed in new JSON contracts.

UUIDv7 generation should live in one shared internal ID package used by runs, factory work items, attempts, approvals, findings, plans, and other public IDs. Tests should inject deterministic time/random sources. New run log and evidence directories use the UUIDv7 Run ID directly, such as `.bach/runs/<uuidv7>/...`.

`internal/id` should wrap `github.com/google/uuid` for UUIDv7 generation if that dependency supports the required behavior, keeping Bach's ID policy and deterministic test hooks in one package.

Factory Work Items should expose both a top-level lifecycle and a current phase. The lifecycle gives operators a simple state such as `pending`, `planning`, `active`, `waiting_approval`, `blocked`, `completed`, `failed`, or `cancelled`. The current phase records exact workflow position such as `plan`, `implement`, `merge`, `deploy.staging`, `verify.staging`, `deploy.production`, or `verify.production`.

Factory Work Items support simple priority from day one. Priority defaults to `normal` and allowed values are `low`, `normal`, `high`, and `urgent`. Manual submit may pass `--priority`, Trigger Providers may supply priority hints, and queue ordering uses priority before created time. Priority never bypasses locks, approvals, or policies.

Priority does not preempt active work in v1. Urgent or high priority affects the next queue claim/order only; operators can cancel, block, or replan lower-priority work if manual intervention is needed.

Factory Work Items support labels/tags as metadata. Manual submit may pass labels, and Trigger Providers may supply labels from source systems. Labels can be used for routing before the Work Item is created and for filtering, dashboards, or later policy selection, but they do not override the workflow after routing selects it.

Work Item labels are mutable metadata with event history. Label changes are recorded, but they do not silently change the selected workflow. If labels change in a way that implies a different workflow, Bach should require an explicit replan/new Work Item decision rather than rerouting in place.

Factory cancellation is Bach-owned and durable. A command such as `bach factory cancel <factory> <work-item-id> --reason "duplicate"` records cancellation time, actor when available, reason, and final lifecycle state in the Backend. Pending or waiting items cancel immediately; active phases receive a cancellation request and stop according to runner/agent cancellation support. Cancelled items are retained as evidence rather than deleted.

Factory retry creates a new attempt on the same Work Item by default. A command such as `bach factory retry <factory> <work-item-id> --from-phase implement` preserves previous failure evidence, records a new attempt, and resumes from the failed phase or an explicitly selected earlier phase. A new Work Item should be created only when the intake request changes meaningfully.

Retry is allowed only for failed, blocked, or cancelled Work Items by default. Completed Work Items require a new Work Item rather than retrying in place, so completed deploys or other side effects are not duplicated accidentally.

Factory replan is distinct from retry. A command such as `bach factory replan <factory> <work-item-id>` creates a new Work Item attempt starting at the `plan` phase, preserving prior Plan, approval, and execution evidence while producing and approving a new Plan from the current intake snapshot.

Completed Work Items cannot be replanned. Follow-up changes after completion require a new Work Item so the meaning of the completed delivery remains stable.

`retry --from-phase plan` should be rejected. Operators use `replan` for plan-phase restarts and `retry` for rerunning implementation or later workflow phases.

Work Item attempts are first-class Backend records. Each attempt records attempt id, work item id, started and finished timestamps, start phase, end phase/status, associated Run IDs, associated accepted Plan ID and ledger/evidence record IDs, failure reason, and retry or cancellation linkage.

Plans are created as part of a Work Item attempt and linked back to the stable Work Item. A retry may create a new Plan if the repository, request, or execution context changed. Prior Plans remain attached to prior attempts so planning and execution evidence is not overwritten.

Each Work Item attempt has exactly one accepted Plan in v1. A planner may discuss alternatives inside that Plan, but approval and implementation operate on one accepted Plan snapshot.

Manual submission may provide an explicit Plan with a command such as `bach factory submit <factory> --workflow ship --plan plans/fix-billing.md`. The Work Item is still created, the attempt links to the supplied Plan, and the Factory validates the Plan. The planning phase records acceptance of the supplied Plan rather than disappearing.

Submitted Plans use the same workflow resolution rules as other submit paths: a single-workflow Factory may infer the workflow, while multi-workflow Factories require `--workflow` or trigger routing. Plan frontmatter may not override the selected workflow; Bach validates the Plan against the workflow chosen by submit or trigger routing.

Plans accepted by a Work Item attempt should be snapshotted into Bach evidence. The original path may be stored as metadata, but the attempt references the immutable evidence snapshot so later file edits or deletion do not rewrite planning history.

Factory-generated Plans may live as working files under `plans/` for human review, editing, and agent handoff, but the evidence snapshot is authoritative. On acceptance or attempt start, Bach snapshots the Plan into evidence and links the attempt to that immutable copy while retaining the working path as metadata.

### Daemon Parallelism

`bach factory start <factory>` defaults to one active work item.

`bach factory start <factory>` runs in the foreground by default and streams concise daemon status in the terminal. Service managers or external supervisors can wrap it when background operation is needed.

Future daemon preflight may use `bach factory start <factory> --dry-run` without acquiring the long-lived daemon lease or processing work. Preflight should validate workflow configuration, Backend capabilities, provider command shape, required targets, locks, and obvious approval/deploy wiring issues.

The intended operating model is one active Factory daemon per Project/Factory. `bach factory start <factory>` acquires a Backend-held daemon lease with heartbeat/expiry. If another active daemon already holds the lease, the new daemon exits with a clear error. Lease expiry supports crash recovery without designing normal multi-daemon scheduling semantics.

Factory daemon lifecycle events are recorded both as structured Backend records and as detailed local evidence/log files. Backend records support status, audit, and dashboard queries; files preserve detailed diagnostics without forcing large logs through Backend RPCs.

Factory daemon parallelism controls how many Factory Work Items may be active inside the one daemon. Plan concurrency is a separate plan-first execution concern across selected Plans. Merge and deploy locks still serialize dangerous phases regardless of work-item parallelism.

```sh
bach factory start sldc
bach factory start sldc --jobs 4
```

Parallelism can be increased, but merge and deploy safety are enforced through locks.

Factory merge and deploy locks are Backend-held domain locks. Locks include owner identity, heartbeat, and expiry so crashes can recover without unsafe concurrent merge or deploy execution. Backend lock records are the authoritative coordination boundary for Factory phases.

Recommended lock identities:

```text
factory:sldc:merge
factory:sldc:deploy:staging
factory:sldc:deploy:production
```

Daemon crash recovery resumes from the last recorded safe phase boundary. Bach should not attempt to resume midway through an individual target, agent step, merge, or deploy action unless a later phase-specific contract explicitly supports safe exact-step resume.

Factory phase transitions are compare-and-set Backend operations. Each transition supplies the expected current lifecycle/phase/attempt so stale daemons, duplicate commands, or delayed approvals cannot overwrite newer state.

Failed phases do not auto-retry in v1. A failed or blocked Work Item requires explicit `retry` or `replan` unless a future policy adds configured retry behavior.

### Approvals

Factory approval is a durable Backend record.

Production deploy approval uses this command shape:

```sh
bach factory approve sldc <work-item-id> --phase deploy.production
```

Plan approval uses the same phase-based approval command:

```sh
bach factory approve sldc <work-item-id> --phase plan
```

Approval uses canonical phase strings validated against the Factory Workflow definition. Unknown phases fail, phases that exist but do not require or await approval fail with a clear message, and approval records store the canonical phase string such as `plan` or `deploy.production`.

When a workflow reaches a gated phase:

```text
phase reaches requires_approval
work item enters waiting_approval
daemon skips or pauses the item
operator records approval
daemon resumes from the approved phase
```

Approval evidence includes:

- factory.
- work item id.
- phase.
- approved time.
- approver identity when available.
- optional reason.

Approval reasons are optional in v1. Approver identity is resolved best-effort from available CLI, user, environment, or git identity context. Approval records should preserve the resolved identity source when available.

Duplicate approval commands are idempotent when they refer to the same Work Item, attempt, and waiting phase; Bach returns the existing approval rather than recording conflicting evidence. Approvals are not revocable in v1. Before a gated phase starts, operators can cancel or replan; after a phase starts, the approval remains durable evidence.

External approval systems submit approval evidence into Bach; they do not decide workflow advancement directly. Slack, ServiceNow, GitHub, Discord, or Atelier may be where a human approves, but Bach validates the approval against the waiting factory, work item, and phase, stores the durable Backend approval record, and advances the workflow.

Future Approval Providers submit evidence through the same Bach approval path and validation rules as CLI approvals. External approval evidence is not a separate state transition mechanism.

### Deploy And Verify

Deploy and verify phases call normal Bach targets.

Deploy phases remain Target-backed. A future Deployment Provider may coordinate external deployment systems or change-management workflows, but it must act through a Bach Target/Run contract so deployment remains auditable through normal Bach evidence. Deployment Providers are not alternate workflow engines.

Deploy and verify environment names are workflow-defined labels such as `staging`, `production`, or `preview`, not a fixed built-in enum. Phase strings are canonicalized from those labels, such as `deploy.production` and `verify.production`.

Matching verify phases are recommended but not required by syntax in v1. Workflow authors can deploy without a matching verify phase, though completion evidence is stronger when each deploy environment has a corresponding verify target.

Deploy approval is explicit. Environment names do not imply approval behavior; `deploy "production"` requires approval only when `requires_approval = true` is configured.

Deploy and verify target Runs are linked to the Factory Work Item attempt, phase, and environment so deployment evidence is visible from both normal Run history and Work Item inspection.

If a daemon crashes after approval but before a deploy starts, the same attempt and phase may resume using the existing approval. Reapproval is not required unless the attempt, phase, approved evidence, or workflow state changes.

```hcl
deploy "staging" {
  target = pipeline.deploy_staging
}

verify "staging" {
  target = group.staging_gate
}

deploy "production" {
  target = pipeline.deploy_production
  requires_approval = true
}

verify "production" {
  target = group.production_gate
}
```

The Factory does not introduce new deploy execution semantics in v1.

### Evidence And Findings

Bach records normalized per-run evidence. Atelier computes dashboards, cross-run trends, common failures, and self-improvement recommendations.

Bach should record or export:

- Run.
- TargetRun.
- FactoryWorkItem.
- FactoryPhase.
- AgentAttempt.
- AgentReport.
- PolicyVerdict.
- QualityFinding.
- QualityMetric.
- ArtifactManifest.
- Approval.

Bach may compute stable per-finding fingerprints. Atelier owns cross-run and cross-repo aggregation.

The Backend interface should support operational reads and writes needed by Bach, not dashboard analytics.

Operational reads and writes include:

- enqueue work item.
- claim next work item.
- update work item status.
- record approval.
- record trigger cursor.
- record run/evidence export.
- record normalized finding and metric summaries.
- inspect run/evidence for CLI.

Not included in Bach Backend APIs:

- aggregate findings by repository.
- failure trend dashboards.
- model/prompt success analytics.
- cross-organization reporting.

### Atelier Boundary

Atelier is the future managed-control-plane consumer.

Atelier should consume supported evidence exports and Backend/provider records, not private Bach SQLite tables or internal Go packages.

Atelier owns:

- organization state.
- encrypted credentials.
- prompt packs and AgentPacks.
- runtime provisioning.
- dashboards.
- common-failure analysis.
- self-improvement recommendations.
- workflow scheduling through Atelier Use-cases and OpenWorkflow.

Bach owns:

- local execution.
- Bachfile syntax.
- plan-first workflows.
- Targets.
- Policies.
- factory daemon behavior.
- normalized evidence production.
- local dry-runs.

## Target HCL Shape

Root Bachfile can compose focused files:

```hcl
import "Bachfile.agents"
import "Bachfile.deploy"
import "Bachfile.factory"

project "my-repo" {
  root = "."

  backend {
    type = "sqlite"
  }
}
```

`Bachfile.agents` owns provider, prompt, policy, and template machinery.

```hcl
provider "opencode" {
  type = "opencode"
}

prompt "implementer" {
  path = "prompts/agents/implementer.md"
}

agent_template "implementer" {
  provider = provider.opencode
  role     = "implementer"
  prompt   = prompt.implementer

  workspace {
    mode = "clone"
    path = ".bach/agents/${factory.workflow}/${work_item.id}/${plan.id}"
  }

  git {
    branch = "bach/${factory.workflow}/${work_item.id}/${plan.id}"
    commit = "required"
  }
}
```

`Bachfile.deploy` owns normal deploy and verify targets.

```hcl
pipeline "deploy_staging" {
  steps = [shell.build, shell.deploy_staging]
}

group "staging_gate" {
  targets = [shell.smoke_staging]
}
```

`Bachfile.factory` owns daemon configuration.

```hcl
factory "sldc" {
  repo = "."

  triggers {
    manual {}

    provider "github_issues" {
      command = ["atelier-trigger-github-issues"]

      config = {
        repo   = "owner/repo"
        labels = ["factory:ship"]
      }

      route {
        label    = "factory:ship"
        workflow = "ship"
      }
    }
  }

  workflow "ship" {
    plan {
      template          = agent_template.planner
      policy            = policy.plan_ready
      requires_approval = true
    }

    implement {
      template = agent_template.implementer
      policy   = policy.implementation_ready
    }

    merge {
      template = agent_template.merge
      policy   = policy.merge_ready
    }

    deploy "staging" {
      target = pipeline.deploy_staging
    }

    verify "staging" {
      target = group.staging_gate
    }

    deploy "production" {
      target = pipeline.deploy_production
      requires_approval = true
    }

    verify "production" {
      target = group.production_gate
    }
  }
}
```

## Trigger Provider Protocol

Trigger providers use JSON-RPC over stdio.

Minimal methods:

```text
trigger.handshake
trigger.poll
trigger.ack
trigger.nack
```

`trigger.handshake` request:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "trigger.handshake",
  "params": {
    "protocol": "bach.trigger.v1",
    "factory": "sldc",
    "trigger": "github_issues"
  }
}
```

response:

```json
{
  "protocol": "bach.trigger.v1",
  "provider": "atelier-github-issues",
  "version": "0.1.0",
  "capabilities": ["poll", "ack"]
}
```

`trigger.poll` request:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "trigger.poll",
  "params": {
    "cursor": "opaque-cursor-or-empty",
    "config": {
      "repo": "owner/repo",
      "labels": ["factory:ship"]
    }
  }
}
```

response:

```json
{
  "cursor": "opaque-next-cursor",
  "items": [
    {
      "source": {
        "type": "github_issue",
        "id": "owner/repo#123",
        "url": "https://github.com/owner/repo/issues/123"
      },
      "dedupe_key": "github_issue:owner/repo#123:v4",
      "title": "Add billing webhook",
      "body": "Issue body...",
      "labels": ["factory:ship"],
      "metadata": {
        "author": "kris",
        "issue_number": 123
      }
    }
  ]
}
```

Bach routes the normalized item to a Factory Workflow after poll. Providers do not choose workflows directly.

## Documentation And Schemas

Provider protocols become public contracts when implemented. The Backend Provider protocol, bundled `bach backend sqlite` provider entrypoint, Trigger Provider protocol, and related CLI/Bachfile syntax should be documented in generated reference docs once shipped.

Machine-readable provider message and DTO contracts should live under `docs/schemas/` when they ship. Design notes can describe planned protocol shape before implementation, but shipped protocol details belong in reference docs and schemas.

Provider and evidence DTOs use RFC3339 UTC timestamps. Validation is strict for top-level DTO fields while explicit `metadata` objects remain open for provider/source details. Persisted evidence, finding, Work Item, and related DTOs include schema versions where they may outlive a single RPC call; the JSON-RPC protocol version is negotiated through `backend.initialize` or provider handshake. Enum values use lower snake case for statuses and dot-separated canonical phases such as `deploy.production`.

Phase 1 schema files should use a protocol-focused split: `docs/schemas/backend-provider-v1.schema.json`, `docs/schemas/backend-run-v1.schema.json`, `docs/schemas/backend-evidence-ref-v1.schema.json`, and `docs/schemas/backend-finding-v1.schema.json`.

Phase 1 reference updates cover Bachfile Backend syntax, default Backend behavior, Backend Provider protocol, `bach backend sqlite`, UUIDv7 Run ID/run-history migration behavior, and CLI JSON/redaction notes. Examples should prefer omitting `backend` so they use the implicit default; one advanced example should show the explicit stdio SQLite Backend config.

## Factory Work Item States

Factory Work Items expose a top-level lifecycle plus current phase.

Top-level lifecycle vocabulary:

```text
pending
planning
active
waiting_approval
blocked
completed
failed
cancelled
```

Current phase examples:

```text
plan
implement
merge
deploy.staging
verify.staging
deploy.production
verify.production
```

These states are Backend records and should be exposed through CLI and future evidence export, not through direct table reads.

## CLI Surface

Initial commands:

```sh
bach factory submit sldc --workflow ship --title "Add billing webhook" --body-file request.md
bach factory submit sldc --workflow ship --plan plans/fix-billing.md
bach factory start sldc
bach factory start sldc --jobs 4
bach factory list sldc
bach factory status sldc
bach factory inspect sldc <work-item-id>
bach factory approve sldc <work-item-id> --phase plan
bach factory approve sldc <work-item-id> --phase deploy.production
bach factory cancel sldc <work-item-id> --reason "duplicate"
bach factory retry sldc <work-item-id> --from-phase implement
bach factory replan sldc <work-item-id>
```

Likely follow-ups:

```sh
bach factory intake sldc --trigger github_issues
bach factory explain sldc
bach factory events sldc <work-item-id>
```

## Implementation Phases

### Phase 1: Backend Syntax And Internal Backend Architecture

Goal: make Backend the project persistence abstraction before adding Factory state.

Scope:

- Replace `project.state` with `project.backend`.
- Default omitted backend to the bundled SQLite stdio Backend Provider command with `path = ".bach/state.db"`.
- Hard-error on `state =`.
- Accept only `backend.type = "stdio"` initially.
- Require provider commands as argv arrays, not shell strings.
- Treat provider `config` as mostly opaque and provider-validated.
- Add supported low-level `bach backend sqlite` JSON-RPC stdio provider entrypoint.
- Define the stable Backend Provider protocol, `backend.initialize` capabilities, domain RPCs, and DTO reads.
- Store new run records plus normalized findings through the Backend protocol.
- Switch all new public IDs, including Run IDs and run evidence paths, to UUIDv7 through a shared internal ID package.
- Drop legacy non-UUIDv7 local run history during the Backend/SQLite provider migration; do not expose legacy IDs in new JSON contracts.
- Keep `.bach/state.db` as the default SQLite database path.
- Move `internal/state` language toward `internal/backend` and make SQLite provider own schema/migrations.
- Use `pkg/backendprotocol` and `pkg/jsonrpcstdio` as public stable protocol SDK packages for provider authors.
- Use `internal/id` for shared UUIDv7 generation with injectable deterministic test sources.
- Keep the `bach backend sqlite` Cobra command adapter in `internal/cli`, wired through the composition root.
- Fully rename `internal/state` toward the Backend architecture rather than leaving a long-term state wrapper.
- Update docs, examples, tests, schemas, and AGENTS/DOX where terminology changes.
- Include stdio JSON-RPC conformance tests for `bach backend sqlite` covering initialization, run lifecycle, evidence references, quality reports, findings, errors, and framing through the actual provider process.
- Add a small stdio fixture Backend Provider for client, framing, and domain-error tests independent of SQLite.
- Add CLI e2e coverage for `bach backend sqlite --help`, default Backend use, invalid legacy `state` syntax, and provider startup behavior.
- Verify Phase 1 with Bach targets for formatting, lint, unit tests, e2e/provider CLI tests, docs generation after reference changes, and the full gate when the slice is ready.
- Before coding Phase 1, create `plans/phase-1-backend-stdio-sqlite-uuidv7.md` with workstreams, file-level change map, tests, docs/schema updates, and acceptance criteria.
- Phase 1 acceptance requires default Backend behavior through stdio SQLite, UUIDv7 public IDs including Run IDs and run paths, existing persistence writes through Backend, findings storage through Backend, public SDK packages, schemas/reference updates, e2e tests, and stdio provider conformance tests.
- Split the Phase 1 plan into 4-5 workstreams, roughly config/CLI, protocol SDK, SQLite provider/migration, runner integration/UUIDv7, and docs/tests.

### Phase 2: Agent Templates

Goal: define reusable agent machinery independently of runnable Agent Targets.

Status: delivered.

Scope:

- Add `agent_template` Bachfile block.
- Validate provider, prompt, workspace, git, role, policy defaults.
- Keep templates non-runnable and out of `bach list` normal target output unless explicitly requested.

### Phase 3: Factory Work Items And Manual Queue

Goal: parse Factory daemon configuration enough to queue manual submissions.

Status: delivered.

Scope:

- Add top-level `factory` block with `workflow` and manual trigger support.
- Add Backend domain operations and SQLite provider storage for Factory Work Items, attempts, and events.
- Add `bach factory submit`, `bach factory list`, `bach factory inspect`, and `bach factory cancel`.
- No daemon execution yet beyond queueing.

### Phase 4: Plan Status Foundation

Goal: load durable Plans and Plan-level dependency graphs without executing agents.

Status: delivered.

Scope:

- Add Plan metadata inference and optional frontmatter parser.
- Add Plan document, dependency, and hash models.
- Add `bach plan status`.
- Add Plan-level hash calculation.
- Add Backend Plan ledger/evidence storage and read model.
- Add multi-plan DAG validation and dry-run wave output.

### Phase 5: Plan Execution

Goal: execute one or more accepted Plans through generated Agent Targets.

Scope:

- Materialize one temporary implementer Agent Target from each accepted Plan + template.
- Run implementer policy as existing Agent Target policy fan-out.
- Write Backend Plan ledger/evidence records after evidence passes.
- Support idempotent skip when ledger hash/evidence remains valid.
- Add `bach plan implement` for focused Plan execution.

### Phase 6: Factory Daemon Execution

Goal: run queued work items through Factory Workflows.

Scope:

- Add `bach factory start`.
- Add `bach factory start --dry-run` preflight in a later hardening phase.
- Add `bach factory status`.
- Claim pending work items.
- Run plan phase.
- Run implement phase through plan-first execution.
- Run merge phase.
- Run deploy/verify target phases.
- Persist phase status and evidence links.
- Enforce Backend-held daemon lease plus merge/deploy lock leases.
- Support global `--jobs`, default `1` for daemon active Work Items.

### Phase 7: Multi-Plan Batch And Review Queue

Goal: support overnight-style plan batches after the daemon can process queued work.

Scope:

- Execute ready waves across selected plans.
- Support `--parallelism` and explicit stop modes.
- Write batch summaries.
- Add morning review queue grouped by decision state.
- Record evidence links to runs, commits, policy verdicts, and artifacts.

### Phase 8: Approvals

Goal: support durable approval-gated phases.

Scope:

- Add `requires_approval` to deploy phases.
- Make plan approval default true with optional explicit opt-out.
- Add `waiting_approval` state.
- Add `bach factory approve <factory> <work-item-id> --phase <phase>`.
- Store approval evidence in Backend.
- Resume approved work item from the gated phase.

### Phase 9: Trigger Provider Protocol

Goal: make non-manual triggers pluggable without shipping private provider logic.

Scope:

- Add trigger-provider HCL block.
- Add JSON-RPC-over-stdio client.
- Implement `handshake`, `poll`, `ack`, and `nack` protocol.
- Store trigger cursors in Backend.
- Route normalized provider items to workflows.
- Add fixture provider tests.

### Phase 10: Evidence And Finding Export

Goal: make Backend evidence useful to external dashboards and improvement loops without exposing storage internals.

Scope:

- Define normalized evidence records for Factory Work Items, phases, approvals, findings, metrics, policy verdicts, and artifact manifests.
- Persist stable per-run and per-finding fingerprints.
- Add CLI JSON export surfaces for factory work items and evidence.
- Keep analytics out of Bach.

### Phase 11: External Backend Providers

Goal: enable non-SQLite Backend Providers through the already-stable stdio JSON-RPC Backend protocol.

Scope:

- Add fixture external Backend Provider implementation for tests.
- Allow non-bundled Backend Provider commands after protocol conformance tests pass.
- Validate command preflight, capabilities, provider stderr handling, and failure behavior.
- Preserve one authoritative Backend per Project.

### Phase 12: Private Providers And Atelier Integration

Goal: build the moat outside Bach core.

Scope:

- Private GitHub Issues trigger provider.
- Private Discord trigger provider.
- Private Atelier intake provider.
- Atelier evidence/backend provider.
- Atelier Project factory bootstrap.
- Atelier dashboards for findings, common failures, and self-improvement loops.

## Open Questions For Phase Drilldown

1. Should the internal Backend client facade be flat or split into domain subclients from the first implementation?
2. Which optional Plan frontmatter fields remain necessary after path and heading inference?
3. What interpolation variables are allowed in `agent_template` paths and branches?
4. Should generated target addresses be visible through `bach list --generated` or dry-run only?
5. How should Plan ledger/evidence records relate to Factory Work Item attempts and phase state?
6. What exact Backend Provider DTO schemas ship in v1?
7. What conformance tests certify external Backend Providers?
8. What is the first normalized finding schema version?
9. What evidence is too sensitive for external Backends or managed control planes by default?
10. How should graceful daemon shutdown mark active attempts and phase boundaries?
