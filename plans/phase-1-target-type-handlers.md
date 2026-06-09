# Phase 1 - `target-type-handlers`: split target specs from target behavior

This phase makes the target model less misleading by separating the common target envelope from shell, image, and pipeline-specific payloads. Today `model.TargetSpec` carries `Command`, `Image`, and `Pipeline` fields at the same time, even though only one target type should use one of those payloads. The runtime API is also command-centric, even though future target types may be MCP calls, gRPC calls, HTTP requests, Kubernetes operations, or native Go actions with no local process command.

This phase is behavior-preserving. It does not change Bachfile syntax, target names, cache behavior, command output, image command generation, pipeline semantics, quality ingestion, or plugin contracts.

Depends on the current post-quality-seam codebase.

---

## Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Phase identity | `phase-1-target-type-handlers`. |
| 2 | Model shape | Keep `TargetSpec` as the common envelope; move target-type payload into one typed body. |
| 3 | Body representation | Use `Body TargetBody` so `TargetSpec` does not grow one field per target type. |
| 4 | Interface body | Accept a small `TargetBody` interface now because it models exactly one target-type payload and fits future target handlers better. |
| 5 | Language | New code uses target type / `TargetType` / `TargetHandler`, not kind / `KindHandler` / plugin. |
| 6 | Target type rename | Rename `TargetKind` to `TargetType` across active code and reference docs. Historical ADRs remain as history unless a supersession note is needed. |
| 7 | Runtime vocabulary | Replace command-centric handler language with `Describe` and `Execute`. `command` remains only inside `ShellSpec` and compatibility output. |
| 8 | Operation | Rename `TargetRunRecord.Command` and the State Store `target_runs.command` column to operation. |
| 9 | Config decoding | Target handlers own target-type body decoding; config owns the common envelope and delegates body decoding by target type. |
| 10 | Target runtime role | `internal/target` reads the typed body and owns target-type behavior. |
| 11 | Fingerprint language | Rename operation-level fingerprint parts and stale reasons from command terminology to operation terminology. |
| 12 | Runtime wording | Timeout/retry and completion-contract wording changes from command execution to operation execution. |
| 13 | Docs | Update docs/reference for operation semantics where run, dry-run, state, cache, timeout/retry, and contracts changed; keep shell/probe/plugin command docs where they truly mean commands. |
| 14 | Compatibility | Bachfile syntax and normal human CLI output remain stable except dry-run JSON field rename to `operation`. |
| 15 | Out of scope | No external plugin contract. |

---

## 1. Architecture

Current model shape:

```go
type TargetSpec struct {
	Name     string
	Kind     TargetKind
	Metadata TargetMetadata
	Runtime  TargetRuntime
	Quality  TargetQuality
	Cache    TargetCache
	Command  TargetCommand
	Image    TargetImage
	Pipeline TargetPipeline
	Contract TargetContract
}
```

Target kind is encoded by `Kind`, but each target still exposes every kind-specific field. A shell target has an empty image/pipeline payload. An image target has an empty command/pipeline payload. A pipeline target has an empty command/image payload.

New model shape:

```go
type TargetSpec struct {
	Name     string
	Metadata TargetMetadata
	Runtime  TargetRuntime
	Quality  TargetQuality
	Cache    TargetCache
	Contract TargetContract
	Body     TargetBody
}

type TargetBody interface {
	TargetType() TargetType
}

type ShellSpec struct {
	Command []string
	Shell   string
	WorkDir string
}

func (ShellSpec) TargetType() TargetType { return TargetTypeShell }

type ImageSpec struct {
	Builder    string
	Image      string
	Tags       []string
	Dockerfile string
	Context    string
	Platform   string
	Push       bool
	BuildArgs  []string
}

func (ImageSpec) TargetType() TargetType { return TargetTypeImage }

type PipelineSpec struct {
	Steps []string
}

func (PipelineSpec) TargetType() TargetType { return TargetTypePipeline }
```

This keeps `TargetSpec` as the stable envelope while making the kind-specific payload exactly one value.

New target handler shape:

```go
type TargetHandler interface {
	Type() model.TargetType
	Runnable(model.TargetBody) bool
	Describe(context.Context, DescribeRequest) (RunDescription, error)
	Execute(context.Context, ExecuteRequest) error
	FingerprintParts(model.TargetBody) map[string]string
}

type RunDescription struct {
	Operation string
	WorkDir   string
}
```

`Describe` gives runner and dry-run JSON a human-readable operation without assuming the operation is a shell command. `Execute` runs the target-type behavior, which may be local process execution for shell/image or non-process execution for future target types.

Target handlers also own target-type body decoding. Config remains responsible for decoding the shared target envelope, resolving variables/env blocks, aliases, inputs, resources, and graph wiring. Once config determines a target type, it delegates the target-specific body to the corresponding handler. This avoids preserving the wide config target shape as a second source of target-type truth.

---

## 2. Schema

State Store schema changes are in scope.

`target_runs.command` currently stores the human-readable command string used by shell-like targets. This phase renames the concept to operation because not every target type has a command.

Logical model change:

```go
type TargetRunRecord struct {
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	LogPath    string
	Operation  string
}
```

SQLite storage change:

```text
target_runs.command         -> target_runs.operation
```

Migration requirements:

- Existing databases with only `command` must still load.
- New writes should use `operation`.
- If both columns exist during migration, `operation` wins and `command` is fallback read compatibility.
- Dry-run JSON should emit `operation`, not `command`.

---

## 3. Events

No events.

---

## 4. Use-cases

- A shell target exposes shell payload only through `spec.Body.(ShellSpec)`.
- An image target exposes image payload only through `spec.Body.(ImageSpec)`.
- A pipeline target exposes pipeline payload only through `spec.Body.(PipelineSpec)`.
- Target handlers fail clearly if the expected body is missing.
- Runner logs and dry-run output use an operation from `Describe`, not a required command string.
- Shell targets may still have commands inside `ShellSpec`; non-shell target types do not need commands.
- Run records persist `Operation`, not `Command`.
- Planner and runner keep behavior stable while using typed bodies.

---

## 5. CLI Contract

Normal human CLI output remains stable. Dry-run JSON changes `targets[].command` to `targets[].operation` because dry-run describes operations, not necessarily shell commands.

Docs/reference should be updated for the semantic rename where it affects the CLI contract:

- Dry-run output describes planned operations.
- Run records store operations.
- Cache stale reasons use operation terminology.
- Timeout/retry wrap operation execution.
- Completion contracts run after operation execution.

Docs should keep command terminology where the domain object is still a command:

- Shell target `command` / `shell` fields.
- Required tool probe commands.
- Preflight probe commands.
- Completion-check verification commands.
- Graph-load plugin launch commands.

---

## 6. Workstreams

### Workstream A - Model Shape

Goal: introduce a single typed target-type body in `internal/model`.

- [ ] Replace `TargetCommand`, `TargetImage`, and `TargetPipeline` envelope fields with `Body TargetBody`.
- [ ] Add `TargetType`, `TargetBody`, `ShellSpec`, `ImageSpec`, and `PipelineSpec` body types.
- [ ] Rename `TargetKind` constants/types/aliases to `TargetType` in active code.
- [ ] Keep existing shared envelope fields: metadata, runtime, quality, cache, contract.
- [ ] Add helper predicates only if they remove repeated nil checks.

### Workstream B - Target Body Decoding

Goal: move target-type body decoding behind target handlers while keeping config responsible for the shared Bachfile envelope.

- [ ] Add a handler decode method for shell/image/pipeline target bodies.
- [ ] Update config loading to delegate target-specific body decoding to the registered target handler.
- [ ] Preserve existing target-type detection rules and Bachfile syntax.
- [ ] Preserve copy semantics for slices/maps.

### Workstream C - Target Handlers

Goal: make `internal/target` consume typed bodies through target handlers and remove command-centric runtime naming from new interfaces.

- [ ] Rename `KindHandler` / `KindRegistry` concepts to `TargetHandler` / target-type registry.
- [ ] Add target body decoding to `TargetHandler`.
- [ ] Replace `CommandString` with `Describe` returning a `RunDescription` operation.
- [ ] Replace command-list oriented execution with `Execute` at the handler boundary.
- [ ] Shell handler may internally build local `exec.Cmd` values from `ShellSpec`.
- [ ] Image handler may internally build local `exec.Cmd` values from `ImageSpec`.
- [ ] Pipeline handler returns a pipeline operation label and keeps direct execution orchestration in runner for this phase.
- [ ] Return clear errors for missing/mismatched bodies.

