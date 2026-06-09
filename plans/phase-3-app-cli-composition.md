# Phase 3 - `app-cli-composition`: make app the composition root

This phase finishes the package-direction cleanup after runner stopped importing config. The current graph is much cleaner, but `internal/cli` still imports `internal/config` for project loading, config load options, explanation/list/affected helpers, and state query presentation paths. `internal/config` also still imports `internal/state` through stale state aliases and runtime helper files that no longer belong to Bachfile decoding.

The goal is to make `internal/app` the production composition root described by ADR-0013, keep `internal/cli` focused on the CLI Contract, and shrink `internal/config` toward Bachfile loading/decoding/validation over model data.

Depends on Phase 2, where `internal/runner` consumes `model.RunProject` / `model.RunTarget` and no longer imports `internal/config`.

---

## Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Phase identity | `phase-3-app-cli-composition`. |
| 2 | Composition root | `internal/app` owns production wiring for project loading, runtime project adaptation, runner invocation, state queries, and config-backed graph/list/explain/affected services. |
| 3 | CLI role | `internal/cli` owns command parsing, flags, argument validation, output formatting, and command adapter registration. |
| 4 | CLI config dependency | Remove production `internal/cli -> internal/config` imports. CLI receives interfaces/functions with model-shaped data from app. |
| 5 | Config state dependency | Remove `internal/config -> internal/state` once obsolete state aliases/runtime helpers are moved or deleted. |
| 6 | Runtime helpers left in config | Delete or move config copies of runner-owned runtime helpers: artifact indexing, summary formatting, preflight/tool reporting, log helpers, state wrappers. |
| 7 | Config role | Keep config responsible for Bachfile loading, decode-time validation, env/profile resolution, plugin graph loading, target spec projection, graph/list/explain/affected domain helpers until a later graph/plugin package exists. |
| 8 | Model shape | Do not reintroduce a wide runtime target. Continue using `model.RunTarget{SpecValue model.TargetSpec}` for runner input. |
| 9 | CLI contract | No command, flag, human output, dry-run JSON, State Store schema, or Bachfile syntax changes. |
| 10 | Tests | Preserve existing CLI tests through dependency injection. Add import-boundary tests or grep checks only if they are low-friction. |
| 11 | Out of scope | No public Go API, no external plugin contract, no target handler decode redesign, no graph/plugin package extraction yet. |

---

## 0.1 Why App Owns Wiring Now

ADR-0013 says `internal/app` is the Composition Root, but today it only passes `config.LoadWithOptions` into CLI. That leaves CLI aware of config-specific types and keeps production wiring spread across `internal/cli/root.go`, `internal/cli/run.go`, and `internal/app/app.go`.

After Phase 2, runner already consumes model runtime data. That makes this the right time to invert CLI dependencies: CLI should ask for a loaded command project and call command services, while app decides that production loading comes from config and production run execution comes from runner.

## 0.2 Why Config State Aliases Must Go

`internal/config/state.go` exists as a compatibility facade over `internal/state`. That made sense while runner/config were entangled, but now it causes the wrong graph:

```text
config -> state
```

Config should decode Bachfiles and validate model data. State is a separate persistence subsystem. Keeping state aliases in config makes config look like the runtime hub even after runtime behavior moved away.

---

## 1. Architecture

Current graph after Phase 2 implementation:

```text
app
  -> cli
  -> config

cli
  -> config
  -> model
  -> runner
  -> state

config
  -> model
  -> state

runner
  -> model
  -> target
  -> quality
  -> state

target
  -> model

quality
  -> model
  -> state

state
  -> no internal deps

model
  -> no internal deps
```

Target graph after this phase:

```text
app
  -> cli
  -> config
  -> model
  -> runner
  -> state

cli
  -> model
  -> state

config
  -> model

runner
  -> model
  -> target
  -> quality
  -> state

target
  -> model

quality
  -> model
  -> state

state
  -> no internal deps

model
  -> no internal deps
```

Longer-term graph after a later graph/plugin extraction:

```text
app
  -> cli
  -> config
  -> graph
  -> runner
  -> quality
  -> state
  -> target

cli
  -> model
  -> state

config -> model
graph  -> model
runner -> model, target, quality, state
target -> model
quality -> model, state
state
model
```

---

## 2. Schema

No State Store schema changes.

---

## 3. Events

No events.

---

## 4. Use-cases

