# Feature Agent Orchestration Example

This example shows Bach as the control plane for an OpenCode AI engineering swarm. Bach owns the execution graph, dry-run trace, merge locks, risk metadata, and deterministic gates. OpenCode agents own feature worktrees, branch commits, merge conflict resolution, and test execution.

The public graph uses agent-role names: `feature-agent-1`, `feature-agent-2`, `merge-agent-1`, and `merge-agent-2`. The scripts still use the existing plan marker contract internally: `bach/plan-1`, `PLAN_1_COMPLETED`, and `MERGE_PLAN_1_COMPLETED`.

## Commands

List the agent graph:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile list
```

Preview the delivery program without launching agents:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile --dry-run run pipeline/delivery_program
```

Run the default delivery program:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile run pipeline/delivery_program
```

Run all feature agents without merging:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile run build-features
```

Preview one merge lane:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile --dry-run run pipeline/core-merge-lane
```

## Agent Topology

The default target is `pipeline/delivery_program`.

```text
pipeline/delivery_program
  pipeline/foundation-lane
    shell/feature-agent-1
    shell/merge-agent-1
  pipeline/core-merge-lane
    shell/core-feature-swarm
      shell/feature-agent-2
      shell/feature-agent-3
      shell/feature-agent-4
    shell/merge-agent-2
    shell/merge-agent-3
    shell/merge-agent-4
  pipeline/extension-merge-lane
    shell/extension-feature-swarm
      shell/feature-agent-5
      shell/feature-agent-6
      shell/feature-agent-7
      shell/feature-agent-8
    shell/merge-agent-5
    shell/merge-agent-6
    shell/merge-agent-7
    shell/merge-agent-8
  shell/final-regression
```

The shape is intentionally agentic:

- `feature-agent-1` builds the foundational branch that downstream features assume exists.
- `merge-agent-1` lands that foundation before the core swarm starts.
- `feature-agent-2`, `feature-agent-3`, and `feature-agent-4` fan out in parallel on isolated worktrees.
- `merge-agent-2`, `merge-agent-3`, and `merge-agent-4` serialize integration through the shared `git-merge` lock.
- `feature-agent-5` through `feature-agent-8` fan out only after the core lane is integrated.
- `merge-agent-5` through `merge-agent-8` run as a deterministic merge train.
- `final-regression` verifies the integrated mainline after every merge agent has completed.
- `long-running-agent-loop` shows an open-ended, resumable maintenance loop with an 8-hour timeout,
  a dedicated lock, a checkpoint, and explicit lint/test gates.

## Bach Features Shown

The Bachfile demonstrates:

- Nested `pipeline` targets for reusable agent lanes.
- Aggregate `shell` targets with `depends_on` for parallel feature swarms.
- `alias` targets such as `build-features` and `ship-features`.
- `input.file` declarations to document source inputs for agent prompts.
- `tools` and `preflights` for early environment checks.
- Quality-gate validation remains a first-class Bach target; in this repository, root
  `shell/lint` runs golangci-lint and stores Checkstyle findings.
- Target metadata such as `description`, `when`, and `cost` for agent/human guidance.
- Explicit risk flags on merge agents: `remote`, `destructive`, and `requires_confirmation`.
- `lock = "git-merge"` so merge agents never integrate concurrently.
- `lock = "agent-loop"` so only one long-running maintenance loop owns the checkout.
- A local verification Bachfile: `examples/plan-agents/Bachfile.verification`.

## Lanes

Named lanes keep the top-level program readable and make partial dry-runs useful:

```text
pipeline/foundation-lane
  shell/feature-agent-1
  shell/merge-agent-1

pipeline/core-merge-lane
  shell/core-feature-swarm
  shell/merge-agent-2
  shell/merge-agent-3
  shell/merge-agent-4

pipeline/extension-merge-lane
  shell/extension-feature-swarm
  shell/merge-agent-5
  shell/merge-agent-6
  shell/merge-agent-7
  shell/merge-agent-8
```

`shell/feature-swarm-all` is intentionally different from the delivery program. It runs every feature agent concurrently subject to `-jobs`, but it does not start any merge agent.

## Expected Dry-Run Output

Dry-run prints the agent graph without calling `opencode`:

```text
[pipeline/delivery_program] pipeline: pipeline/foundation-lane -> pipeline/core-merge-lane -> pipeline/extension-merge-lane -> shell/final-regression
[pipeline/foundation-lane] pipeline: shell/feature-agent-1 -> shell/merge-agent-1
[shell/feature-agent-1] sh examples/plan-agents/scripts/run-plan-agent.sh 1 computed variables foundation shell/plan-1-acceptance
[shell/merge-agent-1 lock=git-merge] sh examples/plan-agents/scripts/run-merge-agent.sh 1
[pipeline/core-merge-lane] pipeline: shell/core-feature-swarm -> shell/merge-agent-2 -> shell/merge-agent-3 -> shell/merge-agent-4
[shell/core-feature-swarm] aggregate
[shell/feature-agent-2] sh examples/plan-agents/scripts/run-plan-agent.sh 2 ordered pipelines shell/plan-2-acceptance
...
[shell/final-regression] go run ./cmd/bach -f examples/plan-agents/Bachfile.verification run pipeline/all-plan-tests
```

The exact order inside a feature swarm may vary because those agents are ready at the same time.

## Resumable Sessions

Feature and merge agents are intentionally resumable. The wrapper script derives a stable OpenCode session title from the agent role, scope, and prompt hash:

```text
bachkator/feature-agent-2/ordered-pipelines/<prompt-hash>
bachkator/merge-agent-2/bach-plan-2/<prompt-hash>
```

On the first run, the wrapper starts OpenCode with `opencode run --title <stable-title>`. After the session exists, it records the session id under:

```text
.bach/opencode-sessions/feature-agent-2-ordered-pipelines-<prompt-hash>.env
.bach/opencode-sessions/merge-agent-2-bach-plan-2-<prompt-hash>.env
```

On a later run with the same prompt, the wrapper resumes with `opencode run --session <session-id>`. If the env file is missing, it searches `opencode session list --format json` for the stable title before starting a new session.

Each run writes a fresh current log and appends it to the durable aggregate log only after marker checks:

```text
.bach/agent-runs/plan-2.current.log
.bach/agent-runs/plan-2.log
.bach/merge-runs/plan-2.current.log
.bach/merge-runs/plan-2.log
```

This prevents an old `PLAN_N_FAILED` or `MERGE_PLAN_N_FAILED` marker from poisoning a later successful resume.

## Long-Running Agent Loop

`shell/long-running-agent-loop` is a deliberately high-cost, confirmation-gated example for ongoing
repo maintenance. It writes a stable prompt and checkpoint under `.bach/opencode-sessions/`, then
resumes the same OpenCode session through `run-opencode-session.sh`.

Dry-run it first:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile --dry-run run shell/long-running-agent-loop
```

