# Phase 4 - `graph-evidence`: extract graph evidence from config

This phase moves graph-derived project services out of `internal/config` and into a new `internal/graph` package. It also extracts shared Git workspace evidence into `internal/git` so affected-target defaults and runner Git env/fingerprint evidence do not duplicate Git probing. After Phases 2 and 3, config no longer acts as a runner or state facade, but it still owns affected-target matching, target risk traversal, explain output assembly, and Git changed-file discovery. Those are not Bachfile decoding concerns.

The goal is to make config focus on Bachfile load/decode/validation while graph owns read-only project graph evidence used by app-backed CLI services.

Depends on Phase 3, where `internal/app` became the production wiring point and production `internal/cli` stopped importing config.

---

## Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | Phase identity | `phase-4-graph-evidence`. |
| 2 | New package | Add `internal/graph` for read-only graph evidence over loaded projects. |
| 3 | Project shape | `internal/graph` consumes model-shaped project data, not config-specific decoded HCL structs. |
| 4 | Affected ownership | Move affected-target matching and input/resource resolution to `internal/graph`. |
| 5 | Risk ownership | Move effective target risk traversal to `internal/graph`. |
| 6 | Explain ownership | Move explain output assembly to `internal/graph` for now, preserving exact CLI output. |
| 7 | Git ownership | Add `internal/git` for Git workspace evidence. Graph must not own Git probing. |
| 8 | Runner Git reuse | Move runner's private Git context helper to `internal/git` and have runner consume it. |
| 9 | Affected default wiring | App owns the "no explicit affected paths means Git changed files" orchestration by calling git then graph. |
| 10 | Plugin execution | Keep graph-load plugin execution inside config during this phase. Plugin extraction is a later phase. |
| 11 | App wiring | App wires config-loaded projects into graph services for explain, affected, and risk labels used by CLI project adaptation. |
| 12 | Config role | Config retains Bachfile loading, HCL decoding, variables, profiles, env overlays, config validation, plugin execution during load, and runtime project adaptation until later cleanup. |
| 13 | CLI contract | No output, flag, target address, or Bachfile syntax changes. |
| 14 | Tests | Move config affected/explain/risk tests to graph or duplicate narrowly, then delete config tests that no longer belong. |
| 15 | Out of scope | No `internal/plugin` extraction, no config target decoding redesign, no public Go API, no graph CLI output changes. |

---

## 0.1 Why Graph Evidence Leaves Config

Config should answer: "What does this Bachfile declare after variables, profiles, env blocks, and plugins have been applied?"

Graph evidence answers different questions:

- Which targets are affected by these changed paths?
- What risks are inherited through dependency and pipeline edges?
- What facts should explain print for a target or alias?

Those questions operate over a loaded project. They do not need HCL decoding, config block registration, or Bachfile parser internals. Keeping them in config makes config look like the project service hub even after app became the composition root.

Git changed-file discovery is not graph evidence either. It is workspace/VCS evidence. App should decide when affected-target matching needs Git evidence, then pass those concrete paths to graph.

## 0.2 Why Plugin Execution Stays In Config

Graph-load plugins currently run during `config.LoadWithOptions`. They are tied to HCL plugin blocks, eval context objects, loaded inputs/resources, and load-time target patching. Pulling plugin execution out now would make this phase much bigger and would blur the goal.

This phase treats plugin execution as part of project loading. The loaded project can include plugin-provided inputs and dependency patches. `internal/graph` then consumes that already-loaded graph evidence.

## 0.3 Why Git Becomes Its Own Package

Runner already needs Git context for environment variables and cache fingerprints. Affected-target matching also needs changed files when the user does not pass explicit paths. Putting changed-file discovery in graph would mix VCS probing with graph matching. Keeping a private runner Git helper would duplicate Git logic.

`internal/git` should own workspace evidence:

```text
git.Context(ctx, root) -> branch, commit, dirty, staged, unstaged, untracked, changed
git.ChangedFiles(ctx, root) -> changed paths
Context.Env() -> BACH_GIT_* env entries
```

Then app and runner can both consume Git evidence without importing each other or pushing Git behavior into config or graph.

---

## 1. Architecture

Current graph:

```text
app -> cli, config, model, runner
cli -> model, state
config -> model
runner -> model, quality, state, target
quality -> model, state
target -> model
state
model
```

Target graph after this phase:

```text
cmd/bach ---> app

app -------> cli -------> model
 |           |
 |           +----------> state
 |
 +---------> config ----> model
 |
 +---------> graph -----> model
 |
 +---------> git
 |
 +---------> runner ----> model
 |           |----------> target ----> model
 |           |----------> quality ---> model
 |           |----------> git
 |           |             |
 |           +----------> state <-----+
 |
 +---------> target ----> model
 |
 +---------> quality ---> model
 |             |
 +-----------> state <----+
```

Package responsibilities after this phase:

```text
config: Bachfile load/decode/validation/plugin-load patches -> model data
graph: affected/risk/explain evidence over loaded model data
git: workspace/VCS evidence
app: production wiring from config project to graph/git/runner/cli services
cli: command parsing and output formatting
runner: run planning/execution/cache/logs/summaries/state mutation
```

---

## 2. Schema

No State Store schema changes.

---

## 3. Events

No events.

---

## 4. Use-cases