- `bach run <target>` still loads the Bachfile, resolves aliases, prints alias deprecation hints, and invokes runner.
- `bach list`, `bach explain`, `bach graph`, and `bach affected` still render exactly the same output.
- `bach runs`, `bach artifacts`, and `bach quality ...` still query the State Store directly through the intended state/quality boundaries.
- CLI tests can provide fake project loaders or fake command services without importing config.
- App tests can prove production wiring loads config, adapts runtime projects, and invokes runner.

---

## 5. CLI Contract

No CLI Contract changes.

Output stability is load-bearing for this phase. The command adapter refactor must not change:

- command names.
- flags or flag defaults.
- target address parsing behavior.
- alias warning text.
- list/explain/graph/affected output.
- run summaries.
- dry-run JSON.

---

## 6. Workstreams

### Workstream A - App-Owned Project Services

Goal: give CLI model-shaped command services instead of config-shaped dependencies.

- [ ] Replace `cli.ProjectLoader`'s config-specific signature with a CLI-owned load request/result shape.
- [ ] Add app production implementation that calls `config.LoadWithOptions`.
- [ ] Move `config.RuntimeProject(project)` call out of `internal/cli/run.go` and into app's run service wiring.
- [ ] Keep alias resolution and alias warning semantics identical.
- [ ] Update CLI tests to inject fake loaded projects/services without importing config where practical.

### Workstream B - CLI Command Services

Goal: remove direct CLI calls into config helpers while preserving command presentation.

- [ ] Introduce CLI dependency functions for list, explain, graph, affected, and run execution.
- [ ] Keep output formatting in CLI unless the formatting is clearly subsystem-owned.
- [ ] Have app wire config-backed implementations for list/explain/graph/affected.
- [ ] Have app wire runner-backed implementation for run execution.
- [ ] Confirm `internal/cli` production code has no `internal/config` import.

### Workstream C - Config State Cleanup

Goal: remove stale config-to-state facade code.

- [ ] Delete or move `internal/config/state.go` and `internal/config/state_test.go` if no production code uses them.
- [ ] Delete or move config copies of artifact indexing and summary helpers if runner owns the behavior.
- [ ] Delete or move config copies of required-tool/preflight reporting if runner owns the behavior.
- [ ] Delete or move config log helper files if only obsolete runtime helpers use them.
- [ ] Update config summary/preflight tests to either move with the helper or delete if duplicated by runner tests.
- [ ] Confirm `internal/config` production code has no `internal/state` import.

### Workstream D - Boundary Documentation

Goal: make the final package graph explicit before OSS.

- [ ] Update or add a short architecture note showing the current dependency graph.
- [ ] Update `CONTEXT.md` only if term definitions changed; avoid churn if this phase only fulfills existing direction.
- [ ] Consider adding a supersession note to ADR-0013 only if the implemented wiring meaningfully narrows the ADR.
- [ ] Run import-boundary grep checks and full verification.

---

## 7. Tests

- `rtk go test ./...`
- `go run ./cmd/bach affected`
- `go run ./cmd/bach run shell/test`

Focused checks:

- `rtk go test ./internal/cli ./internal/app ./internal/config`
- import grep: `internal/cli` has no `internal/config` production import.
- import grep: `internal/config` has no `internal/state` production import.
- behavior checks for `list`, `explain`, `graph`, `affected`, `run --dry-run`, `runs`, `artifacts`, and `quality` through existing tests.

---

## 8. Open questions / known limitations

1. **OQ-1 - Should CLI depend on `internal/state`?** Acceptable in this phase for run/artifact/quality query presentation, but app-owned query services could remove it later if CLI becomes pure presentation.
2. **OQ-2 - Should graph/list/explain/affected leave config now?** No. Keep them config-backed until a dedicated graph/plugin package is planned. This phase only moves the dependency direction through app.
3. **OQ-3 - Should runtime helpers duplicated in config be deleted immediately?** Yes if unused. If a config command still needs one, move it to the package that owns the behavior rather than keeping config as a facade.

---

## 9. Out of scope

- Extracting `internal/graph` or `internal/plugin`.
- Redesigning config target body decoding.
- Adding external plugins or public registries.
- Changing Bachfile syntax.
- Changing command output.
- Changing State Store schema.
- Removing `internal/cli -> internal/state` unless it falls out naturally.

---

## 10. Phase Boundary

Done when:

1. `internal/app` is the production wiring point for config loading, runtime project adaptation, runner execution, and config-backed project services.
2. `internal/cli` production code does not import `internal/config`.
3. `internal/config` production code does not import `internal/state`.
4. Obsolete config runtime helper files are deleted or moved to their owning subsystem.
5. CLI behavior remains stable under existing tests.
6. Full Go tests and Bach `shell/test` pass.