Real execution requires `-yes`, a clean worktree preflight, and an authenticated `opencode` binary:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile -yes run shell/long-running-agent-loop
```

The prompt tells the agent to repeat a safe loop: inspect status, run `bach affected`, make one
coherent improvement, run focused checks plus root `shell/lint` and `shell/test`, update docs when
behavior changes, and append checkpoint notes. It must not release unless explicitly instructed.

## Checkpoints

Each agent also gets a checkpoint file:

```text
.bach/opencode-sessions/feature-agent-2.checkpoint
.bach/opencode-sessions/merge-agent-2.checkpoint
```

The wrapper records session metadata and whether it is starting or resuming. The prompt requires OpenCode to update the checkpoint at milestones such as worktree ready, dry-run inspected, edits complete, affected targets inspected, acceptance passed, committed, conflicts resolved, blocked, or complete.

## Feature Agent Outputs

Each feature agent writes one log file:

```text
.bach/agent-runs/plan-1.log
.bach/agent-runs/plan-2.log
.bach/agent-runs/plan-3.log
.bach/agent-runs/plan-4.log
.bach/agent-runs/plan-5.log
.bach/agent-runs/plan-6.log
.bach/agent-runs/plan-7.log
.bach/agent-runs/plan-8.log
```

A successful feature agent emits one readiness marker:

```text
PLAN_1_COMPLETED
PLAN_2_COMPLETED
PLAN_3_COMPLETED
PLAN_4_COMPLETED
PLAN_5_COMPLETED
PLAN_6_COMPLETED
PLAN_7_COMPLETED
PLAN_8_COMPLETED
```

A blocked feature agent emits a failure marker and omits the completed marker:

```text
PLAN_4_FAILED: profile tests failed after config conflict
```

## Merge Agent Outputs

Each merge agent writes one log file:

```text
.bach/merge-runs/plan-1.log
.bach/merge-runs/plan-2.log
.bach/merge-runs/plan-3.log
.bach/merge-runs/plan-4.log
.bach/merge-runs/plan-5.log
.bach/merge-runs/plan-6.log
.bach/merge-runs/plan-7.log
.bach/merge-runs/plan-8.log
```

A successful merge agent writes:

```text
MERGE_PLAN_1_STARTED
branch=bach/plan-1
...
MERGE_PLAN_1_COMPLETED
```

A failed merge agent writes a failure marker and omits the completed marker:

```text
MERGE_PLAN_3_FAILED: missing branch bach/plan-3
MERGE_PLAN_5_FAILED: worktree is not clean
```

## Failure Modes

`opencode` exits non-zero:

The corresponding `shell/feature-agent-N` target fails. `run-plan-agent.sh` appends `PLAN_N_FAILED: opencode exited non-zero` to the worker log.

Feature agent omits `PLAN_N_COMPLETED`:

The corresponding `shell/feature-agent-N` target fails because `run-plan-agent.sh` checks for the marker before exiting.

Feature agent writes `PLAN_N_FAILED`:

The corresponding `shell/feature-agent-N` target fails immediately.

Merge branch is missing:

OpenCode should write `MERGE_PLAN_N_FAILED: missing branch bach/plan-N` to `.bach/merge-runs/plan-N.log`. `run-merge-agent.sh` fails if the completed marker is absent.

Main worktree is dirty:

OpenCode should stop before merging and write `MERGE_PLAN_N_FAILED: worktree is not clean`. This protects unrelated local changes.

Git conflict during merge:

OpenCode should attempt a careful conflict resolution. If it cannot resolve safely, it should write `MERGE_PLAN_N_FAILED: reason`, and the lane stops.

Post-merge regression fails:

OpenCode should write `MERGE_PLAN_N_FAILED: regression failed` if the example verification regression fails. Fix the merged result before continuing to the next merge agent.

The example verification Bachfile focuses on plan acceptance tests. For this repository, run the root
`shell/lint` gate as a separate quality check before committing or releasing integrated work.

Final regression fails:

`shell/final-regression` fails because it runs:

```sh
go run ./cmd/bach -f examples/plan-agents/Bachfile.verification run pipeline/all-plan-tests
```

For the root repository, also run:

```sh
go run ./cmd/bach run shell/lint
```

## Scripts

Long prompts and marker checks live in scripts instead of the Bachfile:

```text
examples/plan-agents/scripts/run-plan-agent.sh
examples/plan-agents/scripts/run-merge-agent.sh
examples/plan-agents/scripts/verify-plan-markers.sh
```

This keeps the Bachfile focused on the agent graph and makes prompt logic reviewable.
