# Phase 2 - `runtime-project-model`: remove runner's config dependency

This phase removes the remaining runtime dependency from `internal/runner` to `internal/config`. The previous phase moved target-type behavior and operation execution out of config, but runner still imports config for loaded `Project` / `Target` data, git and dotenv helpers, required tool/preflight runtime checks, artifact indexing, run summary printing, and a few type aliases.

The goal is to make config a Bachfile loader/decoder only. Runtime execution should consume model/runtime data and explicit subsystem services wired by `internal/app`.

Depends on Phase 1, where target-type handlers and operation terminology replaced command-centric target runtime adapters.

---

## Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Phase identity | `phase-2-runtime-project-model`. |
| 2 | Runtime data ownership | Move runtime `Project`, `Target`, `Alias`, `Input`, and `Resource` shapes out of config and into `internal/model`. |
| 3 | Config role | Config decodes Bachfile syntax and returns a `model.Project`; it does not own runtime target behavior. |
| 4 | Runner dependency | `internal/runner` must not import `internal/config` in production code. |
| 5 | App wiring | `internal/app` or CLI loading code may still call config loader and pass the resulting model project to runner. |
| 6 | Tool/preflight checks | Move runtime required-tool and preflight checking out of config, preferably into runner for this phase. |
| 7 | Artifact indexing | Move artifact indexing out of config into runner or a small artifact helper owned by runner/state. |
| 8 | Run summary | Move run summary printing out of config into runner or CLI presentation. |
| 9 | Git/dotenv helpers | Move git context and dotenv loading out of config into runner-owned helpers or a tiny runtime env helper. |
| 10 | Compatibility | No Bachfile syntax, normal CLI output, cache semantics, operation semantics, or State Store schema changes. |
| 11 | Out of scope | No external target plugins, no Composition Root redesign beyond changed function wiring, no CLI adapter registry changes. |

---

## 0.1 Why Runtime Project Leaves Config

`internal/config` should describe how Bachfile syntax becomes runtime data. It should not be the package runner depends on to execute a Run. Keeping `Project` and `Target` in config makes every runtime package look like a config consumer even when it only needs already-decoded target data.

Moving the runtime data model to `internal/model` makes the dependency direction explicit:

```text
config -> model
runner -> model
cli    -> model
```

Config remains important, but as a producer of runtime data rather than a runtime facade.

## 0.2 Why Tool/Preflight Checks Move With Runner

Required tool and preflight checks happen in the Run lifecycle. They inspect planned targets, print dry-run/check output, and can record synthetic target failures. That makes them runtime behavior, not Bachfile decoding.

This phase can move the code into runner without inventing a new package. If checks grow into a reusable subsystem later, they can be extracted from runner once the dependency direction is already clean.

---

## 1. Architecture

Current graph:

```text
internal/runner
  -> internal/config   // Project/Target aliases, git/dotenv, tools/preflights, artifacts, summary
  -> internal/model
  -> internal/target
  -> internal/quality
  -> internal/state

internal/config
  -> internal/model
  -> internal/state
```

Target graph after this phase:

```text
internal/app
  -> internal/cli
  -> internal/config

internal/cli
  -> internal/config    // loading path only, until app owns all wiring
  -> internal/runner
  -> internal/state
  -> internal/model

internal/config
  -> internal/model
  -> internal/state     // only if loader-time state helpers remain; remove if unused

internal/runner
  -> internal/model
  -> internal/target
  -> internal/quality
  -> internal/state

internal/target
  -> internal/model

internal/quality
  -> internal/model
  -> internal/state

internal/state
  -> no internal deps

internal/model
  -> no internal deps
```

Perfect graph to aim for after later phases:

```text
cmd/bach
  |
  v
internal/app
  |-------------------> internal/config
  |-------------------> internal/target
  |-------------------> internal/quality
  |-------------------> internal/state
  |-------------------> internal/runner
  v
internal/cli

internal/config ------> internal/model

internal/runner ------> internal/model
internal/runner ------> internal/target
internal/runner ------> internal/quality
internal/runner ------> internal/state

internal/target ------> internal/model

internal/quality -----> internal/model
internal/quality -----> internal/state

internal/state
internal/model
```

---

## 2. Schema

No State Store schema changes.

---

## 3. Events

No events.

---

## 4. Use-cases

- CLI loads a Bachfile through config and receives a runtime `model.Project`.
- Runner accepts a runtime project without importing config.
- Runner plans, checks, executes, records, indexes artifacts, and prints summaries without config runtime helpers.
- Config tests continue to prove Bachfile decoding behavior.
- Runner tests construct runtime projects from model/config-compatible test helpers without importing config in production code.

---

## 5. CLI Contract

No CLI output changes.

---

## 6. Workstreams

### Workstream A - Runtime Model Types

Goal: make `internal/model` own runtime project graph data.

- [ ] Move `Project`, full runtime `Target`, `Alias`, `Input`, and `Resource` shapes to `internal/model`.
- [ ] Keep HCL-only structs in `internal/config` where tags or decode-only fields are needed.
- [ ] Update `config.Load` / `LoadWithOptions` to return runtime model data or config aliases to model data.
- [ ] Preserve `Target.Spec()` and target-address behavior on runtime model types.

### Workstream B - Runner Type Decoupling

Goal: remove config imports from runner production code.

- [ ] Replace runner aliases from config with aliases/imports from model/state.
- [ ] Move or duplicate git context loading into runner/runtime helpers.
- [ ] Move dotenv/env-file parsing into runner/runtime helpers.
- [ ] Update dry-run JSON and plan code to use model types directly.

### Workstream C - Runtime Checks

Goal: move required-tool and preflight check execution out of config.

- [ ] Move planned tool/preflight structs and check errors into runner or model+runner.
- [ ] Move dry-run reporting and execution of required tools/preflights into runner.
- [ ] Preserve `preflight-failed` status and synthetic target logs.
- [ ] Update tests currently calling config check collectors.

### Workstream D - Artifacts And Summary

Goal: remove config from Run completion presentation/persistence helpers.

- [ ] Move artifact indexing into runner or a small runner-owned helper.
- [ ] Move run summary printing out of config into runner or CLI presentation.
- [ ] Remove now-unused config exports and config state aliases.

### Workstream E - Cleanup And Docs

Goal: make the dependency graph obvious before OSS.

- [ ] Delete obsolete config runtime files or shrink them to decode-only helpers.
- [ ] Add/update a short architecture note showing the package graph.
- [ ] Run import grep proving runner production code has no `internal/config` import.
- [ ] Run full verification.

---

## 7. Tests

- `rtk go test ./...`
- `go run ./cmd/bach affected`
- `go run ./cmd/bach run shell/test`

Add focused tests as needed for:

- config loader returning runtime model project data.
- runner required-tool/preflight check behavior after move.
- artifact indexing after move.
- run summary stability after move.

---

## 8. Open questions / known limitations

1. **OQ-1 - Should CLI still import config directly?** Acceptable for this phase if CLI owns loading. A later Composition Root phase can have app wire loader dependencies into CLI.
2. **OQ-2 - Should tool/preflight checks become their own package?** Not yet. Move them to runner first; extract only if they grow.

---

## 9. Out of scope

- External target plugins.
- External quality plugins.
- State Store schema changes.
- CLI output changes.
- Target operation semantic changes.
- Full Composition Root redesign.

---

## 10. Phase Boundary

Done when:

1. `internal/runner` production code does not import `internal/config`.
2. Runtime `Project` / `Target` data lives outside config.
3. Config is clearly a Bachfile loader/decoder package.
4. Required tool/preflight runtime checks are no longer config-owned.
5. Artifact indexing and run summary printing are no longer config-owned.
6. Full tests and Bach verification pass.