### Workstream D - Runner/Planner Call Sites

Goal: update existing call sites with minimal churn.

- [ ] Update planner pipeline step traversal.
- [ ] Update runner workdir selection.
- [ ] Update dry-run JSON to emit `operation` instead of `command`.
- [ ] Update target run start records to store operations in `Operation`.
- [ ] Update any remaining direct references to `spec.Command`, `spec.Image`, or `spec.Pipeline`.
- [ ] Rename operation-level fingerprint key/stale reason from command to operation.
- [ ] Update timeout/retry and completion-contract code comments/errors/docs to refer to operation execution where they are not shell-specific.

### Workstream E - State Store Rename

Goal: rename persisted target-run command terminology to operation terminology.

- [ ] Rename `state.TargetRunRecord.Command` to `Operation`.
- [ ] Rename the SQLite column from `target_runs.command` to `target_runs.operation`.
- [ ] Add migration/read compatibility for existing state databases with `command` only.
- [ ] Update save/load paths and state tests.

### Workstream F - Tests

Goal: preserve behavior and pin the new shape.

- [ ] Update target registry tests for typed bodies.
- [ ] Add config decoding tests proving shell/image/pipeline bodies are produced through handlers.
- [ ] Add tests for dry-run JSON `operation` field.
- [ ] Add tests for operation stale reason/fingerprint behavior.
- [ ] Keep runner integration tests green.

### Workstream G - Docs

Goal: align documentation with operation terminology without erasing real command concepts.

- [ ] Update reference docs for dry-run planned operations.
- [ ] Update run/state/log docs from command record terminology to operation terminology.
- [ ] Update cache docs from changed command configuration to changed operation configuration.
- [ ] Update timeout/retry and completion-contract docs to operation execution.
- [ ] Update active reference docs from target-kind terminology to target-type terminology where applicable.
- [ ] Keep shell target, tool/preflight probe, completion-check command, and graph-load plugin command docs using command terminology.

---

## 7. Open questions / known limitations

1. Should typed bodies be pointer fields or a `TargetBody` interface? Phase 1 chooses `TargetBody` because the common spec should not know every concrete kind slot.
2. Should target plugins depend on this typed-body shape? Not in this phase; external plugin contract design comes later.
3. Should dry-run JSON preserve a compatibility `command` field? No; this phase renames the field to `operation`.
4. Should config keep a wide decoded target shape as a transitional detail? No; target handlers own target-type body decoding in this phase.
5. Should historical ADRs be rewritten from target kind to target type? No; leave history intact unless a supersession note is needed.

---

## 8. Out of scope

- Bachfile syntax changes.
- External target plugin contract.
- Quality plugin contract.
- Config runtime adapter cleanup unrelated to target body decoding and operation terminology.
- Pipeline execution unification.
- Cache fingerprint behavior changes.
- CLI output changes beyond the dry-run JSON field rename from `command` to `operation`.

---

## 9. Phase Boundary

Done when:

1. `model.TargetSpec` no longer exposes `Command`, `Image`, and `Pipeline` as parallel value fields.
2. Config delegates target-type body decoding to target handlers and produces exactly one `Body` value per target.
3. `internal/target` handlers use typed bodies and reject missing bodies clearly.
4. Active code uses `TargetType`, not `TargetKind`.
5. New target runtime interfaces use `Describe`/`Execute`, not `CommandString`/`Commands` as the public handler vocabulary.
6. Runner/planner no longer rely on empty target-type payloads.
7. `TargetRunRecord.Operation` and `target_runs.operation` replace command terminology in State Store code/storage.
8. Existing State Store databases with `target_runs.command` continue to load.
9. Dry-run JSON emits `operation`, not `command`.
10. Operation-level fingerprint parts and stale reasons use operation terminology.
11. Docs/reference use operation terminology for run/dry-run/state/cache/runtime semantics and preserve command terminology for shell/probe/plugin commands.
12. `go run ./cmd/bach run shell/test` passes.