- `bach affected path/to/file` reports the same target/match lines as before, but uses `internal/graph`.
- `bach affected` with no explicit paths still uses Git changed files, but app gets those paths from `internal/git` and passes them to graph matching.
- `bach explain <target>` prints the same fields and inherited risks as before, but uses `internal/graph`.
- `bach list --verbose` and `bach graph` still show inherited risk labels, but app gets those labels from graph while adapting the loaded project for CLI.
- Runner still injects the same `BACH_GIT_*` environment variables and fingerprint evidence, but gets Git context from `internal/git`.
- Config tests continue to prove loading, decoding, plugin execution, env/profile behavior, and validation.
- Graph tests prove affected matching, risk traversal, explain output, and Git path discovery behavior.

---

## 5. CLI Contract

No CLI Contract changes.

The following outputs must remain byte-for-byte compatible unless existing tests already allow flexible matching:

- `bach affected`
- `bach explain <target>`
- `bach list --verbose`
- `bach graph --format json`
- `bach graph --format mermaid`

---

## 6. Workstreams

### Workstream A - Graph Project Shape

Goal: introduce a graph-owned read-only view over loaded model data.

- [ ] Add `internal/graph` package.
- [ ] Define the minimal project/target inputs graph needs, preferably using existing `model` types directly.
- [ ] Avoid depending on config decoded HCL structs.
- [ ] Add adapter code in app if config's loaded project needs projection before graph can consume it.

### Workstream B - Affected Evidence

Goal: move affected-target matching out of config.

- [ ] Move `AffectedTarget` and `AffectedTargets` to `internal/graph`.
- [ ] Move input/resource resolution used for affected matching.
- [ ] Preserve matching behavior for named inputs, plugin inputs, project-root input, globs, and normalized paths.
- [ ] Move `internal/config/affected_test.go` to graph tests.
- [ ] Update app's affected service to call graph.

### Workstream C - Risk And Explain Evidence

Goal: move inherited risk traversal and explain assembly out of config.

- [ ] Move effective target risk traversal to `internal/graph`.
- [ ] Add `TargetRiskLabels` in graph or expose risk facts consumed by app.
- [ ] Move `Explain` output assembly to graph, preserving output.
- [ ] Move `internal/config/explain_test.go` to graph tests.
- [ ] Update app's explain service and CLI project adaptation to call graph.

### Workstream D - Git Workspace Evidence

Goal: extract shared Git probing out of config and runner.

- [ ] Add `internal/git` package.
- [ ] Move runner's `GitContext`, context loading, env rendering, and changed-file helpers into `internal/git`.
- [ ] Update runner to consume `git.Context` for env and fingerprints.
- [ ] Update app's affected service to call `git.ChangedFiles` when no paths are provided, then call `graph.AffectedTargets`.
- [ ] Preserve dirty/staged/unstaged/untracked path behavior for affected tests and runner Git env tests.
- [ ] Delete config Git helper if no longer used by config.

### Workstream E - Cleanup And Verification

Goal: leave package boundaries obvious.

- [ ] Delete moved config files or shrink them to load/decode-only responsibilities.
- [ ] Confirm `internal/config` production code still imports only `internal/model` among internal packages.
- [ ] Confirm `internal/graph` imports only `internal/model` among internal packages.
- [ ] Confirm `internal/git` imports no internal packages.
- [ ] Confirm `internal/app` wires graph-backed explain/affected/risk services.
- [ ] Run full verification.

---

## 7. Tests

- `rtk go test ./...`
- `go run ./cmd/bach affected`
- `go run ./cmd/bach run shell/test`

Focused tests:

- `rtk go test ./internal/graph ./internal/config ./internal/app ./internal/cli`
- `rtk go test ./internal/git ./internal/runner`
- Import graph check with `go list`.
- Grep checks:
  - `internal/config` has no affected/explain/risk/Git-changed graph services.
  - `internal/graph` has no config import.
  - `internal/git` has no internal imports.

---

## 8. Open questions / known limitations

1. **OQ-1 - Should explain formatting live in graph or CLI?** For this phase, graph may own explain assembly to preserve behavior with minimal churn. A later CLI-presentation cleanup can split facts from formatting.
2. **OQ-2 - Should Git changed-file discovery be its own package?** Yes. Runner already needs richer Git context, and affected default behavior needs changed files. Shared Git evidence belongs in `internal/git`, not graph or config.
3. **OQ-3 - Should graph-load plugins move now?** No. Keep plugin execution inside config load until a dedicated `internal/plugin` phase.

---

## 9. Out of scope

- Extracting `internal/plugin`.
- Changing plugin block semantics.
- Changing Bachfile syntax.
- Changing CLI output.
- Changing target specs or target handlers.
- Changing runner behavior.
- Changing State Store schema.

---

## 10. Phase Boundary

Done when:

1. `internal/graph` owns affected-target matching, target risk traversal, and explain assembly.
2. `internal/git` owns Git workspace evidence used by app and runner.
3. `internal/config` no longer contains graph evidence services and remains focused on load/decode/validation/plugin-load patches.
4. `internal/app` wires git-backed changed-file discovery into graph-backed affected matching.
5. `internal/app` wires graph-backed services for CLI explain/risk labels.
6. `internal/runner` uses `internal/git` for Git context.
7. `internal/graph` does not import config.
8. `internal/git` imports no internal packages.
9. `internal/config` imports no internal package except `internal/model`.
10. Existing CLI and runner Git behavior is preserved by tests.
11. Full Go tests and Bach `shell/test` pass.
